use enclave_ffi_types::{Ctx, EnclaveBuffer, HandleResult, InitResult, QueryResult};
use std::ffi::c_void;

use crate::crypto;
use crate::crypto::{Keychain,
                    PubKey,
                    AESKey,
                    KeyPair,
                    Seed,
                    SEED_KEY_SIZE,
                    UNCOMPRESSED_PUBLIC_KEY_SIZE,
                    PUBLIC_KEY_SIZE,
                    };
use crate::results::{
    result_handle_success_to_handleresult, result_init_success_to_initresult,
    result_query_success_to_queryresult,
};
use log::*;
use sgx_types::*;
use sgx_trts::trts::{
    rsgx_lfence, rsgx_raw_is_outside_enclave, rsgx_sfence, rsgx_slice_is_outside_enclave
};
use sgx_types::{sgx_quote_sign_type_t, sgx_status_t};
use std::slice;

use crate::utils::{validate_const_ptr, validate_mut_ptr};

use crate::consts::{NODE_SK_SEALING_PATH, SEED_SEALING_PATH, IO_KEY_SEALING_KEY_PATH, ENCRYPTED_SEED_SIZE};
pub use crate::crypto::traits::{SealedKey, Encryptable, Kdf};

use crate::cert::verify_ra_cert;
use crate::attestation::create_attestation_certificate;

use crate::storage::write_to_untrusted;

#[no_mangle]
pub extern "C" fn ecall_allocate(buffer: *const u8, length: usize) -> EnclaveBuffer {
    let slice = unsafe { std::slice::from_raw_parts(buffer, length) };
    let vector_copy = slice.to_vec();
    let boxed_vector = Box::new(vector_copy);
    let heap_pointer = Box::into_raw(boxed_vector);
    EnclaveBuffer {
        ptr: heap_pointer as *mut c_void,
    }
}

/// Take a pointer as returned by `ecall_allocate` and recover the Vec<u8> inside of it.
pub unsafe fn recover_buffer(ptr: EnclaveBuffer) -> Option<Vec<u8>> {
    if ptr.ptr.is_null() {
        return None;
    }
    let boxed_vector = Box::from_raw(ptr.ptr as *mut Vec<u8>);
    Some(*boxed_vector)
}

#[no_mangle]
pub extern "C" fn ecall_init(
    context: Ctx,
    gas_limit: u64,
    contract: *const u8,
    contract_len: usize,
    env: *const u8,
    env_len: usize,
    msg: *const u8,
    msg_len: usize,
) -> InitResult {
    let contract = unsafe { std::slice::from_raw_parts(contract, contract_len) };
    let env = unsafe { std::slice::from_raw_parts(env, env_len) };
    let msg = unsafe { std::slice::from_raw_parts(msg, msg_len) };

    let result = super::contract_operations::init(context, gas_limit, contract, env, msg);
    result_init_success_to_initresult(result)
}

#[no_mangle]
pub extern "C" fn ecall_handle(
    context: Ctx,
    gas_limit: u64,
    contract: *const u8,
    contract_len: usize,
    env: *const u8,
    env_len: usize,
    msg: *const u8,
    msg_len: usize,
) -> HandleResult {
    let contract = unsafe { std::slice::from_raw_parts(contract, contract_len) };
    let env = unsafe { std::slice::from_raw_parts(env, env_len) };
    let msg = unsafe { std::slice::from_raw_parts(msg, msg_len) };

    let result = super::contract_operations::handle(context, gas_limit, contract, env, msg);
    result_handle_success_to_handleresult(result)
}

#[no_mangle]
pub extern "C" fn ecall_query(
    context: Ctx,
    gas_limit: u64,
    contract: *const u8,
    contract_len: usize,
    msg: *const u8,
    msg_len: usize,
) -> QueryResult {
    let contract = unsafe { std::slice::from_raw_parts(contract, contract_len) };
    let msg = unsafe { std::slice::from_raw_parts(msg, msg_len) };

    let result = super::contract_operations::query(context, gas_limit, contract, msg);
    result_query_success_to_queryresult(result)
}

