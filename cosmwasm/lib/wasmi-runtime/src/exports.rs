use enclave_ffi_types::{Ctx, EnclaveBuffer, HandleResult, InitResult, KeyGenResult, QueryResult};
use crate::results::{
    result_handle_success_to_handleresult, result_init_success_to_initresult,
    result_query_success_to_queryresult,
};
use std::ffi::c_void;
use std::ptr::null;

use log::*;

use sgx_types::{sgx_report_t, sgx_target_info_t, sgx_status_t, sgx_quote_sign_type_t};

#[cfg(feature = "SGX_MODE_HW")]
use crate::attestation::{create_attestation_report};

#[cfg(not(feature = "SGX_MODE_HW"))]
use crate::attestation::{create_report_with_data, software_mode_quote};
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

#[no_mangle]
pub extern "C" fn ecall_key_gen() -> KeyGenResult {
    // Generate node-specific key-pair
    // let key_pair = match KeyPair::new() {
    //     Ok(kp) => kp,
    //     Err(err) => return KeyGenResult::Failure { err },
    // };

    todo!();

    // let pk = key_pair.privkey;
    // seal(pk.serialize(), "dsacdsa");

    // seal(key_pair)
}

#[cfg(feature = "SGX_MODE_HW")]
#[no_mangle]
pub extern "C" fn ecall_get_attestation_report() -> sgx_status_t {
    let (private_key_der, cert) = match create_attestation_certificate(sgx_quote_sign_type_t::SGX_UNLINKABLE_SIGNATURE) {
        Err(e) => {
            error!("Error in create_attestation_certificate: {:?}", e);
            return e;
        }
        Ok(res) => {
            res
        }
    };
    info!("private key {:?}, cert: {:?}", private_key_der, cert);

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
