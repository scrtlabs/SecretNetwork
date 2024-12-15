//!
use super::attestation::{create_attestation_certificate, get_quote_ecdsa};
use super::seed_service::get_next_consensus_seed_from_service;
use crate::registration::attestation::verify_quote_ecdsa;
use crate::registration::onchain::split_combined_cert;
#[cfg(feature = "verify-validator-whitelist")]
use block_verifier::validator_whitelist;
use core::convert::TryInto;
use core::ptr::null;
use ed25519_dalek::{PublicKey, Signature};
use enclave_crypto::consts::{
    make_sgx_secret_path, ATTESTATION_CERT_PATH, ATTESTATION_DCAP_PATH, COLLATERAL_DCAP_PATH,
    CONSENSUS_SEED_VERSION, FILE_CERT_COMBINED, FILE_MIGRATION_CERT_LOCAL,
    FILE_MIGRATION_CERT_REMOTE, FILE_MIGRATION_DATA, FILE_MIGRATION_TARGET_INFO,
    INPUT_ENCRYPTED_SEED_SIZE, MIGRATION_CONSENSUS_PATH, PUBKEY_PATH, SEED_UPDATE_SAVE_PATH,
    SIGNATURE_TYPE,
};
#[cfg(feature = "random")]
use enclave_crypto::sha_256;
use enclave_crypto::{AESKey, Ed25519PublicKey, KeyPair, SIVEncryptable, PUBLIC_KEY_SIZE};
use enclave_ffi_types::SINGLE_ENCRYPTED_SEED_SIZE;
use enclave_utils::pointers::validate_mut_slice;
use enclave_utils::storage::{migrate_all_from_2_17, SELF_REPORT_BODY};
use enclave_utils::validator_set::ValidatorSetForHeight;
use enclave_utils::{validate_const_ptr, validate_mut_ptr, Keychain, KEY_MANAGER};
/// These functions run off chain, and so are not limited by deterministic limitations. Feel free
/// to go crazy with random generation entropy, time requirements, or whatever else
///
use log::*;
use sgx_trts::trts::rsgx_read_rand;
use sgx_tse::{rsgx_create_report, rsgx_verify_report};
use sgx_types::{
    sgx_measurement_t, sgx_report_body_t, sgx_report_t, sgx_status_t, sgx_target_info_t, SgxResult,
};
use sha2::{Digest, Sha256};
use std::collections::HashMap;
use std::fs::File;
use std::io::prelude::*;
use std::panic;
use std::slice;
use tendermint::validator::Set;
use tendermint::Hash::Sha256 as tm_Sha256;
use tendermint_proto::Protobuf;

#[cfg(feature = "verify-validator-whitelist")]
use validator_whitelist::ValidatorList;

use super::persistency::{write_master_pub_keys, write_seed};
use super::seed_exchange::{decrypt_seed, encrypt_seed, SeedType};

#[cfg(feature = "light-client-validation")]
use block_verifier::VERIFIED_BLOCK_MESSAGES;

use enclave_utils::storage::write_to_untrusted;
///
/// `ecall_init_bootstrap`
///
/// Function to handle the initialization of the bootstrap node. Generates the master private/public
/// key (seed + pk_io/sk_io). This happens once at the genesis of a chain. Returns the master
/// public key (pk_io), which is saved on-chain, and used to propagate the seed to registering nodes
///
/// # Safety
///  Something should go here
///
#[no_mangle]
pub unsafe extern "C" fn ecall_init_bootstrap(
    public_key: &mut [u8; PUBLIC_KEY_SIZE],
    spid: *const u8,
    spid_len: u32,
    api_key: *const u8,
    api_key_len: u32,
) -> sgx_status_t {
    validate_mut_ptr!(
        public_key.as_mut_ptr(),
        public_key.len(),
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );

    validate_const_ptr!(spid, spid_len as usize, sgx_status_t::SGX_ERROR_UNEXPECTED);

    validate_const_ptr!(
        api_key,
        api_key_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );

    let mut key_manager = Keychain::new_empty();

    if let Err(_e) = key_manager.create_consensus_seed() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    #[cfg(feature = "use_seed_service_on_bootstrap")]
    {
        let api_key_slice = slice::from_raw_parts(api_key, api_key_len as usize);

        let temp_keypair = match KeyPair::new() {
            Ok(kp) => kp,
            Err(e) => {
                error!("failed to create keypair {:?}", e);
                return sgx_status_t::SGX_ERROR_UNEXPECTED;
            }
        };
        let genesis_seed = key_manager.get_consensus_seed().unwrap().genesis;

        let new_consensus_seed = match get_next_consensus_seed_from_service(
            &mut key_manager,
            0,
            genesis_seed,
            api_key_slice,
            temp_keypair,
            CONSENSUS_SEED_VERSION,
        ) {
            Ok(s) => s,
            Err(e) => {
                error!("Consensus seed failure: {}", e as u64);
                return sgx_status_t::SGX_ERROR_UNEXPECTED;
            }
        };

        key_manager.set_consensus_seed(genesis_seed, new_consensus_seed);
    }

    if let Err(_e) = key_manager.generate_consensus_master_keys() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(_e) = key_manager.create_registration_key() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(status) = write_master_pub_keys(&key_manager) {
        return status;
    }

    public_key.copy_from_slice(
        &key_manager
            .seed_exchange_key()
            .unwrap()
            .current
            .get_pubkey(),
    );

    trace!(
        "ecall_init_bootstrap consensus_seed_exchange_keypair public key: {:?}",
        hex::encode(public_key)
    );

    sgx_status_t::SGX_SUCCESS
}

