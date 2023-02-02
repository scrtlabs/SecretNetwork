use log::*;
use secret_attestation_token::AttestationType;

use enclave_crypto::{consts::SIGNATURE_TYPE, CryptoError, KeyPair, Keychain, Seed};

use sgx_types::c_int;

use epid::generate_authentication_material;
use std::{
    io::{BufReader, ErrorKind, Read, Write},
    net::{SocketAddr, TcpStream},
    os::unix::io::IntoRawFd,
    str,
    string::String,
    sync::Arc,
};

fn create_socket_to_service(host_name: &str) -> Result<c_int, CryptoError> {
    use std::net::ToSocketAddrs;

    let mut addr: Option<SocketAddr> = None;

    const SERVICE_PORT: u16 = 4487;
    let addrs = (host_name, SERVICE_PORT).to_socket_addrs().map_err(|err| {
        trace!("Error while trying to convert to socket addrs {:?}", err);
        CryptoError::SocketCreationError
    })?;

    for a in addrs {
        if let SocketAddr::V4(_) = a {
            addr = Some(a);
        }
    }

    if addr.is_none() {
        trace!("Failed to resolve the IPv4 address of the service");
        return Err(CryptoError::IPv4LookupError);
    }

    let sock = TcpStream::connect(&addr.unwrap()).map_err(|err| {
        trace!(
            "Error while trying to connect to service with addr: {:?}, err: {:?}",
            addr,
            err
        );
        CryptoError::SocketCreationError
    })?;

    Ok(sock.into_raw_fd())
}

fn make_client_config() -> rustls::ClientConfig {
    let mut config = rustls::ClientConfig::new();

    pub const SSS_CA: &[u8] = include_bytes!("sss_ca.pem");
    let mut pem_reader = BufReader::new(SSS_CA);

    let mut root_store = rustls::RootCertStore::empty();
    root_store
        .add_pem_file(&mut pem_reader)
        .expect("Failed to add PEM");

    config.root_store = root_store;

    config
}

fn get_body_from_response(resp: &[u8]) -> Result<String, CryptoError> {
    trace!("get_body_from_response");
    let mut headers = [httparse::EMPTY_HEADER; 16];
    let mut respp = httparse::Response::new(&mut headers);
    let result = respp.parse(resp);
    trace!("parse result {:?}", result);

    match respp.code {
        Some(200) => info!("Response okay"),
        Some(401) => {
            error!("Unauthorized Failed to authenticate or authorize request.");
            return Err(CryptoError::BadResponse);
        }
        Some(404) => {
            error!("Not Found");
            return Err(CryptoError::BadResponse);
        }
        Some(500) => {
            error!("Internal error occurred in SSS server");
            return Err(CryptoError::BadResponse);
        }
        Some(503) => {
            error!(
                "Service is currently not able to process the request (due to
            a temporary overloading or maintenance). This is a
            temporary state – the same request can be repeated after
            some time. "
            );
            return Err(CryptoError::BadResponse);
        }
        _ => {
            error!(
                "response from SSS server :{} - unknown error or response code",
                respp.code.unwrap()
            );
            return Err(CryptoError::BadResponse);
        }
    }

    let mut len_num: u32 = 0;
    for i in 0..respp.headers.len() {
        let h = respp.headers[i];
        //println!("{} : {}", h.name, str::from_utf8(h.value).unwrap());
        if h.name.to_lowercase().as_str() == "content-length" {
            let len_str = String::from_utf8(h.value.to_vec()).unwrap();
            len_num = len_str.parse::<u32>().unwrap();
            trace!("content length = {}", len_num);
        }
    }

    let mut body = "".to_string();
    if len_num != 0 {
        let header_len = result.unwrap().unwrap();
        let resp_body = &resp[header_len..];
        body = str::from_utf8(resp_body).unwrap().to_string();
    }

    Ok(body)
}

fn get_challenge_from_service(
    fd: c_int,
    host_name: &str,
    api_key: &[u8],
    kp: KeyPair,
) -> Result<Vec<u8>, CryptoError> {
    pub const CHALLENGE_ENDPOINT: &str = "/authenticate";
    let cert = match generate_authentication_material(
        &kp.get_pubkey(),
        SIGNATURE_TYPE,
        api_key,
        AttestationType::SgxEpid,
        None,
    ) {
        Err(_) => {
            trace!("Failed to get certificate from intel for seed service");
            return Err(CryptoError::IntelCommunicationError);
        }
        Ok(res) => res,
    };

    let serialized_cert = base64::encode(serde_json::to_string(&cert).unwrap());

    let req = format!("GET {} HTTP/1.1\r\nHOST: {}\r\nContent-Length:{}\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n{}",
                      CHALLENGE_ENDPOINT,
                      host_name,
                      serialized_cert.len(),
                      serialized_cert);

    trace!("{}", req);
    let config = make_client_config();
    let dns_name = webpki::DNSNameRef::try_from_ascii_str(host_name).unwrap();
    let mut sess = rustls::ClientSession::new(&Arc::new(config), dns_name);
    let mut sock = TcpStream::new(fd).map_err(|err| {
        trace!("Error while trying to create TcpStream {:?}", err);
        CryptoError::SocketCreationError
    })?;
    let mut tls = rustls::Stream::new(&mut sess, &mut sock);

    let _result = tls.write(req.as_bytes());
    let mut plaintext = Vec::new();

    info!("write complete");

    match tls.read_to_end(&mut plaintext) {
        Ok(_) => {}
        Err(e) => {
            if e.kind() != ErrorKind::ConnectionAborted {
                trace!("Error while reading https response {:?}", e);
                return Err(CryptoError::SSSCommunicationError);
            }
        }
    }

    info!("read_to_end complete");

    let challenge = base64::decode(get_body_from_response(&plaintext)?).map_err(|err| {
        trace!("https response wasn't base64  {:?}", err);
        CryptoError::SSSCommunicationError
    })?;

    Ok(challenge)
}