// gen (sk_node,pk_node) keypair for new node registration
#[no_mangle]
pub unsafe extern "C" fn ecall_key_gen(public_key: &mut [u8; PUBLIC_KEY_SIZE]) -> sgx_types::sgx_status_t {

    if rsgx_slice_is_outside_enclave(public_key) {
        error!("Tried to access memory outside enclave -- rsgx_slice_is_outside_enclave");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }
    rsgx_sfence();

    let mut key_manager = Keychain::new();

    key_manager.create_node_key();

    let pubkey = key_manager.get_node_key().unwrap().get_pubkey();
    info!("ecall_key_gen key pk: {:?}", public_key.to_vec());
    public_key.clone_from_slice(&pubkey[1..UNCOMPRESSED_PUBLIC_KEY_SIZE]);
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
    let mut key_manager = Keychain::new();
    let kp = key_manager.get_node_key().unwrap();
    info!("ecall_get_attestation_report key pk: {:?}", &kp.get_pubkey().to_vec());
    let (private_key_der, cert) =
        match create_attestation_certificate(&kp, sgx_quote_sign_type_t::SGX_UNLINKABLE_SIGNATURE) {
            Err(e) => {
                error!("Error in create_attestation_certificate: {:?}", e);
                return e;
            }
            Ok(res) => res,
        };
    // info!("private key {:?}, cert: {:?}", private_key_der, cert);

    if let Err(status) = write_to_untrusted(cert.as_slice(), "attestation_cert.der") {
        return status;
    }
    //seal(private_key_der, "ecc_cert_private.der")
    sgx_status_t::SGX_SUCCESS
}

/**
 * `ecall_init_bootstrap`
 *
 *  Function to handle the initialization of the bootstrap node. Generates the master private/public
 *  key (seed + pk_io/sk_io). This happens once at the initialization of a chain. Returns the master
 *  public key (pk_io), which is saved on-chain, and used to propagate the seed to registering nodes
 *
 */
#[no_mangle]
pub extern "C" fn ecall_init_bootstrap(public_key: &mut [u8; PUBLIC_KEY_SIZE]) -> sgx_status_t {

    if let Err(e) = validate_mut_ptr(public_key.as_mut_ptr(), public_key.len()) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let mut key_manager = Keychain::new();

    if let Err(e) = key_manager.create_seed() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(e) = key_manager.generate_master_keys() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let mut key_manager = Keychain::new();
    let kp = key_manager.get_io_key().unwrap();
    let (_, cert) =
        match create_attestation_certificate(&kp, sgx_quote_sign_type_t::SGX_UNLINKABLE_SIGNATURE) {
            Err(e) => {
                error!("Error in create_attestation_certificate: {:?}", e);
                return e;
            }
            Ok(res) => res,
        };
    // info!("private key {:?}, cert: {:?}", private_key_der, cert);

    if let Err(status) = write_to_untrusted(cert.as_slice(), "attestation_cert.der") {
        return status;
    }

    // don't want to copy the first byte (no need to pass the 0x4 uncompressed byte)
    public_key.copy_from_slice(&key_manager.get_io_key().unwrap().get_pubkey()[1..UNCOMPRESSED_PUBLIC_KEY_SIZE]);
    debug!("ecall_init_bootstrap key pk: {:?}", &public_key.to_vec());

    sgx_status_t::SGX_SUCCESS
}

/**
  *  `ecall_get_encrypted_seed`
  *
  *  This call is used to help new nodes register in the network. The function will authenticate the
  *  new node, based on a received certificate. If the node is authenticated successfully, the seed
  *  will be encrypted and shared with the registering node.
  *
  *  The seed is encrypted with a key derived from the secret master key of the chain, and the public
  *  key of the requesting chain
  *
  */