///
///  `ecall_init_node`
///
/// This function is called during initialization of __non__ bootstrap nodes.
///
/// It receives the master public key (pk_io) and uses it, and its node key (generated in [ecall_key_gen])
/// to decrypt the seed.
///
/// The seed was encrypted using Diffie-Hellman in the function [ecall_get_encrypted_seed]
///
/// This function happens off-chain, so if we panic for some reason it _can_ be acceptable,
///  though probably not recommended
///
/// 15/10/22 - this is now called during node startup and will evaluate whether or not a node is valid
///
/// # Safety
///  Something should go here
///
#[no_mangle]
pub unsafe extern "C" fn ecall_init_node(
    master_key: *const u8,
    master_key_len: u32,
    encrypted_seed: *const u8,
    encrypted_seed_len: u32,
    api_key: *const u8,
    api_key_len: u32,
    // seed structure 1 byte - length (96 or 48) | genesis seed bytes | current seed bytes (optional)
) -> sgx_status_t {
    validate_const_ptr!(
        master_key,
        master_key_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );

    validate_const_ptr!(
        encrypted_seed,
        encrypted_seed_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );

    validate_const_ptr!(
        api_key,
        api_key_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );

    let api_key_slice = slice::from_raw_parts(api_key, api_key_len as usize);

    let key_slice = slice::from_raw_parts(master_key, master_key_len as usize);

    if encrypted_seed_len != INPUT_ENCRYPTED_SEED_SIZE {
        error!("Encrypted seed bad length");
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    // validate this node is patched and updated

    // generate temporary key for attestation
    // let temp_key_result = KeyPair::new();
    //
    // if temp_key_result.is_err() {
    //     error!("Failed to generate temporary key for attestation");
    //     return sgx_status_t::SGX_ERROR_UNEXPECTED;
    // }

    // // this validates the cert and handles the "what if it fails" inside as well
    // let res =
    //     create_attestation_certificate(&temp_key_result.unwrap(), SIGNATURE_TYPE, api_key_slice);
    // if res.is_err() {
    //     error!("Error starting node, might not be updated",);
    //     return sgx_status_t::SGX_ERROR_UNEXPECTED;
    // }

    let encrypted_seed_slice = slice::from_raw_parts(encrypted_seed, encrypted_seed_len as usize);

    // validate this node is patched and updated

    // generate temporary key for attestation
    let temp_key_result = KeyPair::new();

    if temp_key_result.is_err() {
        error!("Failed to generate temporary key for attestation");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    #[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
    {
        // this validates the cert and handles the "what if it fails" inside as well
        let res = crate::registration::attestation::validate_enclave_version(
            temp_key_result.as_ref().unwrap(),
            SIGNATURE_TYPE,
            api_key_slice,
            None,
        );
        if res.is_err() {
            error!("Error starting node, might not be updated",);
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    }

    // public keys in certificates don't have 0x04, so we'll copy it here
    let mut target_public_key: [u8; PUBLIC_KEY_SIZE] = [0u8; PUBLIC_KEY_SIZE];

    let pk = key_slice.to_vec();

    // just make sure the of the public key isn't messed up
    if pk.len() != PUBLIC_KEY_SIZE {
        error!("Got public key with the wrong size: {:?}", pk.len());
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }
    target_public_key.copy_from_slice(&pk);

    trace!(
        "ecall_init_node target public key is: {:?}",
        target_public_key
    );

    let mut key_manager = Keychain::new();

    key_manager.delete_consensus_seed();

    // Skip the first byte which is the length of the seed
    let mut single_seed_bytes = [0u8; SINGLE_ENCRYPTED_SEED_SIZE];
    single_seed_bytes.copy_from_slice(&encrypted_seed_slice[1..(SINGLE_ENCRYPTED_SEED_SIZE + 1)]);

    trace!("Target public key is: {:?}", target_public_key);
    let genesis_seed = match decrypt_seed(&key_manager, target_public_key, single_seed_bytes) {
        Ok(result) => result,
        Err(status) => return status,
    };

    let encrypted_seed_len = encrypted_seed_slice[0] as u32;
    let new_consensus_seed;

    if encrypted_seed_len as usize == 2 * SINGLE_ENCRYPTED_SEED_SIZE {
        debug!("Got both keys from registration");

        single_seed_bytes.copy_from_slice(
            &encrypted_seed_slice
                [(SINGLE_ENCRYPTED_SEED_SIZE + 1)..(SINGLE_ENCRYPTED_SEED_SIZE * 2 + 1)],
        );
        new_consensus_seed = match decrypt_seed(&key_manager, target_public_key, single_seed_bytes)
        {
            Ok(result) => result,
            Err(status) => return status,
        };

        key_manager.set_consensus_seed(genesis_seed, new_consensus_seed);
    } else {
        let reg_key = key_manager.get_registration_key().unwrap();
        let my_pub_key = reg_key.get_pubkey();

        debug!("New consensus seed not found! Need to get it from service");
        if key_manager.get_consensus_seed().is_err() {
            new_consensus_seed = match get_next_consensus_seed_from_service(
                &mut key_manager,
                1,
                genesis_seed,
                api_key_slice,
                reg_key,
                CONSENSUS_SEED_VERSION,
            ) {
                Ok(s) => s,
                Err(e) => {
                    error!("Consensus seed failure: {}", e as u64);
                    return sgx_status_t::SGX_ERROR_UNEXPECTED;
                }
            };

            key_manager.set_consensus_seed(genesis_seed, new_consensus_seed);
        } else {
            debug!("New consensus seed already exists, no need to get it from service");
        }

        let mut res: Vec<u8> = encrypt_seed(my_pub_key, SeedType::Genesis, false).unwrap();
        let res_current: Vec<u8> = encrypt_seed(my_pub_key, SeedType::Current, false).unwrap();
        res.extend(&res_current);

        trace!("Done encrypting seed, got {:?}, {:?}", res.len(), res);

        if let Err(_e) = write_seed(&res, SEED_UPDATE_SAVE_PATH) {
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    }

    // this initializes the key manager with all the keys we need for computations
    if let Err(_e) = key_manager.generate_consensus_master_keys() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(status) = write_master_pub_keys(&key_manager) {
        return status;
    }

    sgx_status_t::SGX_SUCCESS
}

unsafe fn get_attestation_report_epid(
    api_key: *const u8,
    api_key_len: u32,
    kp: &KeyPair,
) -> Result<Vec<u8>, sgx_status_t> {
    // validate_const_ptr!(spid, spid_len as usize, sgx_status_t::SGX_ERROR_UNEXPECTED);
    // let spid_slice = slice::from_raw_parts(spid, spid_len as usize);

    validate_const_ptr!(
        api_key,
        api_key_len as usize,
        Err(sgx_status_t::SGX_ERROR_UNEXPECTED),
    );
    let api_key_slice = slice::from_raw_parts(api_key, api_key_len as usize);

    let (_private_key_der, cert) =
        match create_attestation_certificate(kp, SIGNATURE_TYPE, api_key_slice, None) {
            Err(e) => {
                warn!("Error in create_attestation_certificate: {:?}", e);
                return Err(e);
            }
            Ok(res) => res,
        };

    #[cfg(feature = "SGX_MODE_HW")]
    {
        crate::registration::print_report::print_local_report_info(cert.as_slice());
    }

    Ok(cert)
}

pub unsafe fn get_attestation_report_dcap(
    pub_k: &[u8],
) -> Result<(Vec<u8>, Vec<u8>), sgx_status_t> {
    let (vec_quote, vec_coll) = match get_quote_ecdsa(pub_k) {
        Ok(r) => r,
        Err(e) => {
            warn!("Error creating attestation report");
            return Err(e);
        }
    };

    Ok((vec_quote, vec_coll))
}

pub fn save_attestation_combined(
    res_dcap: &Result<(Vec<u8>, Vec<u8>), sgx_status_t>,
    res_epid: &Result<Vec<u8>, sgx_status_t>,
    is_migration_report: bool,
) -> sgx_status_t {
    let mut size_epid: u32 = 0;
    let mut size_dcap_q: u32 = 0;
    let mut size_dcap_c: u32 = 0;

    if let Ok(ref vec_cert) = res_epid {
        size_epid = vec_cert.len() as u32;

        if !is_migration_report {
            write_to_untrusted(vec_cert.as_slice(), ATTESTATION_CERT_PATH.as_str()).unwrap();
        }
    }

    if let Ok((ref vec_quote, ref vec_coll)) = res_dcap {
        size_dcap_q = vec_quote.len() as u32;
        size_dcap_c = vec_coll.len() as u32;

        if !is_migration_report {
            write_to_untrusted(vec_quote, ATTESTATION_DCAP_PATH.as_str()).unwrap();
            write_to_untrusted(vec_coll, COLLATERAL_DCAP_PATH.as_str()).unwrap();
        }
    }

    let out_path = make_sgx_secret_path(if is_migration_report {
        FILE_MIGRATION_CERT_REMOTE
    } else {
        FILE_CERT_COMBINED
    });

    let mut f_out = match File::create(out_path.as_str()) {
        Ok(f) => f,
        Err(e) => {
            error!("failed to create file {}", e);
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    };

    f_out.write_all(&size_epid.to_le_bytes()).unwrap();
    f_out.write_all(&size_dcap_q.to_le_bytes()).unwrap();
    f_out.write_all(&size_dcap_c.to_le_bytes()).unwrap();

    if let Ok(ref vec_cert) = res_epid {
        f_out.write_all(vec_cert.as_slice()).unwrap();
    }

    if let Ok((vec_quote, vec_coll)) = res_dcap {
        f_out.write_all(vec_quote.as_slice()).unwrap();
        f_out.write_all(vec_coll.as_slice()).unwrap();
    }

    if (size_epid == 0) && (size_dcap_q == 0) {
        if let Err(status) = res_epid {
            return *status;
        }
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    sgx_status_t::SGX_SUCCESS
}

fn get_verified_migration_report_body() -> SgxResult<sgx_report_body_t> {
    if let Ok(mut f_in) = File::open(make_sgx_secret_path(FILE_MIGRATION_CERT_LOCAL)) {
        let mut buffer = vec![0u8; std::mem::size_of::<sgx_report_t>()];
        if f_in.read_exact(&mut buffer).is_ok() {
            println!("Found local migration report");
            let report: sgx_report_t =
                unsafe { std::ptr::read(buffer.as_ptr() as *const sgx_report_t) };

            match rsgx_verify_report(&report) {
                Ok(()) => {
                    return Ok(report.body);
                }
                Err(e) => {
                    error!("Can't verify local report: {}", e);
                }
            }
        }
    }

    if let Ok(mut f_in) = File::open(make_sgx_secret_path(FILE_MIGRATION_CERT_REMOTE)) {
        println!("Found remote migration report");

        let mut cert = vec![];
        f_in.read_to_end(&mut cert).unwrap();

        let (_, vec_quote, vec_coll) = split_combined_cert(cert.as_ptr(), cert.len() as u32);

        match verify_quote_ecdsa(vec_quote.as_slice(), vec_coll.as_slice(), 0) {
            Ok((body, _)) => {
                return Ok(body);
            }
            Err(e) => {
                error!("Can't verify remote quote: {}", e);
            }
        }
    }

    Err(sgx_status_t::SGX_ERROR_NO_PRIVILEGE)
}

#[no_mangle]
/**
 * `ecall_get_attestation_report`
 *
 * Creates the attestation report to be used to authenticate with the blockchain. The output of this
 * function is an X.509 certificate signed by the enclave, which contains the report signed by Intel.
 *
 * Verifying functions will verify the public key bytes sent in the extra data of the __report__ (which
 * may or may not match the public key of the __certificate__ -- depending on implementation choices)
 *
 * This x509 certificate can be used in the future for mutual-RA cross-enclave TLS channels, or for
 * other creative usages.
 * # Safety
 * Something should go here
*/
pub unsafe extern "C" fn ecall_get_attestation_report(
    api_key: *const u8,
    api_key_len: u32,
    flags: u32,
) -> sgx_status_t {
    let mut report_data: [u8; 48] = [0; 48];

    let (kp, is_migration_report) = match 0x10 & flags {
        0x10 => {
            // migration report
            (Keychain::get_migration_keys(), true)
        }
        _ => {
            // standard network registration report
            let kp = KEY_MANAGER.get_registration_key().unwrap();
            trace!(
                "ecall_get_attestation_report key pk: {:?}",
                hex::encode(kp.get_pubkey())
            );

            let mut f_out = match File::create(PUBKEY_PATH.as_str()) {
                Ok(f) => f,
                Err(e) => {
                    error!("failed to create file {}", e);
                    return sgx_status_t::SGX_ERROR_UNEXPECTED;
                }
            };

            f_out.write_all(kp.get_pubkey().as_ref()).unwrap();

            (kp, false)
        }
    };

    let res_epid = match 1 & flags {
        0 => get_attestation_report_epid(api_key, api_key_len, &kp),
        _ => Err(sgx_status_t::SGX_ERROR_FEATURE_NOT_SUPPORTED),
    };

    let res_dcap = match 2 & flags {
        0 => {
            report_data[0..32].copy_from_slice(&kp.get_pubkey());
            get_attestation_report_dcap(&report_data)
        }
        _ => Err(sgx_status_t::SGX_ERROR_FEATURE_NOT_SUPPORTED),
    };

    save_attestation_combined(&res_dcap, &res_epid, is_migration_report)
}

///
/// This function generates the registration_key, which is used in the attestation and registration
/// process
///
#[no_mangle]
pub unsafe extern "C" fn ecall_key_gen(
    public_key: &mut [u8; PUBLIC_KEY_SIZE],
) -> sgx_types::sgx_status_t {
    if let Err(_e) = validate_mut_slice(public_key) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let mut key_manager = Keychain::new_empty();
    if let Err(_e) = key_manager.create_registration_key() {
        error!("Failed to create registration key");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    };

    let reg_key = key_manager.get_registration_key();

    if reg_key.is_err() {
        error!("Failed to unlock node key. Please make sure the file is accessible or reinitialize the node");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let pubkey = reg_key.unwrap().get_pubkey();
    public_key.clone_from_slice(&pubkey);
    trace!("ecall_key_gen key pk: {:?}", hex::encode(&public_key));
    sgx_status_t::SGX_SUCCESS
}

///
/// `ecall_get_genesis_seed
///
/// This call is used to help new nodes that want to full sync to have the previous "genesis" seed
/// A node that is regestering or being upgraded to version 1.9 will call this function.
///
/// The seed is encrypted with a key derived from the secret master key of the chain, and the public
/// key of the requesting chain
///
/// This function happens off-chain
///
#[no_mangle]
pub unsafe extern "C" fn ecall_get_genesis_seed(
    pk: *const u8,
    pk_len: u32,
    seed: &mut [u8; SINGLE_ENCRYPTED_SEED_SIZE],
) -> sgx_types::sgx_status_t {
    validate_mut_ptr!(
        seed.as_mut_ptr(),
        seed.len(),
        sgx_status_t::SGX_ERROR_UNEXPECTED
    );

    let pk_slice = std::slice::from_raw_parts(pk, pk_len as usize);

    let result = panic::catch_unwind(|| -> Result<Vec<u8>, sgx_types::sgx_status_t> {
        // just make sure the length isn't wrong for some reason (certificate may be malformed)
        if pk_slice.len() != PUBLIC_KEY_SIZE {
            warn!(
                "Got public key from certificate with the wrong size: {:?}",
                pk_slice.len()
            );
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }

        let mut target_public_key: [u8; 32] = [0u8; 32];
        target_public_key.copy_from_slice(pk_slice);
        trace!(
            "ecall_get_encrypted_genesis_seed target_public_key key pk: {:?}",
            &target_public_key.to_vec()
        );

        let res: Vec<u8> = encrypt_seed(target_public_key, SeedType::Genesis, true)
            .map_err(|_| sgx_status_t::SGX_ERROR_UNEXPECTED)?;

        Ok(res)
    });

    if let Ok(res) = result {
        match res {
            Ok(res) => {
                trace!("Done encrypting seed, got {:?}, {:?}", res.len(), res);

                seed.copy_from_slice(&res);
                trace!("returning with seed: {:?}, {:?}", seed.len(), seed);
                sgx_status_t::SGX_SUCCESS
            }
            Err(e) => {
                trace!("error encrypting seed {:?}", e);
                e
            }
        }
    } else {
        warn!("Enclave call ecall_get_genesis_seed panic!");
        sgx_status_t::SGX_ERROR_UNEXPECTED
    }
}

#[no_mangle]
pub unsafe extern "C" fn ecall_migration_op(opcode: u32) -> sgx_types::sgx_status_t {
    match opcode {
        0 => {
            println!("Convert legacy SGX files");
            migrate_all_from_2_17()
        }
        1 => {
            println!("Create self migration report");

            export_local_migration_report();
            ecall_get_attestation_report(null(), 0, 0x11) // migration, no-epid
        }
        2 => {
            println!("Export encrypted data to the next aurhorized enclave");
            export_sealed_data()
        }
        3 => {
            println!("Import sealed data from the previous enclave");
            import_sealed_data()
        }
        4 => {
            println!("Import sealed data from the legacy enclave");
            import_sealing_legacy()
        }
        5 => {
            println!("Export self target info");
            export_self_target_info()
        }
        _ => sgx_status_t::SGX_ERROR_UNEXPECTED,
    }
}

fn is_msg_mrenclave(msg_in_block: &[u8], mrenclave: &[u8]) -> bool {
    trace!("*** block msg: {:?}", hex::encode(msg_in_block));

    // we expect a message of the form:
    // 0a 2d (addr, len=45 bytes) 12 20 (mrenclave 32 bytes)

    if msg_in_block.len() != 81 {
        trace!("len mismatch: {}", msg_in_block.len());
        return false;
    }

    if &msg_in_block[0..2] != [0x0a, 0x2d].as_slice() {
        trace!("wrong sub1");
        return false;
    }

    if &msg_in_block[47..49] != [0x12, 0x20].as_slice() {
        trace!("wrong sub2");
        return false;
    }

    if &msg_in_block[49..81] != mrenclave {
        trace!("wrong mrenclave");
        return false;
    }

    true
}

#[cfg(feature = "light-client-validation")]
fn check_mrenclave_in_block(msg_slice: &[u8]) -> bool {
    let mut verified_msgs = VERIFIED_BLOCK_MESSAGES.lock().unwrap();

    while verified_msgs.remaining() > 0 {
        if let Some(verified_msg) = verified_msgs.get_next() {
            if is_msg_mrenclave(&verified_msg, msg_slice) {
                return true;
            }
        }
    }
    false
}

#[cfg(not(feature = "light-client-validation"))]
fn check_mrenclave_in_block(_msg_slice: &[u8]) -> bool {
    true
}

#[no_mangle]
pub unsafe extern "C" fn ecall_onchain_approve_upgrade(
    msg: *const u8,
    msg_len: u32,
) -> sgx_types::sgx_status_t {
    validate_const_ptr!(msg, msg_len as usize, sgx_status_t::SGX_ERROR_UNEXPECTED);
    let msg_slice = slice::from_raw_parts(msg, msg_len as usize);

    trace!(
        "ecall_onchain_approve_upgrade mrenclave: {:?}",
        hex::encode(msg_slice)
    );

    if !check_mrenclave_in_block(msg_slice) {
        error!("migration target not approved");
        return sgx_types::sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let mut key_chain = Keychain::new();
    key_chain.next_mr_enclave = Some(sgx_measurement_t {
        m: msg_slice.try_into().unwrap(),
    });

    key_chain.save();

    info!(
        "Migration target approved. mr_encalve={}",
        hex::encode(msg_slice)
    );

    sgx_types::sgx_status_t::SGX_SUCCESS
}

fn is_export_approved_offchain(mut f_in: File, report: &sgx_report_body_t) -> bool {
    let mut json_data = String::new();
    f_in.read_to_string(&mut json_data).unwrap();

    // Deserialize the JSON string into a HashMap<String, String>
    let signatures: HashMap<String, (String, String)> =
        serde_json::from_str(&json_data).expect("Failed to deserialize JSON");

    // Build the not-yet-voted validators map
    let mut not_yet_voted_validators: HashMap<[u8; 20], u64> = HashMap::new();
    let mut total_voting_power: u64 = 0;

    {
        let validator_set_vec = Keychain::get_validator_set_for_height().validator_set;
        let validator_set =
            <Set as Protobuf<tendermint_proto::v0_38::types::ValidatorSet>>::decode(
                validator_set_vec.as_slice(),
            )
            .unwrap();

        for validator in validator_set.validators() {
            //println!("Address: {}", validator.address);
            //println!("Voting Power: {}", validator.power);
            let power: u64 = validator.power.value();
            if power > 0 {
                total_voting_power += power;
                let addr: [u8; 20] = validator.address.as_bytes().try_into().unwrap();
                not_yet_voted_validators.insert(addr, power);
            }
        }
    };

    let mut approved_power: u64 = 0;
    let mut approved_whitelisted: usize = 0;

    for (addr_str, (pubkey_str, sig_str)) in &signatures {
        let pubkey_bytes = base64::decode(pubkey_str).unwrap();

        // calculate the address
        let mut addr = [0u8; 20];
        {
            let mut hasher = Sha256::new();
            hasher.update(&pubkey_bytes);
            let res = hasher.finalize();
            addr.copy_from_slice(&res[..20]);
        }

        // make sure pubkey matches the address
        let res = hex::decode(addr_str).unwrap();
        if res != addr {
            panic!(
                "address doesn't match pubkey. Expected={}, actual={}",
                hex::encode(addr),
                addr_str
            );
        }

        // verify signature

        let pubkey_obj = PublicKey::from_bytes(&pubkey_bytes).unwrap();
        let sig_bytes = base64::decode(sig_str).unwrap();
        let sig_obj = Signature::from_bytes(&sig_bytes).unwrap();

        if pubkey_obj
            .verify_strict(&report.mr_enclave.m, &sig_obj)
            .is_err()
        {
            panic!("Incorrect signature for address: {}", addr_str);
        }

        let (voter_power, is_whitelisted) =
            if let Some((_, power)) = not_yet_voted_validators.remove_entry(&addr) {
                //not_yet_voted_validators.remove(&addr);

                #[cfg(feature = "verify-validator-whitelist")]
                let is_whitelisted = validator_whitelist::VALIDATOR_WHITELIST.contains(addr_str);

                #[cfg(not(feature = "verify-validator-whitelist"))]
                let is_whitelisted = false;

                approved_power += power;
                if is_whitelisted {
                    approved_whitelisted += 1;
                }

                (power, is_whitelisted)
            } else {
                (0, false)
            };

        println!(
            "  Approved by {}, power = {}, whitelisted = {}",
            addr_str, voter_power, is_whitelisted
        );
    }

    println!(
        "Total Power = {}, Approved Power = {}, Total whitelisted = {}",
        total_voting_power, approved_power, approved_whitelisted
    );

    #[cfg(feature = "verify-validator-whitelist")]
    if (approved_whitelisted < validator_whitelist::VALIDATOR_THRESHOLD) {
        return false;
        error!("not enogh whitelisted validators");
    }

    if approved_power * 3 < total_voting_power * 2 {
        error!("not enogh voting power");
        return false;
    }

    true
}

fn is_export_approved(report: &sgx_report_body_t) -> bool {
    // Current policy: we demand the same mr_signer

    if report.mr_signer.m != SELF_REPORT_BODY.mr_signer.m {
        println!("Migration target uses different signer");
        return false;
    }

    if let Some(val) = KEY_MANAGER.next_mr_enclave {
        if val.m == report.mr_enclave.m {
            println!("Migration is authorized by on-chain consensus");
            return true;
        }
    }

    if let Ok(f_in) = File::open(MIGRATION_CONSENSUS_PATH.as_str()) {
        if is_export_approved_offchain(f_in, report) {
            println!("Migration is authorized by off-chain (emergency) consensus");
            return true;
        }
    }

    false
}

fn export_self_target_info() -> sgx_status_t {
    let mut target_info = sgx_target_info_t::default();
    unsafe { sgx_types::sgx_self_target(&mut target_info) };

    let mut f_out = match File::create(make_sgx_secret_path(FILE_MIGRATION_TARGET_INFO)) {
        Ok(f) => f,
        Err(e) => {
            error!("failed to create file {}", e);
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    };

    let info_ptr = &target_info as *const sgx_target_info_t as *const u8;
    let info_size = std::mem::size_of::<sgx_target_info_t>();

    // Convert the report to a byte slice
    let report_bytes: &[u8] = unsafe { slice::from_raw_parts(info_ptr, info_size) };

    // Write the byte slice to the file
    f_out.write_all(report_bytes).unwrap();

    println!("Local migration self target saved");
    sgx_status_t::SGX_SUCCESS
}

fn export_local_migration_report() -> sgx_status_t {
    if let Ok(mut f_in) = File::open(make_sgx_secret_path(FILE_MIGRATION_TARGET_INFO)) {
        let mut buffer = vec![0u8; std::mem::size_of::<sgx_target_info_t>()];
        if f_in.read_exact(&mut buffer).is_ok() {
            println!("Found local migration target info");
            let target_info: sgx_target_info_t =
                unsafe { std::ptr::read(buffer.as_ptr() as *const sgx_target_info_t) };

            let mut report_data = sgx_types::sgx_report_data_t::default();
            report_data.d[..32].copy_from_slice(&Keychain::get_migration_keys().get_pubkey());

            let my_report = match rsgx_create_report(&target_info, &report_data) {
                Ok(report) => report,
                Err(e) => {
                    return e;
                }
            };

            let mut f_out = match File::create(make_sgx_secret_path(FILE_MIGRATION_CERT_LOCAL)) {
                Ok(f) => f,
                Err(e) => {
                    error!("failed to create file {}", e);
                    return sgx_status_t::SGX_ERROR_UNEXPECTED;
                }
            };

            let report_ptr = &my_report as *const sgx_report_t as *const u8;
            let report_size = std::mem::size_of::<sgx_report_t>();

            // Convert the report to a byte slice
            let report_bytes: &[u8] = unsafe { slice::from_raw_parts(report_ptr, report_size) };

            // Write the byte slice to the file
            f_out.write_all(report_bytes).unwrap();

            println!("Local migration report successfully saved");
        }
    }
    sgx_status_t::SGX_SUCCESS
}

fn export_sealed_data() -> sgx_status_t {
    let next_report = match get_verified_migration_report_body() {
        Ok(report) => report,
        Err(e) => {
            error!("No next migration report: {}", e);
            return e;
        }
    };

    if !is_export_approved(&next_report) {
        error!("Export sealing not authorized");
        return sgx_status_t::SGX_ERROR_NO_PRIVILEGE;
    }

    let kp = KeyPair::new().unwrap();
    let other_pub_k = &next_report.report_data.d[0..32].try_into().unwrap();
    let aes_key = AESKey::new_from_slice(&kp.diffie_hellman(other_pub_k));

    let mut data_plain = Vec::new();
    KEY_MANAGER.serialize(&mut data_plain).unwrap();

    let data_encrypted = aes_key.encrypt_siv(&data_plain, None).unwrap();

    let mut f_out = match File::create(make_sgx_secret_path(FILE_MIGRATION_DATA)) {
        Ok(f) => f,
        Err(e) => {
            error!("failed to create file {}", e);
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    };

    f_out.write_all(&kp.get_pubkey()).unwrap();
    f_out.write_all(&data_encrypted).unwrap();

    println!("Sealed data successfully exported");
    sgx_status_t::SGX_SUCCESS
}

fn import_sealed_data() -> sgx_status_t {
    let mut f_in = match File::open(make_sgx_secret_path(FILE_MIGRATION_DATA)) {
        Ok(f) => f,
        Err(e) => {
            error!("failed to open file {}", e);
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    };

    let mut other_pub_k: Ed25519PublicKey = Ed25519PublicKey::default();
    f_in.read_exact(&mut other_pub_k).unwrap();

    let mut data_encrypted = Vec::new();
    f_in.read_to_end(&mut data_encrypted).unwrap();

    let kp = Keychain::get_migration_keys();
    let aes_key = AESKey::new_from_slice(&kp.diffie_hellman(&other_pub_k));

    let data_plain = match aes_key.decrypt_siv(&data_encrypted, None) {
        Ok(res) => res,
        Err(err) => {
            error!("Can't decrypt sealing key: {}", err);
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    };

    let mut key_manager = Keychain::new_empty();

    match key_manager.deserialize(&mut std::io::Cursor::new(data_plain)) {
        Ok(_) => {
            key_manager.save();
            info!("Sealing data successfully imported");
            sgx_status_t::SGX_SUCCESS
        }
        Err(err) => {
            info!("Failed to read sealed data: {}", err);
            sgx_status_t::SGX_ERROR_UNEXPECTED
        }
    }
}

fn import_sealing_legacy() -> sgx_status_t {
    match Keychain::new_from_legacy() {
        Some(key_manager) => {
            key_manager.save();
            info!("Legacy data successfully imported");
            sgx_status_t::SGX_SUCCESS
        }
        None => {
            info!("Legacy data not found");
            sgx_status_t::SGX_ERROR_UNEXPECTED
        }
    }
}

const MAX_VARIABLE_LENGTH: u32 = 100_000;
const ENCRYPTED_RANDOM_LENGTH: u32 = 48;
const PROOF_LENGTH: u32 = 32;
const BLOCK_HASH_LENGTH: u32 = 32;

macro_rules! validate_input_length {
    ($input:expr, $var_name:expr, $constant:expr) => {
        if $input > $constant {
            error!(
                "Error: {} ({}) is larger than the constant value ({})",
                $var_name, $input, $constant
            );
            return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
        }
    };
}

pub fn calculate_validator_set_hash(validator_set_slice: &[u8]) -> SgxResult<tendermint::Hash> {
    match <tendermint::validator::Set as Protobuf<tendermint_proto::v0_38::types::ValidatorSet,>>::decode(validator_set_slice)
    {
        Ok(vs) => Ok(vs.hash()),
        Err(e) => {
            error!("error decoding validator set: {:?}", e);
            Err(sgx_status_t::SGX_ERROR_UNEXPECTED)
        }
    }
}

#[no_mangle]
pub unsafe extern "C" fn ecall_submit_validator_set_evidence(
    val_set_evidence: *const u8,
) -> sgx_status_t {
    let evidence_len: usize = 32;

    validate_const_ptr!(
        val_set_evidence,
        evidence_len,
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    );

    let mut verified_msgs = VERIFIED_BLOCK_MESSAGES.lock().unwrap();

    verified_msgs
        .next_validators_evidence
        .copy_from_slice(slice::from_raw_parts(val_set_evidence, evidence_len));

    sgx_status_t::SGX_SUCCESS
}

/// # Safety
/// make sure to check that block_hash is a valid pointer and that it's exactly 32 bytes long
#[no_mangle]
pub unsafe extern "C" fn ecall_generate_random(
    block_hash: *const u8,
    block_hash_len: u32,
    _height: u64,
    random: &mut [u8; ENCRYPTED_RANDOM_LENGTH as usize],
    _proof: &mut [u8; PROOF_LENGTH as usize],
) -> sgx_status_t {
    validate_const_ptr!(
        block_hash,
        block_hash_len as usize,
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    );

    if block_hash_len != BLOCK_HASH_LENGTH {
        error!("block hash bad length");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let mut rand_buf: [u8; 32] = [0; 32];

    if let Err(_e) = rsgx_read_rand(&mut rand_buf) {
        error!("Error generating random value");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    };

    let validator_set_hash = match calculate_validator_set_hash(
        Keychain::get_validator_set_for_height()
            .validator_set
            .as_slice(),
    ) {
        Ok(tm_Sha256(hash)) => hash,
        _ => {
            error!("Got invalid validator set");
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    };

    // todo: add entropy detection

    let encrypted: Vec<u8> = if let Ok(res) =
        KEY_MANAGER.random_encryption_key.unwrap().encrypt_siv(
            &rand_buf,
            Some(vec![validator_set_hash.as_slice()].as_slice()),
        ) {
        res
    } else {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    };

    random.copy_from_slice(encrypted.as_slice());

    // proof is an encrypted value that allows enclaves to validate that the encrypted value was created for
    // this specific height & block. This allows for replay protection.
    // optional improvement: Add public key signatures to be able to validate this outside the enclave
    #[cfg(feature = "random")]
    {
        let block_hash_slice = slice::from_raw_parts(block_hash, block_hash_len as usize);

        let proof_computed = enclave_utils::random::create_random_proof(
            &KEY_MANAGER.initial_randomness_seed.unwrap(),
            _height,
            encrypted.as_slice(),
            block_hash_slice,
        );
        _proof.copy_from_slice(proof_computed.as_slice());
    }

    // debug!("Calculated proof: {:?}", proof_computed);

    sgx_status_t::SGX_SUCCESS
}

/// # Safety
/// Validator set can be of variable length, but it shouldn't be too long (and obv a valid pointer)
#[no_mangle]
pub unsafe extern "C" fn ecall_submit_validator_set(
    val_set: *const u8,
    val_set_len: u32,
    height: u64,
) -> sgx_status_t {
    validate_input_length!(val_set_len, "validator set length", MAX_VARIABLE_LENGTH);
    validate_const_ptr!(
        val_set,
        val_set_len as usize,
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    );

    let validator_set_slice = slice::from_raw_parts(val_set, val_set_len as usize);
    let validator_set_hash = match calculate_validator_set_hash(validator_set_slice) {
        Ok(tm_Sha256(hash)) => hash,
        _ => {
            error!("invalid validator set");
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    };

    let validator_set_evidence = KEY_MANAGER.encrypt_hash(validator_set_hash, height);

    {
        let verified_msgs = VERIFIED_BLOCK_MESSAGES.lock().unwrap();

        if verified_msgs.next_validators_evidence != validator_set_evidence {
            error!("************************ validator set evidence mismatch");
            //return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    }

    let val_set_for_height = ValidatorSetForHeight {
        height,
        validator_set: validator_set_slice.to_vec(),
    };

    Keychain::set_validator_set_for_height(val_set_for_height);

    // calculate hash, and compare with the stored next_validators_hash

    sgx_status_t::SGX_SUCCESS
}

/// # Safety
/// Random will be 48 bytes
/// Proof will be 32 bytes
#[no_mangle]
pub unsafe extern "C" fn ecall_validate_random(
    random: *const u8,
    random_len: u32,
    proof: *const u8,
    proof_len: u32,
    block_hash: *const u8,
    block_hash_len: u32,
    _height: u64,
) -> sgx_status_t {
    validate_input_length!(random_len, "encrypted_random", ENCRYPTED_RANDOM_LENGTH);
    validate_input_length!(proof_len, "proof", PROOF_LENGTH);
    if block_hash_len != BLOCK_HASH_LENGTH {
        error!("block hash bad length");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    validate_const_ptr!(
        random,
        random_len as usize,
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    );
    validate_const_ptr!(
        proof,
        proof_len as usize,
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    );
    validate_const_ptr!(
        block_hash,
        block_hash_len as usize,
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    );

    #[cfg(feature = "random")]
    {
        let random_slice = slice::from_raw_parts(random, random_len as usize);
        let proof_slice = slice::from_raw_parts(proof, proof_len as usize);
        let block_hash_slice = slice::from_raw_parts(block_hash, block_hash_len as usize);

        let calculated_proof = enclave_utils::random::create_random_proof(
            &KEY_MANAGER.initial_randomness_seed.unwrap(),
            _height,
            random_slice,
            block_hash_slice,
        );

        // debug!("Calculated proof: {:?}", calculated_proof);
        // debug!("Got proof: {:?}", proof_slice);

        if calculated_proof != proof_slice {
            // otherwise on an upgrade this will break horribly - next patch we can remove this
            let legacy_proof = create_legacy_proof(_height, random_slice, block_hash_slice);
            if legacy_proof != calculated_proof {
                return sgx_status_t::SGX_ERROR_INVALID_SIGNATURE;
            }
        }
    }

    sgx_status_t::SGX_SUCCESS
}

#[cfg(feature = "random")]
fn create_legacy_proof(height: u64, random: &[u8], block_hash: &[u8]) -> [u8; 32] {
    let mut data = vec![];
    data.extend_from_slice(&height.to_be_bytes());
    data.extend_from_slice(random);
    data.extend_from_slice(block_hash);
    data.extend_from_slice(KEY_MANAGER.initial_randomness_seed.unwrap().get());

    sha_256(data.as_slice())
}
