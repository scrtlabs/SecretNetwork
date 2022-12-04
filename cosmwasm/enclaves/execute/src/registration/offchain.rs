//!
/// These functions run off chain, and so are not limited by deterministic limitations. Feel free
/// to go crazy with random generation entropy, time requirements, or whatever else
///
use log::*;

use sgx_types::{sgx_status_t, SgxResult};
use std::slice;

use enclave_crypto::consts::{
    SigningMethod, ATTESTATION_CERT_PATH, ENCRYPTED_SEED_SIZE, IO_CERTIFICATE_SAVE_PATH,
    SEED_EXCH_CERTIFICATE_SAVE_PATH, SIGNATURE_TYPE,
};

use enclave_crypto::{KeyPair, Keychain, KEY_MANAGER, PUBLIC_KEY_SIZE};

use enclave_utils::pointers::validate_mut_slice;
use enclave_utils::storage::write_to_untrusted;
use enclave_utils::{validate_const_ptr, validate_mut_ptr};

use enclave_ffi_types::SINGLE_ENCRYPTED_SEED_SIZE;

use super::attestation::create_attestation_certificate;
use super::cert::verify_ra_cert;
use super::seed_service::get_next_consensus_seed_from_service;

use super::seed_exchange::decrypt_seed;

#[cfg(not(feature = "use_seed_service"))]
const EXPECTED_SEED_SIZE: u32 = 96;

