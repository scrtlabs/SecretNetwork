use serde_json::to_string;
use std::net::{SocketAddr, TcpStream};
use std::os::unix::io::IntoRawFd;

use sgx_types::{c_int, sgx_status_t, SgxResult};

use log::{debug, error};

#[no_mangle]
pub extern "C" fn ocall_get_socket(
    ret_fd: *mut c_int,
    hostname: *mut u8,
    hostname_len: u32,
    port: u16,
) -> sgx_status_t {
    let host = unsafe { std::slice::from_raw_parts(hostname, hostname_len as usize) };

    let as_str = String::from_utf8_lossy(host);
    //
    // if as_str.is_err() {
    //     error!("Failed to convert hostname to valid string");
    //     return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    // }

    //let hostname = as_str.to_string();

    debug!("Opening socket to address: {:?}", &as_str);
    let addr = lookup_ipv4(&as_str, port);

    if let Err(e) = addr {
        return e;
    }

    let sock = TcpStream::connect(&addr.unwrap()).expect("[-] Connect tls server failed!");

    unsafe {
        *ret_fd = sock.into_raw_fd();
    }

    sgx_status_t::SGX_SUCCESS
}

pub fn lookup_ipv4(host: &str, port: u16) -> SgxResult<SocketAddr> {
    use std::net::ToSocketAddrs;

    let addrs = (host, port).to_socket_addrs().map_err(|e| {
        error!("Failed to create socket for {:?} {:?}: {:?}", host, port, e);
        sgx_status_t::SGX_ERROR_NETWORK_FAILURE
    })?;
    for addr in addrs {
        if let SocketAddr::V4(_) = addr {
            return Ok(addr);
        }
    }

    unreachable!("Cannot lookup address");
}
