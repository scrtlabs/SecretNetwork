use crate::error::NetError;
use log::*;
use std::net::{SocketAddr, TcpStream};

pub fn create_socket_to_service(host_name: &str, port: u16) -> Result<TcpStream, NetError> {
    use std::net::ToSocketAddrs;

    let mut addr: Option<SocketAddr> = None;

    let addrs = (host_name, port).to_socket_addrs().map_err(|err| {
        trace!("Error while trying to convert to socket addrs {:?}", err);
        NetError::SocketCreateFailed
    })?;

    for a in addrs {
        if let SocketAddr::V4(_) = a {
            addr = Some(a);
        }
    }

    if addr.is_none() {
        trace!("Failed to resolve the IPv4 address of the service");
        return Err(NetError::IPv4LookupError);
    }

    let sock = TcpStream::connect(&addr.unwrap()).map_err(|err| {
        trace!(
            "Error while trying to connect to service with addr: {:?}, err: {:?}",
            addr,
            err
        );
        NetError::SocketCreateFailed
    })?;

    Ok(sock)
}