#[cfg(feature = "use_seed_service")]
const EXPECTED_SEED_SIZE: u32 = 48;

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
    let api_key_slice = slice::from_raw_parts(api_key, api_key_len as usize);

    let mut key_manager = Keychain::new();

    if let Err(_e) = key_manager.create_consensus_seed() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    #[cfg(feature = "use_seed_service")]
    {
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
            enclave_crypto::consts::CONSENSUS_SEED_VERSION,
        ) {
            Ok(s) => s,
            Err(e) => {
                error!("Consensus seed failure: {}", e as u64);
                return sgx_status_t::SGX_ERROR_UNEXPECTED;
            }
        };

        if key_manager
            .set_consensus_seed(genesis_seed, new_consensus_seed)
            .is_err()
        {
            error!("failed to set new consensus seed");
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    }

    if let Err(_e) = key_manager.generate_consensus_master_keys() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(_e) = key_manager.create_registration_key() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let kp = key_manager.seed_exchange_key().unwrap();
    if let Err(status) =
        create_certificate_from_key(&kp.current, SEED_EXCH_CERTIFICATE_SAVE_PATH, api_key_slice)
    {
        return status;
    }

    let kp = key_manager.get_consensus_io_exchange_keypair().unwrap();
    if let Err(status) =
        create_certificate_from_key(&kp.current, IO_CERTIFICATE_SAVE_PATH, api_key_slice)
    {
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
    master_cert: *const u8,
    master_cert_len: u32,
    encrypted_seed: *const u8,
    encrypted_seed_len: u32,
    api_key: *const u8,
    api_key_len: u32,
) -> sgx_status_t {
    validate_const_ptr!(
        master_cert,
        master_cert_len as usize,
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

    let cert_slice = slice::from_raw_parts(master_cert, master_cert_len as usize);

    if encrypted_seed_len != ENCRYPTED_SEED_SIZE {
        error!("Encrypted seed bad length");
        return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    }

    let encrypted_seed_slice = slice::from_raw_parts(encrypted_seed, encrypted_seed_len as usize);

    // validate this node is patched and updated

    // generate temporary key for attestation
    let temp_key_result = KeyPair::new();

    if temp_key_result.is_err() {
        error!("Failed to generate temporary key for attestation");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    // this validates the cert and handles the "what if it fails" inside as well
    let res = create_attestation_certificate(
        temp_key_result.as_ref().unwrap(),
        SIGNATURE_TYPE,
        api_key_slice,
        None,
    );
    if res.is_err() {
        error!("Error starting node, might not be updated",);
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    //let encrypted_seed_slice = slice::from_raw_parts(encrypted_seed, encrypted_seed_len as usize);

    // let mut encrypted_seed = [0u8; ENCRYPTED_SEED_SIZE];
    // encrypted_seed.copy_from_slice(encrypted_seed_slice);

    if encrypted_seed_slice[0] as u32 != EXPECTED_SEED_SIZE {
        error!(
            "Got encrypted seed of different size than expected: {}",
            encrypted_seed_slice[0]
        );
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    // public keys in certificates don't have 0x04, so we'll copy it here
    let mut target_public_key: [u8; PUBLIC_KEY_SIZE] = [0u8; PUBLIC_KEY_SIZE];

    // validate master certificate - basically test that we're on the correct network
    let pk = match verify_ra_cert(cert_slice, Some(SigningMethod::MRSIGNER)) {
        Err(e) => {
            debug!("Error validating master certificate - {:?}", e);
            error!("Error validating network parameters. Are you on the correct network? (error code 0x01)");
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
        Ok(res) => res,
    };

    // just make sure the of the public key isn't messed up
    if pk.len() != PUBLIC_KEY_SIZE {
        error!(
            "Got public key from certificate with the wrong size: {:?}",
            pk.len()
        );
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }
    target_public_key.copy_from_slice(&pk);

    trace!(
        "ecall_init_node target public key is: {:?}",
        target_public_key
    );

    let mut key_manager = Keychain::new();

    // even though key is overwritten later we still want to explicitly remove it in case we increase the security version
    // to make sure that it is resealed using the new svn
    if let Err(_e) = key_manager.reseal_registration_key() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    };

    let delete_res = key_manager.delete_consensus_seed();
    if delete_res {
        debug!("Successfully removed consensus seed");
    } else {
        debug!("Failed to remove consensus seed. Didn't exist?");
    }

    let mut single_seed_bytes = [0u8; SINGLE_ENCRYPTED_SEED_SIZE];
    single_seed_bytes.copy_from_slice(&encrypted_seed_slice[1..(SINGLE_ENCRYPTED_SEED_SIZE + 1)]);

    trace!("Target public key is: {:?}", target_public_key);
    let genesis_seed = match decrypt_seed(&key_manager, target_public_key, single_seed_bytes) {
        Ok(result) => result,
        Err(status) => return status,
    };

    let new_consensus_seed;

    #[cfg(feature = "use_seed_service")]
    {
        let reg_key = key_manager.get_registration_key().unwrap();
        debug!("New consensus seed not found! Need to get it from service");
        if key_manager.get_consensus_seed().is_err() {
            new_consensus_seed = match get_next_consensus_seed_from_service(
                &mut key_manager,
                0,
                genesis_seed,
                api_key_slice,
                reg_key,
                crate::APP_VERSION,
            ) {
                Ok(s) => s,
                Err(e) => {
                    error!("Consensus seed failure: {}", e as u64);
                    return sgx_status_t::SGX_ERROR_UNEXPECTED;
                }
            };

            // TODO get current seed from seed server
            if let Err(_e) = key_manager.set_consensus_seed(genesis_seed, new_consensus_seed) {
                return sgx_status_t::SGX_ERROR_UNEXPECTED;
            }
        } else {
            debug!("New consensus seed already exists, no need to get it from service");
        }
    }

    #[cfg(not(feature = "use_seed_service"))]
    {
        debug!("Consensus seed service not active. Loading from registration");

        single_seed_bytes.copy_from_slice(
            &encrypted_seed_slice
                [(SINGLE_ENCRYPTED_SEED_SIZE + 1)..(SINGLE_ENCRYPTED_SEED_SIZE * 2 + 1)],
        );
        new_consensus_seed = match decrypt_seed(&key_manager, target_public_key, single_seed_bytes)
        {
            Ok(result) => result,
            Err(status) => return status,
        };

        // TODO get current seed from seed server
        if let Err(_e) = key_manager.set_consensus_seed(genesis_seed, new_consensus_seed) {
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    }

    // this initializes the key manager with all the keys we need for computations
    if let Err(_e) = key_manager.generate_consensus_master_keys() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    sgx_status_t::SGX_SUCCESS
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
) -> sgx_status_t {
    // validate_const_ptr!(spid, spid_len as usize, sgx_status_t::SGX_ERROR_UNEXPECTED);
    // let spid_slice = slice::from_raw_parts(spid, spid_len as usize);

    validate_const_ptr!(
        api_key,
        api_key_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );
    let api_key_slice = slice::from_raw_parts(api_key, api_key_len as usize);

    let kp = KEY_MANAGER.get_registration_key().unwrap();
    trace!(
        "ecall_get_attestation_report key pk: {:?}",
        &kp.get_pubkey().to_vec()
    );
    let (_private_key_der, cert) =
        match create_attestation_certificate(&kp, SIGNATURE_TYPE, api_key_slice, None) {
            Err(e) => {
                warn!("Error in create_attestation_certificate: {:?}", e);
                return e;
            }
            Ok(res) => res,
        };

    //let path_prefix = ATTESTATION_CERT_PATH.to_owned();
    if let Err(status) = write_to_untrusted(cert.as_slice(), ATTESTATION_CERT_PATH.as_str()) {
        return status;
    }

    #[cfg(feature = "SGX_MODE_HW")]
    {
        crate::registration::print_report::print_local_report_info(cert.as_slice());
    }

    sgx_status_t::SGX_SUCCESS
}

///
/// This function generates the registration_key, which is used in the attestation and registration
/// process
///
#[no_mangle]
pub unsafe extern "C" fn ecall_get_new_consensus_seed(
    seed_id: u32,
    api_key: *const u8,
    api_key_len: u32,
    // seed structure 1 byte - length (96 or 48) | genesis seed bytes | current seed bytes (optional)
    seed: &mut [u8; ENCRYPTED_SEED_SIZE as usize],
) -> sgx_status_t {
    let mut key_manager = Keychain::new();
    if key_manager.unseal_only_genesis().is_err() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let genesis_seed = key_manager.get_consensus_seed().unwrap().genesis;
    let registration_key = key_manager.get_registration_key().unwrap();
    let api_key_slice = slice::from_raw_parts(api_key, api_key_len as usize);

    match get_next_consensus_seed_from_service(
        &mut key_manager,
        0,
        genesis_seed,
        api_key_slice,
        registration_key,
        seed_id as u16,
    ) {
        Ok(s) => s,
        Err(e) => {
            error!("Consensus seed failure: {}", e as u64);
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    };

    sgx_status_t::SGX_SUCCESS
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
    trace!("ecall_key_gen key pk: {:?}", public_key.to_vec());
    sgx_status_t::SGX_SUCCESS
}

/// create_certificate_from_key takes a keypair and uses IAS to create a signed certificate with the
/// public key of the keypair in the payload
pub fn create_certificate_from_key(kp: &KeyPair, save_path: &str, api_key: &[u8]) -> SgxResult<()> {
    let (_, cert) = match create_attestation_certificate(kp, SIGNATURE_TYPE, api_key, None) {
        Err(e) => {
            error!("Error in create_attestation_certificate: {:?}", e);
            return Err(e);
        }
        Ok(res) => res,
    };

    if let Err(status) = write_to_untrusted(cert.as_slice(), save_path) {
        return Err(status);
    }

    Ok(())
}
