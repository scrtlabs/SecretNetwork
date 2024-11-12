//!
use super::attestation::{create_attestation_certificate, get_quote_ecdsa};
use super::seed_service::get_next_consensus_seed_from_service;
use crate::registration::attestation::verify_quote_ecdsa;
use crate::registration::onchain::split_combined_cert;
#[cfg(feature = "verify-validator-whitelist")]
use block_verifier::validator_whitelist;
use core::convert::TryInto;
use core::mem;
use ed25519_dalek::{PublicKey, Signature};
use enclave_crypto::consts::{
    ATTESTATION_CERT_PATH, ATTESTATION_DCAP_PATH, CERT_COMBINED_PATH, COLLATERAL_DCAP_PATH,
    CONSENSUS_SEED_VERSION, INPUT_ENCRYPTED_SEED_SIZE, MIGRATION_APPROVAL_PATH,
    MIGRATION_CERT_PATH, MIGRATION_CONSENSUS_PATH, PUBKEY_PATH, SEED_UPDATE_SAVE_PATH,
    SIGNATURE_TYPE,
};
use enclave_crypto::{sha_256, KeyPair, Keychain, SIVEncryptable, KEY_MANAGER, PUBLIC_KEY_SIZE};
use enclave_ffi_types::SINGLE_ENCRYPTED_SEED_SIZE;
use enclave_utils::pointers::validate_mut_slice;
use enclave_utils::storage::export_all_to_kdk_safe;
use enclave_utils::storage::migrate_all_from_2_17;
use enclave_utils::storage::SEALING_KDK;
use enclave_utils::storage::SELF_REPORT_BODY;
use enclave_utils::validator_set::ValidatorSetForHeight;
use enclave_utils::{validate_const_ptr, validate_mut_ptr};
/// These functions run off chain, and so are not limited by deterministic limitations. Feel free
/// to go crazy with random generation entropy, time requirements, or whatever else
///
use log::*;
use sgx_trts::trts::rsgx_read_rand;
use sgx_types::sgx_measurement_t;
use sgx_types::sgx_report_body_t;
use sgx_types::sgx_status_t;
use sgx_types::SgxResult;
use sha2::{Digest, Sha256};
use std::collections::HashMap;
use std::fs::File;
use std::io::prelude::*;
use std::panic;
use std::sgxfs::SgxFile;
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

    let mut key_manager = Keychain::new();

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

    let out_path: &String = if is_migration_report {
        &MIGRATION_CERT_PATH
    } else {
        &CERT_COMBINED_PATH
    };

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

fn dh_xor(my_k: &KeyPair, other_k: &[u8; 32], data: &mut [u8; 16]) {
    let dhk = my_k.diffie_hellman(other_k);
    for i in 0..16 {
        data[i] ^= dhk[i] ^ dhk[i + 16];
    }
}

