use enclave_ffi_types::{Ctx, EnclaveBuffer, HandleResult, InitResult, QueryResult};
use std::ffi::c_void;

use crate::keys::{KeyPair, PubKey};
use crate::results::{
    result_handle_success_to_handleresult, result_init_success_to_initresult,
    result_query_success_to_queryresult,
};
use log::*;
use sgx_trts::trts::{
    rsgx_lfence, rsgx_raw_is_outside_enclave, rsgx_sfence, rsgx_slice_is_outside_enclave,
};
use sgx_types::{sgx_quote_sign_type_t, sgx_report_t, sgx_status_t, sgx_target_info_t};
use std::ptr::null;
use std::slice;

use crate::keys::init_seed;

#[cfg(feature = "SGX_MODE_HW")]
use crate::attestation::create_attestation_report;
#[cfg(feature = "SGX_MODE_HW")]
use crate::attestation::create_attestation_certificate;

#[cfg(not(feature = "SGX_MODE_HW"))]
use crate::attestation::{create_report_with_data, software_mode_quote};
use crate::cert::verify_mra_cert;
use crate::storage::{write_to_untrusted, SealedKey, NODE_SK_SEALING_PATH};

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
pub unsafe extern "C" fn ecall_key_gen(pk_node: *mut PubKey) -> sgx_types::sgx_status_t {
    // Generate node-specific key-pair
    let key_pair = match KeyPair::new() {
        Ok(kp) => kp,
        Err(err) => return sgx_status_t::SGX_ERROR_UNEXPECTED,
    };

    // let privkey = key_pair.get_privkey();
    match key_pair.seal(NODE_SK_SEALING_PATH) {
        Err(err) => return sgx_status_t::SGX_ERROR_UNEXPECTED,
        Ok(_) => { /* continue */ }
    }; // can read with SecretKey::from_slice()

    let pubkey = key_pair.get_pubkey();

    (&mut *pk_node).clone_from_slice(&pubkey);
    sgx_status_t::SGX_SUCCESS
}

#[cfg(feature = "SGX_MODE_HW")]
#[no_mangle]
pub extern "C" fn ecall_get_attestation_report() -> sgx_status_t {
    let (private_key_der, cert) =
        match create_attestation_certificate(sgx_quote_sign_type_t::SGX_UNLINKABLE_SIGNATURE) {
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

#[cfg(not(feature = "SGX_MODE_HW"))]
#[no_mangle]
pub extern "C" fn ecall_get_attestation_report() -> sgx_status_t {
    software_mode_quote()
}

#[cfg(not(feature = "SGX_MODE_HW"))]
#[no_mangle]
// todo: replace 32 with crypto consts once I have crypto library
pub extern "C" fn ecall_get_encrypted_seed(
    cert: *const u8,
    cert_len: u32,
    seed: &mut [u8; 32],
) -> sgx_status_t {
    // just return the seed
    sgx_status_t::SGX_SUCCESS
}

#[no_mangle]
pub extern "C" fn ecall_init_bootstrap() -> sgx_status_t {
    // Generate node-specific key-pair
    let key_pair = match KeyPair::new() {
        Ok(kp) => kp,
        Err(err) => return sgx_status_t::SGX_ERROR_UNEXPECTED,
    };

    // let privkey = key_pair.get_privkey();
    match key_pair.seal(NODE_SK_SEALING_PATH) {
        Err(err) => return sgx_status_t::SGX_ERROR_UNEXPECTED,
        Ok(_) => { /* continue */ }
    }; // can read with SecretKey::from_slice()

    sgx_status_t::SGX_SUCCESS
}

#[cfg(feature = "SGX_MODE_HW")]
#[no_mangle]
// todo: replace 32 with crypto consts once I have crypto library
pub extern "C" fn ecall_get_encrypted_seed(
    cert: *const u8,
    cert_len: u32,
    seed: &mut [u8; 32],
) -> sgx_status_t {
    if rsgx_slice_is_outside_enclave(seed) {
        error!("Tried to access memory outside enclave -- rsgx_slice_is_outside_enclave");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }
    rsgx_sfence();

    if cert.is_null() || cert_len == 0 {
        error!("Tried to access an empty pointer - cert.is_null()");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }
    rsgx_lfence();

    let cert_slice = unsafe { std::slice::from_raw_parts(cert, cert_len as usize) };

    let pk = match verify_mra_cert(cert_slice) {
        Err(e) => {
            error!("Error in validating certificate: {:?}", e);
            return e;
        }
        Ok(res) => res,
    };

    let test_result = [41u8; 32];

    info!("Hello from seed copying!");

    seed.copy_from_slice(&test_result);
    // calc encrypted seed

    // return seed

    sgx_status_t::SGX_SUCCESS
}

#[no_mangle]
pub unsafe extern "C" fn ecall_init_seed(
    public_key: *const u8,
    public_key_len: u32,
    encrypted_seed: *const u8,
    encrypted_seed_len: u32,
) -> sgx_status_t {
    if public_key.is_null() || public_key_len == 0 {
        error!("Tried to access an empty pointer - public_key.is_null()");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }
    rsgx_lfence();

    if encrypted_seed.is_null() || encrypted_seed_len == 0 {
        error!("Tried to access an empty pointer - encrypted_seed.is_null()");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }
    rsgx_lfence();

    let public_key_slice = slice::from_raw_parts(public_key, public_key_len as usize);
    let encrypted_seed_slice = slice::from_raw_parts(encrypted_seed, encrypted_seed_len as usize);
    init_seed(public_key_slice, encrypted_seed_slice)
}
