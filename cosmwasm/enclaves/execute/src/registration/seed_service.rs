use log::*;

use enclave_networking::http::{HttpResponse, Method};
use enclave_networking::tls::TlsSession;

use enclave_crypto::{consts::SIGNATURE_TYPE, CryptoError, KeyPair, Keychain, Seed};

use crate::registration::create_attestation_certificate;
use enclave_networking::endpoints::Endpoint;
use std::{
    io::{ErrorKind, Read, Write},
    str,
};

const SEED_SERVICE_PORT: u16 = 4478;

fn get_challenge_from_service(
    host_name: &str,
    api_key: &[u8],
    kp: KeyPair,
) -> Result<Vec<u8>, CryptoError> {
    pub const CHALLENGE_ENDPOINT: &str = "/authenticate";
    let (_, cert) = match create_attestation_certificate(&kp, SIGNATURE_TYPE, api_key, None) {
        Err(_) => {
            trace!("Failed to get certificate from intel for seed service");
            return Err(CryptoError::IntelCommunicationError);
        }
        Ok(res) => res,
    };

    let serialized_cert = base64::encode(serde_json::to_string(&cert).unwrap());

    let req = enclave_networking::http::HttpRequest::new(
        Method::POST,
        host_name,
        CHALLENGE_ENDPOINT,
        None,
        None,
        Some(serialized_cert),
    )
    .to_string();

    trace!("{}", req);

    let mut tls: TlsSession = enclave_networking::tls::TlsSession::new(
        Some(Endpoint::SeedService),
        host_name,
        Some(SEED_SERVICE_PORT),
    )
    .map_err(|_| CryptoError::SSSCommunicationError)?;
    let mut stream = tls.new_stream();

    let _result = stream.write(req.as_bytes());
    let mut plaintext = Vec::new();

    info!("write complete");

    match stream.read_to_end(&mut plaintext) {
        Ok(_) => {}
        Err(e) => {
            if e.kind() != ErrorKind::ConnectionAborted {
                trace!("Error while reading https response {:?}", e);
                return Err(CryptoError::SSSCommunicationError);
            }
        }
    }

    info!("read_to_end complete");

    let challenge: Vec<u8> = HttpResponse::body_from_response_b64(&plaintext).map_err(|e| {
        trace!("Error while reading https response: {:?}", e);
        CryptoError::BadResponse
    })?;

    Ok(challenge)
}

fn get_seed_from_service(
    host_name: &str,
    api_key: &[u8],
    kp: KeyPair,
    id: u16,
    challenge: Vec<u8>,
) -> Result<Vec<u8>, CryptoError> {
    pub const SEED_ENDPOINT: &str = "/seed/";
    let (_, cert) = match create_attestation_certificate(
        &kp,
        SIGNATURE_TYPE,
        api_key,
        Some(challenge.as_slice()),
    ) {
        Err(_) => {
            trace!("Failed to get certificate from intel for seed service");
            return Err(CryptoError::IntelCommunicationError);
        }
        Ok(res) => res,
    };

    let serialized_cert = base64::encode(serde_json::to_string(&cert).unwrap());
    let req = enclave_networking::http::HttpRequest::new(
        Method::POST,
        host_name,
        &format!("{}{}", SEED_ENDPOINT, id),
        None,
        None,
        Some(serialized_cert),
    )
    .to_string();

    let mut tls: TlsSession = enclave_networking::tls::TlsSession::new(
        Some(Endpoint::SeedService),
        host_name,
        Some(SEED_SERVICE_PORT),
    )
    .map_err(|_| CryptoError::SSSCommunicationError)?;
    let mut stream = tls.new_stream();

    let _result = stream.write(req.as_bytes());
    let mut plaintext = Vec::new();

    debug!("write complete");

    match stream.read_to_end(&mut plaintext) {
        Ok(_) => {}
        Err(e) => {
            if e.kind() != ErrorKind::ConnectionAborted {
                trace!("Error while reading https response {:?}", e);
                return Err(CryptoError::SSSCommunicationError);
            }
        }
    }

    debug!("read_to_end complete");
    let seed = HttpResponse::body_from_response_b64(&plaintext).map_err(|e| {
        trace!("Error while reading https response {:?}", e);
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

    let challenge = get_challenge_from_service(SEED_SERVICE_DNS, api_key, kp)?;
    let s = get_seed_from_service(SEED_SERVICE_DNS, api_key, kp, id, challenge)?;
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
