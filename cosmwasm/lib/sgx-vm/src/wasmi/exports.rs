use enclave_ffi_types::{Ctx, EnclaveBuffer, UserSpaceBuffer};
use log::info;
use std::ffi::c_void;

use sgx_types::*;

use std::os::unix::io::{IntoRawFd, AsRawFd};
use std::net::{TcpListener, TcpStream, SocketAddr};

use crate::context::with_storage_from_context;

/// Copy a buffer from the enclave memory space, and return an opaque pointer to it.
#[no_mangle]
pub extern "C" fn ocall_allocate(buffer: *const u8, length: usize) -> UserSpaceBuffer {
    info!(
        target: module_path!(),
        "ocall_allocate() called with buffer length: {:?}", length
    );

    let slice = unsafe { std::slice::from_raw_parts(buffer, length) };
    let vector_copy = slice.to_vec();
    let boxed_vector = Box::new(vector_copy);
    let heap_pointer = Box::into_raw(boxed_vector);
    UserSpaceBuffer {
        ptr: heap_pointer as *mut c_void,
    }
}

/// Take a pointer as returned by `ocall_allocate` and recover the Vec<u8> inside of it.
pub unsafe fn recover_buffer(ptr: UserSpaceBuffer) -> Option<Vec<u8>> {
    if ptr.ptr.is_null() {
        return None;
    }
    let boxed_vector = Box::from_raw(ptr.ptr as *mut Vec<u8>);
    Some(*boxed_vector)
}

/// Read a key from the contracts key-value store.
/// instance_id should be the sha256 of the wasm blob.
#[no_mangle]
pub extern "C" fn ocall_read_db(mut context: Ctx, key: *const u8, key_len: usize) -> EnclaveBuffer {
    let key = unsafe { std::slice::from_raw_parts(key, key_len) };

    info!(
        target: module_path!(),
        "ocall_read_db() called with len: {:?} key: {:?}",
        key_len,
        String::from_utf8_lossy(key)
    );
    let null_buffer = EnclaveBuffer {
        ptr: std::ptr::null_mut(),
    };

    // Returning `EnclaveBuffer { ptr: std::ptr::null_mut() }` is basically returning a null pointer,
    // which in the enclave is interpreted as signaling that the key does not exist.
    // We also interpret this potential panic here as a missing key because we have no way of handling
    // it at the moment.
    // In the future, if we see that panics do occur here, we should add a way to report this to the enclave.
    std::panic::catch_unwind(move || {
        let mut value: Option<Vec<u8>> = None;
        with_storage_from_context(&mut context, |storage| value = storage.get(key));
        value
    })
    .map(|value| {
        value
            .map(|vec| {
                super::allocate_enclave_buffer(&vec).unwrap_or(unsafe { null_buffer.clone() })
            })
            .unwrap_or(unsafe { null_buffer.clone() })
    })
    // TODO add logging if we fail to write
    .unwrap_or(unsafe { null_buffer.clone() })
}

/// Write a value to the contracts key-value store.
/// instance_id should be the sha256 of the wasm blob.
#[no_mangle]
pub extern "C" fn ocall_write_db(
    mut context: Ctx,
    key: *const u8,
    key_len: usize,
    value: *const u8,
    value_len: usize,
) {
    let key = unsafe { std::slice::from_raw_parts(key, key_len) };
    let value = unsafe { std::slice::from_raw_parts(value, value_len) };

    info!(
        target: module_path!(),
        "ocall_write_db() called with key_len: {:?} key: {:?} val_len: {:?} val: {:?}... (first 20 bytes)",
        key_len,
        String::from_utf8_lossy(key),
        value_len,
        String::from_utf8_lossy(value.get(0..std::cmp::min(20, value_len)).unwrap())
    );

    // We explicitly ignore this potential panic here because we have no way of handling it at the moment.
    // In the future, if we see that panics do occur here, we should add a way to report this to the enclave.
    let _ = std::panic::catch_unwind(move || {
        with_storage_from_context(&mut context, |storage| storage.set(key, value))
    }); // TODO add logging if we fail to write
}

#[no_mangle]
pub extern "C"
fn ocall_sgx_init_quote(ret_ti: *mut sgx_target_info_t,
                        ret_gid : *mut sgx_epid_group_id_t) -> sgx_status_t {
    info!("Entering ocall_sgx_init_quote");
    unsafe {sgx_init_quote(ret_ti, ret_gid)}
}


pub fn lookup_ipv4(host: &str, port: u16) -> SocketAddr {
    use std::net::ToSocketAddrs;

    let addrs = (host, port).to_socket_addrs().unwrap();
    for addr in addrs {
        if let SocketAddr::V4(_) = addr {
            return addr;
        }
    }

    unreachable!("Cannot lookup address");
}


#[no_mangle]
pub extern "C"
fn ocall_get_ias_socket(ret_fd : *mut c_int) -> sgx_status_t {
    let port = 443;
    let hostname = "api.trustedservices.intel.com";
    let addr = lookup_ipv4(hostname, port);
    let sock = TcpStream::connect(&addr).expect("[-] Connect tls server failed!");

    unsafe {*ret_fd = sock.into_raw_fd();}

    sgx_status_t::SGX_SUCCESS
}

#[no_mangle]
pub extern "C"
fn ocall_get_quote (p_sigrl            : *const u8,
                    sigrl_len          : u32,
                    p_report           : *const sgx_report_t,
                    quote_type         : sgx_quote_sign_type_t,
                    p_spid             : *const sgx_spid_t,
                    p_nonce            : *const sgx_quote_nonce_t,
                    p_qe_report        : *mut sgx_report_t,
                    p_quote            : *mut u8,
                    _maxlen             : u32,
                    p_quote_len        : *mut u32) -> sgx_status_t {
    println!("Entering ocall_get_quote");

    let mut real_quote_len : u32 = 0;

    let ret = unsafe {
        sgx_calc_quote_size(p_sigrl, sigrl_len, &mut real_quote_len as *mut u32)
    };

    if ret != sgx_status_t::SGX_SUCCESS {
        println!("sgx_calc_quote_size returned {}", ret);
        return ret;
    }

    println!("quote size = {}", real_quote_len);
    unsafe { *p_quote_len = real_quote_len; }

    let ret = unsafe {
        sgx_get_quote(p_report,
                      quote_type,
                      p_spid,
                      p_nonce,
                      p_sigrl,
                      sigrl_len,
                      p_qe_report,
                      p_quote as *mut sgx_quote_t,
                      real_quote_len)
    };

    if ret != sgx_status_t::SGX_SUCCESS {
        println!("sgx_calc_quote_size returned {}", ret);
        return ret;
    }

    println!("sgx_calc_quote_size returned {}", ret);
    ret
}

#[no_mangle]
pub extern "C"
fn ocall_get_update_info (platform_blob: * const sgx_platform_info_t,
                          enclave_trusted: i32,
                          update_info: * mut sgx_update_info_bit_t) -> sgx_status_t {
    unsafe{
        sgx_report_attestation_status(platform_blob, enclave_trusted, update_info)
    }
}
