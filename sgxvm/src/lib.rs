#![no_std]
#![feature(slice_as_chunks)]
#![feature(core_intrinsics)]

#[macro_use]
extern crate sgx_tstd as std;
extern crate alloc;
extern crate rustls;
extern crate sgx_tse;
extern crate sgx_types;

use sgx_tcrypto::*;
use sgx_types::*;

use std::slice;
use std::string::String;
use std::vec::Vec;

use crate::attestation::dcap::{
    get_qe_quote, 
    utils::{encode_quote_with_collateral, decode_quote_with_collateral},
};
use crate::querier::GoQuerier;
use crate::types::{Allocation, AllocationWithResult};
use crate::protobuf_generated::ffi::{
    ListEpochsResponse, EpochData
};
use protobuf::Message;

mod attestation;
mod backend;
mod coder;
mod encryption;
mod error;
mod handlers;
mod key_manager;
mod memory;
mod ocall;
mod precompiles;
mod protobuf_generated;
mod querier;
mod storage;
mod types;

#[no_mangle]
/// Checks if there is already sealed master key
pub unsafe extern "C" fn ecall_is_initialized() -> i32 {
    println!(
        "[KeyManager] KeyManager file location: {}/{}",
        key_manager::KEYMANAGER_HOME.to_str().unwrap(),
        key_manager::KEYMANAGER_FILENAME
    );

    if let Err(err) = key_manager::KeyManager::unseal() {
        println!(
            "[Enclave] Cannot restore master key. Reason: {:?}",
            err.as_str()
        );
        return false as i32;
    }
    true as i32
}

#[no_mangle]
/// Allocates provided data inside Intel SGX Enclave and returns
/// pointer to allocated data and data length.
pub extern "C" fn ecall_allocate(data: *const u8, len: usize) -> crate::types::Allocation {
    let slice = unsafe { slice::from_raw_parts(data, len) };
    let mut vector_copy = slice.to_vec();

    let ptr = vector_copy.as_mut_ptr();
    let size = vector_copy.len();
    std::mem::forget(vector_copy);

    Allocation {
        result_ptr: ptr,
        result_size: size,
    }
}

#[no_mangle]
/// Performes self attestation and outputs if system was configured
/// properly and node can pass Remote Attestation.
pub extern "C" fn ecall_status() -> sgx_status_t {
    attestation::self_attestation::self_attest()
}

#[no_mangle]
/// Handles incoming protobuf-encoded request
pub extern "C" fn handle_request(
    querier: *mut GoQuerier,
    request_data: *const u8,
    len: usize,
) -> AllocationWithResult {
    handlers::handle_protobuf_request_inner(querier, request_data, len)
}

