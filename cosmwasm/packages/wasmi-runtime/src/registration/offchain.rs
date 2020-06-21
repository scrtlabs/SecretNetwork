///
/// These functions run off chain, and so are not limited by deterministic limitations. Feel free
/// to go crazy with random generation entropy, time requirements, or whatever else
///
use log::*;
use sgx_types::{sgx_quote_sign_type_t, sgx_status_t};

use std::slice;

use crate::consts::{
    ATTESTATION_CERTIFICATE_SAVE_PATH, ENCRYPTED_SEED_SIZE, IO_CERTIFICATE_SAVE_PATH,
    SEED_EXCH_CERTIFICATE_SAVE_PATH,
};
use crate::crypto::{Keychain, KEY_MANAGER, PUBLIC_KEY_SIZE};
use crate::storage::write_to_untrusted;
use crate::utils::{attest_from_key, validate_const_ptr, validate_mut_ptr, validate_mut_slice};

use super::attestation::create_attestation_certificate;
use super::cert::verify_ra_cert;
use super::seed_exchange::decrypt_seed;

///
/// `ecall_init_bootstrap`
///
/// Function to handle the initialization of the bootstrap node. Generates the master private/public
/// key (seed + pk_io/sk_io). This happens once at the initialization of a chain. Returns the master
/// public key (pk_io), which is saved on-chain, and used to propagate the seed to registering nodes
///
///
#[no_mangle]
pub extern "C" fn ecall_init_bootstrap(public_key: &mut [u8; PUBLIC_KEY_SIZE]) -> sgx_status_t {
    if let Err(_e) = validate_mut_ptr(public_key.as_mut_ptr(), public_key.len()) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let mut key_manager = Keychain::new();

    if let Err(_e) = key_manager.create_consensus_seed() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(_e) = key_manager.generate_consensus_master_keys() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(_e) = key_manager.create_registration_key() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let kp = key_manager.seed_exchange_key().unwrap();
    if let Err(status) = attest_from_key(&kp, SEED_EXCH_CERTIFICATE_SAVE_PATH) {
        return status;
    }

    let kp = key_manager.get_consensus_io_exchange_keypair().unwrap();
    if let Err(status) = attest_from_key(&kp, IO_CERTIFICATE_SAVE_PATH) {
        return status;
    }

    public_key.copy_from_slice(&key_manager.seed_exchange_key().unwrap().get_pubkey());
    debug!(
        "ecall_init_bootstrap consensus_seed_exchange_keypair public key: {:?}",
        &public_key.to_vec()
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
/// # Safety
///  Something should go here
///
#[no_mangle]
pub unsafe extern "C" fn ecall_init_node(
    master_cert: *const u8,
    master_cert_len: u32,
    encrypted_seed: *const u8,
    encrypted_seed_len: u32,
) -> sgx_status_t {
    if let Err(_e) = validate_const_ptr(master_cert, master_cert_len as usize) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(_e) = validate_const_ptr(encrypted_seed, encrypted_seed_len as usize) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let cert_slice = slice::from_raw_parts(master_cert, master_cert_len as usize);

    if (encrypted_seed_len as usize) != ENCRYPTED_SEED_SIZE {
        error!(
            "Got encrypted seed with the wrong size: {:?}",
            encrypted_seed_len
        );
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let encrypted_seed_slice = slice::from_raw_parts(encrypted_seed, encrypted_seed_len as usize);

    let mut encrypted_seed = [0u8; ENCRYPTED_SEED_SIZE];
    encrypted_seed.copy_from_slice(&encrypted_seed_slice);

    // public keys in certificates don't have 0x04, so we'll copy it here
    let mut target_public_key: [u8; PUBLIC_KEY_SIZE] = [0u8; PUBLIC_KEY_SIZE];

    // validate certificate w/ attestation report
    let pk = match verify_ra_cert(cert_slice) {
        Err(e) => {
            error!("Error in validating certificate: {:?}", e);
            return e;
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

    let mut key_manager = Keychain::new();
    let seed = match decrypt_seed(&key_manager, target_public_key, encrypted_seed) {
        Ok(result) => result,
        Err(status) => return status,
    };

    if let Err(_e) = key_manager.set_consensus_seed(seed) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

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
 */
pub extern "C" fn ecall_get_attestation_report() -> sgx_status_t {
    let kp = KEY_MANAGER.get_registration_key().unwrap();
    info!(
        "ecall_get_attestation_report key pk: {:?}",
        &kp.get_pubkey().to_vec()
    );
    let (_private_key_der, cert) = match create_attestation_certificate(
        &kp,
        sgx_quote_sign_type_t::SGX_UNLINKABLE_SIGNATURE,
    ) {
        Err(e) => {
            error!("Error in create_attestation_certificate: {:?}", e);
            return e;
        }
        Ok(res) => res,
    };

    if let Err(status) = write_to_untrusted(cert.as_slice(), ATTESTATION_CERTIFICATE_SAVE_PATH) {
        return status;
    }

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

    let pubkey = key_manager.get_registration_key().unwrap().get_pubkey();
    public_key.clone_from_slice(&pubkey);
    // todo: remove this before production O.o
    info!("ecall_key_gen key pk: {:?}", public_key.to_vec());
    sgx_status_t::SGX_SUCCESS
}
