use crate::consts::DEFAULT_PORT;
use crate::endpoints::Endpoint;
use crate::error::NetError;
use rustls::{ClientSession, Stream};
use std::io::BufReader;
use std::net::TcpStream;
use std::sync::Arc;

#[allow(dead_code)]
pub fn new_custom_client_config(cert: &[u8]) -> rustls::ClientConfig {
    let mut config = rustls::ClientConfig::new();

    let mut pem_reader = BufReader::new(cert);

    let mut root_store = rustls::RootCertStore::empty();
    root_store
        .add_pem_file(&mut pem_reader)
        .expect("Failed to add PEM");

    config.root_store = root_store;

    config
}

fn make_client_seed_service_config() -> rustls::ClientConfig {
    let mut config = rustls::ClientConfig::new();

    pub const SSS_CA: &[u8] = include_bytes!("certs/sss_ca.pem");
    let mut pem_reader = BufReader::new(SSS_CA);

    let mut root_store = rustls::RootCertStore::empty();
    root_store
        .add_pem_file(&mut pem_reader)
        .expect("Failed to add PEM");

    config.root_store = root_store;

    config
}

#[allow(dead_code)]
pub fn make_client_ias_config() -> rustls::ClientConfig {
    let mut config = rustls::ClientConfig::new();

    pub const SSS_CA: &[u8] = include_bytes!("certs/sss_ca.pem");
    let mut pem_reader = BufReader::new(SSS_CA);

    let mut root_store = rustls::RootCertStore::empty();
    root_store
        .add_pem_file(&mut pem_reader)
        .expect("Failed to add PEM");

    config.root_store = root_store;

    config
}

pub struct TlsSession {
    socket: Box<TcpStream>,
    session: Box<ClientSession>,
    //pub stream: Stream<'a, ClientSession, TcpStream>,
}

impl TlsSession {
    pub fn new(
        endpoint: Option<Endpoint>,
        host_name: &str,
        port: Option<u16>,
    ) -> Result<Self, NetError> {
        let config = if let Some(ep) = endpoint {
            match ep {
                Endpoint::SeedService => make_client_seed_service_config(),
                Endpoint::IntelAttestationService => make_client_ias_config(),
            }
        } else {
            make_client_ias_config()
        };

        let dns_name = webpki::DNSNameRef::try_from_ascii_str(host_name)
            .map_err(|_| NetError::InvalidDnsName)?;
        let session = rustls::ClientSession::new(&Arc::new(config), dns_name);
        let socket =
            crate::socket::create_socket_to_service(host_name, port.unwrap_or(DEFAULT_PORT))?;

        Ok(TlsSession {
            socket: Box::new(socket),
            session: Box::new(session),
        })
    }

    pub fn new_stream(&mut self) -> Stream<ClientSession, TcpStream> {
        rustls::Stream::new(&mut self.session, &mut self.socket)
    }
}

// pub fn create_tls_session<'a>(
//     config: ClientConfig,
//     host_name: &str,
//     port: Option<u16>,
// ) -> Result<
//     (
//         Stream<'a, ClientSession, TcpStream>,
//         ClientSession,
//         TcpStream,
//     ),
//     NetError,
// > {
//     let dns_name = webpki::DNSNameRef::try_from_ascii_str(host_name).unwrap();
//     let mut sess = rustls::ClientSession::new(&Arc::new(config), dns_name);
//     let mut sock =
//         crate::socket::create_socket_to_service(host_name, port.unwrap_or(DEFAULT_PORT))?;
//     let mut tls = rustls::Stream::new(&mut sess, &mut sock);
//
//     Ok((tls, sess, sock))
// }