#[no_mangle]
/// Handles incoming request for DCAP Remote Attestation
pub unsafe extern "C" fn ecall_request_epoch_keys_dcap(
    hostname: *const u8,
    data_len: usize,
    socket_fd: c_int,
    qe_target_info: &sgx_target_info_t,
    quote_size: u32,
) -> sgx_status_t {
    let hostname = slice::from_raw_parts(hostname, data_len);
    let hostname = match String::from_utf8(hostname.to_vec()) {
        Ok(hostname) => hostname,
        Err(err) => {
            println!("[Enclave] Cannot decode hostname. Reason: {:?}", err);
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    };

    match attestation::tls::perform_master_key_request(
        hostname,
        socket_fd,
        Some(qe_target_info),
        Some(quote_size),
        true,
    ) {
        Ok(_) => sgx_status_t::SGX_SUCCESS,
        Err(err) => err,
    }
}

#[cfg(feature = "attestation_server")]
#[no_mangle]
/// Handles incoming request for sharing master key with new node using DCAP attestation
pub unsafe extern "C" fn ecall_attest_peer_dcap(
    socket_fd: c_int,
    qe_target_info: &sgx_target_info_t,
    quote_size: u32,
) -> sgx_status_t {
    match attestation::tls::perform_epoch_keys_provisioning(
        socket_fd,
        Some(qe_target_info),
        Some(quote_size),
        true,
    ) {
        Ok(_) => sgx_status_t::SGX_SUCCESS,
        Err(err) => err,
    }
}

#[cfg(not(feature = "attestation_server"))]
#[no_mangle]
/// Handles incoming request for sharing master key with new node using DCAP attestation
pub unsafe extern "C" fn ecall_attest_peer_dcap(
    _socket_fd: c_int,
    _qe_target_info: &sgx_target_info_t,
    _quote_size: u32,
) -> sgx_status_t {
    println!("[Enclave] Cannot attest peer. Attestation Server is unaccessible");
    sgx_status_t::SGX_ERROR_UNEXPECTED
}

#[no_mangle]
/// Handles incoming request for sharing master key with new node using EPID attestation
pub unsafe extern "C" fn ecall_attest_peer_epid(socket_fd: c_int) -> sgx_status_t {
    match attestation::tls::perform_epoch_keys_provisioning(socket_fd, None, None, false) {
        Ok(_) => sgx_status_t::SGX_SUCCESS,
        Err(err) => err,
    }
}

#[no_mangle]
/// Handles initialization of a new seed node by creating and sealing master key to seed file
/// If `reset_flag` was set to `true`, it will rewrite existing seed file
pub unsafe extern "C" fn ecall_initialize_enclave(reset_flag: i32) -> sgx_status_t {
    key_manager::init_enclave_inner(reset_flag)
}

#[no_mangle]
/// Handles incoming request for EPID Remote Attestation
pub unsafe extern "C" fn ecall_request_epoch_keys_epid(
    hostname: *const u8,
    data_len: usize,
    socket_fd: c_int,
) -> sgx_status_t {
    let hostname = slice::from_raw_parts(hostname, data_len);
    let hostname = match String::from_utf8(hostname.to_vec()) {
        Ok(hostname) => hostname,
        Err(err) => {
            println!(
                "[Enclave] Seed Client. Cannot decode hostname. Reason: {:?}",
                err
            );
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
    };

    match attestation::tls::perform_master_key_request(hostname, socket_fd, None, None, false) {
        Ok(_) => sgx_status_t::SGX_SUCCESS,
        Err(err) => err,
    }
}

#[no_mangle]
pub unsafe extern "C" fn ecall_dump_dcap_quote(
    qe_target_info: &sgx_target_info_t,
    quote_size: u32,
) -> AllocationWithResult {
    let ecc_handle = SgxEccHandle::new();
    let _ = ecc_handle.open();
    let (_, pub_k) = match ecc_handle.create_key_pair() {
        Ok(res) => res,
        Err(status_code) => {
            println!(
                "[Enclave] Cannot create key pair using SgxEccHandle. Reason: {:?}",
                status_code
            );
            return AllocationWithResult::default();
        }
    };

    let encoded_quote = match get_qe_quote(&pub_k, qe_target_info, quote_size) {
        Ok((quote, coll)) => encode_quote_with_collateral(quote, coll),
        Err(status_code) => {
            println!(
                "[Enclave] Cannot generate QE quote. Reason: {:?}",
                status_code
            );
            return AllocationWithResult::default();
        }
    };

    let _ = ecc_handle.close();

    handlers::allocate_inner(encoded_quote)
}

#[no_mangle]
pub unsafe extern "C" fn ecall_verify_dcap_quote(
    quote_ptr: *const u8,
    quote_len: u32,
) -> sgx_status_t {
    let slice = unsafe { slice::from_raw_parts(quote_ptr, quote_len as usize) };
    let (quote, collateral) = decode_quote_with_collateral(slice.as_ptr(), slice.len() as u32);

    match attestation::dcap::verify_dcap_quote(quote, collateral) {
        Ok(_) => {
            println!("[Enclave] Quote verified");
            sgx_status_t::SGX_SUCCESS
        }
        Err(err) => {
            println!(
                "[Enlcave] Quote verification failed. Status code: {:?}",
                err
            );
            err
        }
    }
}

#[no_mangle]
pub unsafe extern "C" fn ecall_add_epoch(starting_block: u64) -> sgx_status_t {
    #[cfg(feature = "attestation_server")]
    {
        // Unseal old key manager
        let key_manager = match key_manager::KeyManager::unseal() {
            Ok(km) => km,
            Err(err) => {
                return err;
            }
        };

        match key_manager.add_new_epoch(starting_block) {
            Ok(_) => sgx_status_t::SGX_SUCCESS,
            Err(err) => err
        }
    }

    #[cfg(not(feature = "attestation_server"))]
    {
        println!("[Enclave] Not enabled");
        sgx_status_t::SGX_ERROR_UNEXPECTED
    }
}

#[no_mangle]
pub unsafe extern "C" fn ecall_remove_latest_epoch() -> sgx_status_t {
    #[cfg(feature = "attestation_server")]
    {
        // Unseal old key manager
        let key_manager = match key_manager::KeyManager::unseal() {
            Ok(km) => km,
            Err(err) => {
                return err;
            }
        };

        match key_manager.remove_latest_epoch() {
            Ok(_) => sgx_status_t::SGX_SUCCESS,
            Err(err) => err
        }
    }

    #[cfg(not(feature = "attestation_server"))]
    {
        println!("[Enclave] Not enabled");
        sgx_status_t::SGX_ERROR_UNEXPECTED
    }
}

#[no_mangle]
pub unsafe extern "C" fn ecall_list_epochs() -> AllocationWithResult {
    let key_manager = match key_manager::KeyManager::unseal() {
        Ok(km) => km,
        Err(err) => {
            println!("Cannot unseal key manager. Reason: {:?}", err);
            return AllocationWithResult::default();
        }
    };

    let stored_epochs = key_manager.list_epochs();
    
    let mut epochs_response = ListEpochsResponse::new();
    let mut epochs: Vec<EpochData> = Vec::new();
    for (epoch_number, starting_block, node_public_key) in stored_epochs {
        let mut epoch = EpochData::new();
        epoch.set_epochNumber(epoch_number.into());
        epoch.set_startingBlock(starting_block);
        epoch.set_nodePublicKey(node_public_key.clone());
        
        epochs.push(epoch)
    }
    epochs_response.set_epochs(epochs.into());
    let encoded_response = match epochs_response.write_to_bytes() {
        Ok(res) => res,
        Err(err) => {
            println!("Cannot encode protobuf result. Reason: {:?}", err);
            return AllocationWithResult::default();
        }
    };

    handlers::allocate_inner(encoded_response)
}

// Fix https://github.com/apache/incubator-teaclave-sgx-sdk/issues/373 for debug mode
#[cfg(debug_assertions)]
#[no_mangle]
pub extern "C" fn __assert_fail(
    __assertion: *const u8,
    __file: *const u8,
    __line: u32,
    __function: *const u8,
) -> ! {
    use core::intrinsics::abort;
    abort()
}
