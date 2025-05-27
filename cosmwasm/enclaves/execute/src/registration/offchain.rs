//!
use super::attestation::{create_attestation_certificate, get_quote_ecdsa};
use super::seed_service::get_next_consensus_seed_from_service;
use crate::registration::attestation::verify_quote_sgx;
use crate::registration::onchain::split_combined_cert;
#[cfg(feature = "verify-validator-whitelist")]
use block_verifier::validator_whitelist;
use core::convert::TryInto;
use core::ptr::null;
use ed25519_dalek::{PublicKey, Signature};
use enclave_crypto::consts::{
    make_sgx_secret_path, CONSENSUS_SEED_VERSION, FILE_ATTESTATION_CERTIFICATE, FILE_CERT_COMBINED,
    FILE_MIGRATION_CERT_LOCAL, FILE_MIGRATION_CERT_REMOTE, FILE_MIGRATION_CONSENSUS,
    FILE_MIGRATION_DATA, FILE_MIGRATION_TARGET_INFO, FILE_PUBKEY, INPUT_ENCRYPTED_SEED_SIZE,
    SEED_UPDATE_SAVE_PATH, SIGNATURE_TYPE,
};
#[cfg(feature = "random")]
use enclave_crypto::{
    consts::SELF_REPORT_BODY, sha_256, AESKey, Ed25519PublicKey, KeyPair, SIVEncryptable,
    PUBLIC_KEY_SIZE,
};
use enclave_ffi_types::SINGLE_ENCRYPTED_SEED_SIZE;
use enclave_utils::key_manager::KeychainMutableData;
use enclave_utils::pointers::validate_mut_slice;
use enclave_utils::storage::{get_key_from_seed, migrate_all_from_2_17, rotate_store};
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
use std::sgxfs::SgxFile;
use std::slice;
use tendermint::Hash::Sha256 as tm_Sha256;

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
    key_manager.save();

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
    key_manager.save();

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

    key_manager.save();
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
            write_to_untrusted(
                vec_cert.as_slice(),
                make_sgx_secret_path(FILE_ATTESTATION_CERTIFICATE).as_str(),
            )
            .unwrap();
        }
    }

    if let Ok((ref vec_quote, ref vec_coll)) = res_dcap {
        size_dcap_q = vec_quote.len() as u32;
        size_dcap_c = vec_coll.len() as u32;
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

        match verify_quote_sgx(vec_quote.as_slice(), vec_coll.as_slice(), 0) {
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

            let mut f_out = match File::create(make_sgx_secret_path(FILE_PUBKEY).as_str()) {
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
    key_manager.save();

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
pub unsafe extern "C" fn ecall_rotate_store(p_buf: *mut u8, n_buf: u32) -> sgx_types::sgx_status_t {
    validate_const_ptr!(
        p_buf,
        n_buf as usize,
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    );

    let consensus_ikm = match KEY_MANAGER.get_consensus_state_ikm() {
        Ok(keys) => keys,
        Err(e) => {
            error!("no current ikm keys {}", e);
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    };

    let rot_seed = match read_rot_seed() {
        Ok(seed) => seed,
        Err(e) => {
            return e;
        }
    };

    let next_ikm = Keychain::generate_consensus_ikm_key(&rot_seed);

    let mut _num_total: u32 = 0;
    let mut _num_recoded: u32 = 0;

    match rotate_store(
        p_buf,
        n_buf as usize,
        &consensus_ikm.current,
        &next_ikm,
        &mut _num_total,
        &mut _num_recoded,
    ) {
        Ok(()) => {
            //trace!("------- Total={}, Recoded={}", num_total, num_recoded);
            sgx_status_t::SGX_SUCCESS
        }
        Err(e) => e,
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
        6 => {
            println!("Generate true random seed for rotation");
            generate_rot_seed()
        }
        7 => {
            println!("Export rotation seed");
            export_rot_seed()
        }
        8 => {
            println!("Import rotation seed");
            import_rot_seed()
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

    {
        let mut extra = KEY_MANAGER.extra_data.lock().unwrap();
        extra.next_mr_enclave = Some(sgx_measurement_t {
            m: msg_slice.try_into().unwrap(),
        });
    }
    KEY_MANAGER.save();

    info!(
        "Migration target approved. mr_encalve={}",
        hex::encode(msg_slice)
    );

    sgx_types::sgx_status_t::SGX_SUCCESS
}

fn load_offchain_signers(
    mut f_in: File,
    report: &sgx_report_body_t,
) -> std::collections::HashSet<[u8; 20]> {
    let mut json_data = String::new();
    f_in.read_to_string(&mut json_data).unwrap();

    // Deserialize the JSON string into a HashMap<String, String>
    let signatures: HashMap<String, (String, String)> =
        serde_json::from_str(&json_data).expect("Failed to deserialize JSON");

    let mut signers = std::collections::HashSet::new();

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

        if signers.insert(addr) {
            println!("  Approved by {}", addr_str);
        }
    }

    signers
}

#[cfg(feature = "verify-validator-whitelist")]
fn count_included_addresses(
    signers: &std::collections::HashSet<[u8; 20]>,
    list: &validator_whitelist::ValidatorList,
) -> usize {
    let mut res: usize = 0;

    for addr_str in &list.0 {
        let addr_vec = hex::decode(addr_str).unwrap();
        let addr: [u8; 20] = addr_vec.try_into().unwrap();

        if signers.contains(&addr) {
            res += 1;
        }
    }

    res
}

fn is_standard_consensus_reached(signers: &std::collections::HashSet<[u8; 20]>) -> bool {
    let mut total_voting_power: u64 = 0;
    let mut approved_power: u64 = 0;

    let validator_set = {
        let extra = KEY_MANAGER.extra_data.lock().unwrap();
        extra.decode_validator_set().unwrap()
    };

    for validator in validator_set.validators() {
        let power: u64 = validator.power.value();
        total_voting_power += power;

        let addr: [u8; 20] = validator.address.as_bytes().try_into().unwrap();
        if signers.contains(&addr) {
            approved_power += power;
        }
    }

    println!(
        "Total Power = {}, Approved Power = {}",
        total_voting_power, approved_power
    );

    if approved_power * 3 < total_voting_power * 2 {
        println!(" not enogh voting power");
        return false;
    }

    #[cfg(feature = "verify-validator-whitelist")]
    {
        let approved_whitelisted =
            count_included_addresses(signers, &validator_whitelist::VALIDATOR_WHITELIST);
        if approved_whitelisted < validator_whitelist::VALIDATOR_THRESHOLD {
            println!(
                " not enogh whitelisted validators: {}",
                approved_whitelisted
            );
            return false;
        }
    }
    true
}

fn is_export_approved_offchain(f_in: File, report: &sgx_report_body_t) -> bool {
    let signers = load_offchain_signers(f_in, report);

    let b1 = is_standard_consensus_reached(&signers);
    println!("Standard consensus reached: {}", b1);

    #[cfg(not(feature = "verify-validator-whitelist"))]
    let b2 = false;

    #[cfg(feature = "verify-validator-whitelist")]
    let b2 = {
        let approved_whitelisted = count_included_addresses(
            &signers,
            &validator_whitelist::VALIDATOR_WHITELIST_EMERGENCY,
        );
        println!(
            " Emergency whitelisted validators: {}",
            approved_whitelisted
        );

        approved_whitelisted >= validator_whitelist::VALIDATOR_THRESHOLD_EMERGENCY
    };

    println!("Emergency threshold reached: {}", b2);

    b1 || b2
}

fn is_export_approved(report: &sgx_report_body_t) -> bool {
    // Current policy: we demand the same mr_signer

    if report.mr_signer.m != SELF_REPORT_BODY.mr_signer.m {
        println!("Migration target uses different signer");
        return false;
    }

    {
        let extra = KEY_MANAGER.extra_data.lock().unwrap();
        if let Some(val) = extra.next_mr_enclave {
            if val.m == report.mr_enclave.m {
                println!("Migration is authorized by on-chain consensus.");
                return true;
            }
        }
    }

    if let Ok(f_in) = File::open(make_sgx_secret_path(FILE_MIGRATION_CONSENSUS).as_str()) {
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

fn get_rot_seed_file_params() -> (String, [u8; 16]) {
    let kdk = get_key_from_seed("seal.rot_seed".as_bytes());
    (make_sgx_secret_path("rot_seed.sealed"), kdk)
}

fn get_rot_seed_encrypted_path() -> String {
    make_sgx_secret_path("rot_seed_encr.bin")
}

fn save_rot_seed(rot_seed: &enclave_crypto::Seed) {
    let (path, kdk) = get_rot_seed_file_params();
    let mut file = SgxFile::create_ex(path, &kdk).unwrap();
    file.write_all(rot_seed.as_slice()).unwrap();
}

fn generate_rot_seed() -> sgx_status_t {
    let rot_seed = match enclave_crypto::Seed::new() {
        Ok(seed) => seed,
        Err(e) => {
            error!("Error generating random: {}", e);
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    };

    save_rot_seed(&rot_seed);
    println!("New seed generated");

    sgx_status_t::SGX_SUCCESS
}

fn get_dh_aes_key_from_report(other_report: &sgx_report_body_t, kp: &KeyPair) -> AESKey {
    let other_pub_k = &other_report.report_data.d[0..32].try_into().unwrap();
    AESKey::new_from_slice(&kp.diffie_hellman(other_pub_k))
}

fn get_seed_rot_report() -> SgxResult<sgx_report_body_t> {
    let next_report = match get_verified_migration_report_body() {
        Ok(report) => report,
        Err(e) => {
            error!("No migration report: {}", e);
            return Err(e);
        }
    };

    if next_report.mr_enclave.m != SELF_REPORT_BODY.mr_enclave.m {
        println!("Not eligible");
        return Err(sgx_status_t::SGX_ERROR_NO_PRIVILEGE);
    }

    Ok(next_report)
}

fn get_dh_aes_key_from_rot_report() -> SgxResult<AESKey> {
    match get_seed_rot_report() {
        Ok(r) => {
            let kp = Keychain::get_migration_keys();
            Ok(get_dh_aes_key_from_report(&r, &kp))
        }
        Err(e) => Err(e),
    }
}

fn read_rot_seed() -> SgxResult<enclave_crypto::Seed> {
    let (path, kdk) = get_rot_seed_file_params();
    let mut file = match SgxFile::open_ex(path, &kdk) {
        Ok(f) => f,
        Err(e) => {
            error!("can't open rot seed file: {}", e);
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    };

    let mut seed = enclave_crypto::Seed::default();
    match file.read_exact(seed.as_mut()) {
        Ok(()) => Ok(seed),
        Err(e) => {
            error!("can't read rot seed file: {}", e);
            Err(sgx_status_t::SGX_ERROR_UNEXPECTED)
        }
    }
}

fn export_rot_seed() -> sgx_status_t {
    let rot_seed = match read_rot_seed() {
        Ok(seed) => seed,
        Err(e) => return e,
    };

    let aes_key = match get_dh_aes_key_from_rot_report() {
        Ok(k) => k,
        Err(e) => return e,
    };

    let data_encrypted = aes_key.encrypt_siv(rot_seed.as_slice(), None).unwrap();

    //println!("ecnrypted seed candidate: {}", hex::encode(data_encrypted));

    let mut f_out = File::create(make_sgx_secret_path(&get_rot_seed_encrypted_path())).unwrap();
    f_out.write_all(&data_encrypted).unwrap();

    sgx_status_t::SGX_SUCCESS
}

fn import_rot_seed() -> sgx_status_t {
    let mut f_in = match File::open(get_rot_seed_encrypted_path()) {
        Err(e) => {
            error!("can't find encrypted seed: {}", e);
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
        Ok(f) => f,
    };

    let mut data_encrypted = Vec::new();
    f_in.read_to_end(&mut data_encrypted).unwrap();

    let aes_key = match get_dh_aes_key_from_rot_report() {
        Ok(k) => k,
        Err(e) => return e,
    };

    let data_plain = match aes_key.decrypt_siv(&data_encrypted, None) {
        Ok(res) => res,
        Err(err) => {
            error!("Can't decrypt: {}", err);
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    };

    if enclave_crypto::SEED_KEY_SIZE != data_plain.len() {
        error!("seed len mismatch");
    }

    let mut rot_seed = enclave_crypto::Seed::default();
    rot_seed.as_mut().copy_from_slice(&data_plain);

    save_rot_seed(&rot_seed);

    println!("Seed imported");
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
    let aes_key = get_dh_aes_key_from_report(&next_report, &kp);

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
    // TODO: disable in production build in the next version
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

pub fn calculate_validator_set_hash(
    validator_set_serialized: &[u8],
) -> SgxResult<tendermint::Hash> {
    match KeychainMutableData::decode_validator_set_ex(validator_set_serialized) {
        Some(res) => Ok(res.hash()),
        None => Err(sgx_status_t::SGX_ERROR_UNEXPECTED),
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

    #[cfg(feature = "light-client-validation")]
    {
        let mut verified_msgs = VERIFIED_BLOCK_MESSAGES.lock().unwrap();

        verified_msgs
            .next_validators_evidence
            .copy_from_slice(slice::from_raw_parts(val_set_evidence, evidence_len));
    }
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

    let validator_set_hash = {
        let extra = KEY_MANAGER.extra_data.lock().unwrap();

        match calculate_validator_set_hash(extra.validator_set_serialized.as_slice()) {
            Ok(tm_Sha256(hash)) => hash,
            _ => {
                error!("Got invalid validator set");
                return sgx_status_t::SGX_ERROR_UNEXPECTED;
            }
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

    {
        let mut extra = KEY_MANAGER.extra_data.lock().unwrap();

        if height != extra.height + 1 {
            if extra.height == height {
                // redundant call, skip
                return sgx_status_t::SGX_SUCCESS;
            }

            if extra.height != 0 {
                error!(
                    "Height range not consequent: current={}, submitted={}",
                    extra.height, height
                );
                return sgx_status_t::SGX_ERROR_UNEXPECTED;
            }
        }

        let validator_set_slice = slice::from_raw_parts(val_set, val_set_len as usize);
        let validator_set_hash = match calculate_validator_set_hash(validator_set_slice) {
            Ok(tm_Sha256(hash)) => hash,
            _ => {
                error!("invalid validator set");
                return sgx_status_t::SGX_ERROR_UNEXPECTED;
            }
        };

        #[cfg(feature = "light-client-validation")]
        {
            let expected_evidence = KEY_MANAGER.encrypt_hash(validator_set_hash, height);
            let verified_msgs = VERIFIED_BLOCK_MESSAGES.lock().unwrap();

            if verified_msgs.next_validators_evidence != expected_evidence {
                if extra.height != 0 {
                    error!("validator set evidence mismatch");
                    return sgx_status_t::SGX_ERROR_UNEXPECTED;
                }

                // We're given an initial validator set, without evidence. This covers the following cases:
                // 1. Start after bootstraping a new network
                // 2. Start after registration and sync normally (without using statesync)
                // in both cases the height should be 1 (i.e. we're at the very 1st block). And both cases are not applicable to production build.

                // Note that there MUST be a valid evidence for the following scenarios:
                // 1. Normal operation. The evidence is computed in submit_block_signatures
                // 2. Just after upgrate from legacy. The initial validator set is imported from legacy files, then computed as usual
                // 3. After statesync. The evidence for the initial validator set is downloaded with the whole state

                if height != 1 {
                    error!("Initial validator set height mismatch");
                    return sgx_status_t::SGX_ERROR_UNEXPECTED;
                }

                #[cfg(feature = "production")]
                {
                    error!("Initial validator set can't be set in production");
                    return sgx_status_t::SGX_ERROR_UNEXPECTED;
                }

                #[cfg(not(feature = "production"))]
                {
                    info!("Setting initial validator set");
                }
            }
        }

        {
            extra.height = height;
            extra.validator_set_serialized = validator_set_slice.to_vec();
        }
    }
    KEY_MANAGER.save();

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