fn get_seed_from_service(
    fd: c_int,
    host_name: &str,
    api_key: &[u8],
    kp: KeyPair,
    id: u16,
    challenge: Vec<u8>,
) -> Result<Vec<u8>, CryptoError> {
    pub const SEED_ENDPOINT: &str = "/seed/";
    let cert = match generate_authentication_material(
        &kp.get_pubkey(),
        SIGNATURE_TYPE,
        api_key,
        AttestationType::SgxEpid,
        None,
    ) {
        Err(_) => {
            trace!("Failed to get certificate from intel for seed service");
            return Err(CryptoError::IntelCommunicationError);
        }
        Ok(res) => res,
    };

    let serialized_cert = base64::encode(serde_json::to_string(&cert).unwrap());

    let req = format!("GET {}{} HTTP/1.1\r\nHOST: {}\r\nContent-Length:{}\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n{}",
                      SEED_ENDPOINT, id,
                      host_name,
                      serialized_cert.len(),
                      serialized_cert);

    trace!("{}", req);
    let config = make_client_config();
    let dns_name = webpki::DNSNameRef::try_from_ascii_str(host_name).unwrap();
    let mut sess = rustls::ClientSession::new(&Arc::new(config), dns_name);
    let mut sock = TcpStream::new(fd).map_err(|err| {
        trace!("Error while trying to create TcpStream {:?}", err);
        CryptoError::SocketCreationError
    })?;
    let mut tls = rustls::Stream::new(&mut sess, &mut sock);

    let _result = tls.write(req.as_bytes());
    let mut plaintext = Vec::new();

    info!("write complete");

    match tls.read_to_end(&mut plaintext) {
        Ok(_) => {}
        Err(e) => {
            if e.kind() != ErrorKind::ConnectionAborted {
                trace!("Error while reading https response {:?}", e);
                return Err(CryptoError::SSSCommunicationError);
            }
        }
    }

    info!("read_to_end complete");

    let seed = base64::decode(get_body_from_response(&plaintext)?).map_err(|err| {
        trace!("https response wasn't base64  {:?}", err);
        CryptoError::SSSCommunicationError
    })?;

    Ok(seed)
}

fn try_get_consensus_seed_from_service(
    id: u16,
    api_key: &[u8],
    kp: KeyPair,
) -> Result<Seed, CryptoError> {
    #[cfg(feature = "production")]
    pub const SEED_SERVICE_DNS: &str = "sss.scrtlabs.com";
    #[cfg(not(feature = "production"))]
    pub const SEED_SERVICE_DNS: &str = "sssd.scrtlabs.com";

    let mut socket = create_socket_to_service(SEED_SERVICE_DNS)?;
    let challenge = get_challenge_from_service(socket, SEED_SERVICE_DNS, api_key, kp)?;
    socket = create_socket_to_service(SEED_SERVICE_DNS)?;
    let s = get_seed_from_service(socket, SEED_SERVICE_DNS, api_key, kp, id, challenge)?;
    let mut seed = Seed::default();
    seed.as_mut().copy_from_slice(&s);
    Ok(seed)
}

// Retreiving consensus seed from SingularitySeedService
// id - The desired seed id
// retries - The amount of times to retry upon failure. 0 means infinite
pub fn get_next_consensus_seed_from_service(
    key_manager: &mut Keychain,
    retries: u8,
    genesis_seed: Seed,
    api_key: &[u8],
    kp: KeyPair,
    seed_id: u16,
) -> Result<Seed, CryptoError> {
    let mut opt_seed: Result<Seed, CryptoError> = Err(CryptoError::DecryptionError);

    match retries {
        0 => {
            trace!("Looping consensus seed lookup forever");
            loop {
                if let Ok(seed) = try_get_consensus_seed_from_service(seed_id, api_key, kp) {
                    opt_seed = Ok(seed);
                    break;
                }
            }
        }
        _ => {
            for try_id in 1..retries + 1 {
                trace!("Looping consensus seed lookup {}/{}", try_id, retries);
                match try_get_consensus_seed_from_service(
                    key_manager.get_consensus_seed_id(),
                    api_key,
                    kp,
                ) {
                    Ok(seed) => {
                        opt_seed = Ok(seed);
                        break;
                    }
                    Err(e) => opt_seed = Err(e),
                }
            }
        }
    };

    if let Err(e) = opt_seed {
        return Err(e);
    }

    let mut seed = opt_seed?;

    // XOR the seed with the genesis seed
    let mut seed_vec = seed.as_mut().to_vec();
    seed_vec
        .iter_mut()
        .zip(genesis_seed.as_slice().to_vec().iter())
        .for_each(|(x1, x2)| *x1 ^= *x2);

    seed.as_mut().copy_from_slice(seed_vec.as_slice());

    trace!("Successfully fetched consensus seed from service");
    key_manager.inc_consensus_seed_id();
    Ok(seed)
}