#[no_mangle]
// todo: replace 32 with crypto consts once I have crypto library
pub extern "C" fn ecall_get_encrypted_seed(
    cert: *const u8,
    cert_len: u32,
    seed: &mut [u8; ENCRYPTED_SEED_SIZE]
) -> sgx_status_t {

    if let Err(e) = validate_mut_ptr(seed.as_mut_ptr(), seed.len()) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(e) = validate_const_ptr(cert, cert_len as usize) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let cert_slice = unsafe { std::slice::from_raw_parts(cert, cert_len as usize) };
    let key_manager = Keychain::new();

    // verify certificate, and return the public key in the extra data of the report
    let pk = match verify_ra_cert(cert_slice) {
        Err(e) => {
            error!("Error in validating certificate: {:?}", e);
            return e;
        }
        Ok(res) => res,
    };

    // just make sure the length isn't wrong for some reason (certificate may be malformed)
    if pk.len() != crypto::PUBLIC_KEY_SIZE {
        error!("Got public key from certificate with the wrong size: {:?}", pk.len());
        return sgx_status_t::SGX_ERROR_UNEXPECTED
    }

    let mut target_public_key: [u8; 65] = [4u8; 65];

    target_public_key[1..].copy_from_slice(&pk);
    debug!("ecall_get_encrypted_seed target_public_key key pk: {:?}", &target_public_key.to_vec());

    let shared_enc_key = match key_manager.get_io_key().unwrap().derive_key(&target_public_key) {
        Ok(r) => r,
        Err(e) => {
            return sgx_status_t::SGX_ERROR_UNEXPECTED
        }
    };

    // encrypt the seed using the symmetric key derived in the previous stage
    let res = match AESKey::new_from_slice(&shared_enc_key).encrypt(
        &key_manager.get_seed().unwrap().get().to_vec()) {
        Ok(r) => {
            if r.len() != ENCRYPTED_SEED_SIZE {
                error!("wtf? {:?}", r.len());
                return sgx_status_t::SGX_ERROR_UNEXPECTED
            }
            r
        },
        Err(e) => {
            return sgx_status_t::SGX_ERROR_UNEXPECTED
        }
    };

    seed.copy_from_slice(&res);

    sgx_status_t::SGX_SUCCESS
}

/**
  *  `ecall_init_seed`
  *
  *  This function is called during initialization of __non__ bootstrap nodes.
  *
  *  It receives the master public key (pk_io) and uses it, and its node key (generated in [ecall_key_gen])
  *  to decrypt the seed.
  *
  *  The seed was encrypted using Diffie-Hellman in the function [ecall_get_encrypted_seed]
  *
  */
#[no_mangle]
pub unsafe extern "C" fn ecall_init_seed(
    master_cert: *const u8,
    master_cert_len: u32,
    encrypted_seed: *const u8,
    encrypted_seed_len: u32,
) -> sgx_status_t {
    if let Err(e) = validate_const_ptr(master_cert, master_cert_len as usize) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(e) = validate_const_ptr(encrypted_seed, encrypted_seed_len as usize) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let mut key_manager = Keychain::new();

    let cert_slice = slice::from_raw_parts(master_cert, master_cert_len as usize);
    let encrypted_seed_slice = slice::from_raw_parts(encrypted_seed, encrypted_seed_len as usize);

    let mut target_public_key: [u8; UNCOMPRESSED_PUBLIC_KEY_SIZE] = [4u8; UNCOMPRESSED_PUBLIC_KEY_SIZE];

    let pk = match verify_ra_cert(cert_slice) {
        Err(e) => {
            error!("Error in validating certificate: {:?}", e);
            return e;
        }
        Ok(res) => res,
    };
    // just make sure the length isn't wrong for some reason (certificate may be malformed)
    if pk.len() != crypto::PUBLIC_KEY_SIZE {
        error!("Got public key from certificate with the wrong size: {:?}", pk.len());
        return sgx_status_t::SGX_ERROR_UNEXPECTED
    }
    target_public_key[1..].copy_from_slice(&pk);

    let shared_enc_key = match key_manager.get_node_key().unwrap().derive_key(&target_public_key) {
        Ok(r) => r,
        Err(e) => {
            return sgx_status_t::SGX_ERROR_UNEXPECTED
        }
    };

    let res = match AESKey::new_from_slice(&shared_enc_key).decrypt(&encrypted_seed_slice) {
        Ok(r) => {
            if r.len() != SEED_KEY_SIZE {
                error!("wtf2? {:?}", r.len());
                return sgx_status_t::SGX_ERROR_UNEXPECTED
            }
            r
        },
        Err(e) => {
            return sgx_status_t::SGX_ERROR_UNEXPECTED
        }
    };

    let mut seed_buf: [u8; 32] = [0u8; 32];
    seed_buf.copy_from_slice(&res);

    info!("Decrypted seed: {:?}", seed_buf);

    let seed = Seed::new_from_slice(&seed_buf);

    if let Err(e) = key_manager.set_seed(seed) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(e) = key_manager.generate_master_keys() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    sgx_status_t::SGX_SUCCESS
}