fn get_report_body(path: &str) -> sgx_report_body_t {
    let (_, vec_quote, vec_coll) = {
        let mut f_in = File::open(path).unwrap();
        let mut cert = vec![];
        f_in.read_to_end(&mut cert).unwrap();

        split_combined_cert(cert.as_ptr(), cert.len() as u32)
    };

    verify_quote_ecdsa(vec_quote.as_slice(), vec_coll.as_slice(), 0)
        .unwrap()
        .0
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
            let prev_report = get_report_body(CERT_COMBINED_PATH.as_str());

            let nnc = KeyPair::new().unwrap();
            let pub_k = &prev_report.report_data.d[0..32].try_into().unwrap();

            let kdk: &[u8; 16] = &SEALING_KDK;
            //trace!("*** Sealing kdk: {:?}", kdk);

            let dst: &mut [u8; 16] = (&mut report_data[32..48]).try_into().unwrap();
            dst.copy_from_slice(kdk);

            dh_xor(&nnc, pub_k, dst);

            (nnc, true)
        }
        _ => {
            // standard network registration report
            let kp = KEY_MANAGER.get_registration_key().unwrap();
            trace!(
                "ecall_get_attestation_report key pk: {:?}",
                hex::encode(&kp.get_pubkey().to_vec())
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

    let mut key_manager = Keychain::new();
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
    trace!(
        "ecall_key_gen key pk: {:?}",
        hex::encode(public_key.to_vec())
    );
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
pub unsafe extern "C" fn ecall_migrate_sealing() -> sgx_types::sgx_status_t {
    migrate_all_from_2_17()
}

#[repr(packed)]
pub struct MigrationApprovalData {
    pub mr_enclave: sgx_measurement_t,
    //    pub mr_signer: sgx_measurement_t,
}

impl MigrationApprovalData {
    fn is_export_approved(&self, report: &sgx_report_body_t) -> bool {
        if self.mr_enclave.m != report.mr_enclave.m {
            info!("mrenclave mismatch");
            return false;
        }

        true
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

    let data = MigrationApprovalData {
        mr_enclave: sgx_measurement_t {
            m: msg_slice.try_into().unwrap(),
        },
    };

    unsafe {
        let d = std::slice::from_raw_parts(
            (&data as *const MigrationApprovalData) as *const u8,
            mem::size_of::<MigrationApprovalData>(),
        );

        let mut f_out = SgxFile::create(MIGRATION_APPROVAL_PATH.as_str()).unwrap();
        f_out.write_all(d).unwrap();

        info!(
            "Migration target approved. mr_encalve={}",
            hex::encode(data.mr_enclave.m)
        );
    }

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
        let validator_set_vec = ValidatorSetForHeight::unseal().unwrap().validator_set;
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

    if let Ok(mut f_in) = SgxFile::open(MIGRATION_APPROVAL_PATH.as_str()) {
        let mut data = vec![];
        f_in.read_to_end(&mut data).unwrap();

        if data.len() != mem::size_of::<MigrationApprovalData>() {
            panic!("wrong file size");
        }

        let res = unsafe {
            let p_data = data.as_ptr() as *const MigrationApprovalData;
            (*p_data).is_export_approved(report)
        };

        if res {
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

#[no_mangle]
pub unsafe extern "C" fn ecall_export_sealing() -> sgx_types::sgx_status_t {
    // migration report
    let mut next_report = get_report_body(MIGRATION_CERT_PATH.as_str());

    if !is_export_approved(&next_report) {
        error!("Export sealing not authorized");
        return sgx_status_t::SGX_ERROR_NO_PRIVILEGE;
    }

    let pub_k = &next_report.report_data.d[0..32].try_into().unwrap();
    let kdk: &mut [u8; 16] = (&mut next_report.report_data.d[32..48]).try_into().unwrap();

    let kp = KEY_MANAGER.get_registration_key().unwrap();

    dh_xor(&kp, pub_k, kdk);
    //trace!("*** Sealing kdk: {:?}", kdk);

    export_all_to_kdk_safe(kdk)
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

pub fn get_validator_set_hash() -> SgxResult<tendermint::Hash> {
    let res = ValidatorSetForHeight::unseal()?;

    let hash = match <tendermint::validator::Set as Protobuf<
        tendermint_proto::v0_38::types::ValidatorSet,
    >>::decode(&*(res.validator_set))
    {
        Ok(vs) => {
            debug!("decoded validator set hash: {:?}", vs.hash());
            vs.hash()
        }
        Err(e) => {
            error!("error decoding validator set: {:?}", e);
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    };

    Ok(hash)
}

/// # Safety
/// make sure to check that block_hash is a valid pointer and that it's exactly 32 bytes long
#[no_mangle]
pub unsafe extern "C" fn ecall_generate_random(
    block_hash: *const u8,
    block_hash_len: u32,
    height: u64,
    random: &mut [u8; ENCRYPTED_RANDOM_LENGTH as usize],
    proof: &mut [u8; PROOF_LENGTH as usize],
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
    let block_hash_slice = slice::from_raw_parts(block_hash, block_hash_len as usize);

    let mut rand_buf: [u8; 32] = [0; 32];

    if let Err(_e) = rsgx_read_rand(&mut rand_buf) {
        error!("Error generating random value");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    };

    let validator_set_hash = match get_validator_set_hash().unwrap_or_default() {
        tm_Sha256(hash) => hash,
        tendermint::Hash::None => {
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
        let proof_computed = enclave_utils::random::create_random_proof(
            &KEY_MANAGER.initial_randomness_seed.unwrap(),
            height,
            encrypted.as_slice(),
            block_hash_slice,
        );
        proof.copy_from_slice(proof_computed.as_slice());
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

    let val_set_slice = slice::from_raw_parts(val_set, val_set_len as usize);

    let val_set = ValidatorSetForHeight {
        height,
        validator_set: val_set_slice.to_vec(),
    };

    let res = val_set.seal();
    if res.is_err() {
        return sgx_status_t::SGX_ERROR_ENCLAVE_FILE_ACCESS;
    }

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
    height: u64,
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

    let random_slice = slice::from_raw_parts(random, random_len as usize);
    let proof_slice = slice::from_raw_parts(proof, proof_len as usize);
    let block_hash_slice = slice::from_raw_parts(block_hash, block_hash_len as usize);

    #[cfg(feature = "random")]
    {
        let calculated_proof = enclave_utils::random::create_random_proof(
            &KEY_MANAGER.initial_randomness_seed.unwrap(),
            height,
            random_slice,
            block_hash_slice,
        );

        // debug!("Calculated proof: {:?}", calculated_proof);
        // debug!("Got proof: {:?}", proof_slice);

        if calculated_proof != proof_slice {
            // otherwise on an upgrade this will break horribly - next patch we can remove this
            let legacy_proof = create_legacy_proof(height, random_slice, block_hash_slice);
            if legacy_proof != calculated_proof {
                return sgx_status_t::SGX_ERROR_INVALID_SIGNATURE;
            }
        }
    }

    sgx_status_t::SGX_SUCCESS
}

fn create_legacy_proof(height: u64, random: &[u8], block_hash: &[u8]) -> [u8; 32] {
    let mut data = vec![];
    data.extend_from_slice(&height.to_be_bytes());
    data.extend_from_slice(random);
    data.extend_from_slice(block_hash);
    data.extend_from_slice(KEY_MANAGER.initial_randomness_seed.unwrap().get());

    sha_256(data.as_slice())
}
