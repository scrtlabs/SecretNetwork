//!
/// These functions run off chain, and so are not limited by deterministic limitations. Feel free
/// to go crazy with random generation entropy, time requirements, or whatever else
///
use log::*;
#[cfg(feature = "SGX_MODE_HW")]
use sgx_types::{sgx_platform_info_t, sgx_update_info_bit_t};
use sgx_types::{sgx_status_t, SgxResult};
use std::slice;

use enclave_crypto::consts::{
    SigningMethod, ATTESTATION_CERT_PATH, ENCRYPTED_SEED_SIZE, IO_CERTIFICATE_SAVE_PATH,
    SEED_EXCH_CERTIFICATE_SAVE_PATH, SIGNATURE_TYPE,
};
use enclave_crypto::{CryptoError, KeyPair, Keychain, Seed, KEY_MANAGER, PUBLIC_KEY_SIZE};
#[cfg(feature = "SGX_MODE_HW")]
use enclave_ffi_types::NodeAuthResult;
use enclave_utils::pointers::validate_mut_slice;
use enclave_utils::storage::write_to_untrusted;
use enclave_utils::{validate_const_ptr, validate_mut_ptr};

use sgx_types::c_int;
use std::net::SocketAddr;
use std::os::unix::io::IntoRawFd;
use std::{
    io::{BufReader, ErrorKind, Read, Write},
    net::TcpStream,
    str,
    string::String,
    sync::Arc,
};

use enclave_ffi_types::SINGLE_ENCRYPTED_SEED_SIZE;

#[cfg(feature = "SGX_MODE_HW")]
use crate::registration::report::AttestationReport;

use super::attestation::create_attestation_certificate;
use super::cert::verify_ra_cert;
#[cfg(feature = "SGX_MODE_HW")]
use super::cert::{ocall_get_update_info, verify_quote_status};
use super::seed_exchange::decrypt_seed;

#[cfg(not(feature = "use_seed_service"))]
const EXPECTED_SEED_SIZE: u32 = 96;

#[cfg(feature = "use_seed_service")]
const EXPECTED_SEED_SIZE: u32 = 48;

///
/// `ecall_init_bootstrap`
///
/// Function to handle the initialization of the bootstrap node. Generates the master private/public
/// key (seed + pk_io/sk_io). This happens once at the genesis of a chain. Returns the master
/// public key (pk_io), which is saved on-chain, and used to propagate the seed to registering nodes
///
/// # Safety
///  Something should go here
///
#[no_mangle]
pub unsafe extern "C" fn ecall_init_bootstrap(
    public_key: &mut [u8; PUBLIC_KEY_SIZE],
    spid: *const u8,
    spid_len: u32,
    api_key: *const u8,
    api_key_len: u32,
) -> sgx_status_t {
    validate_mut_ptr!(
        public_key.as_mut_ptr(),
        public_key.len(),
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );

    validate_const_ptr!(spid, spid_len as usize, sgx_status_t::SGX_ERROR_UNEXPECTED);

    validate_const_ptr!(
        api_key,
        api_key_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );
    let api_key_slice = slice::from_raw_parts(api_key, api_key_len as usize);

    let mut key_manager = Keychain::new();

    if let Err(_e) = key_manager.create_consensus_seed() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    #[cfg(feature = "use_seed_service")]
    {
        let temp_keypair = KeyPair::new()?;

        let new_consensus_seed = match get_next_consensus_seed_from_service(
            &mut key_manager,
            0,
            genesis_seed,
            api_key_slice,
            temp_keypair,
            crate::APP_VERSION
        ) {
            Ok(s) => s,
            Err(e) => {
                error!("Consensus seed failure: {}", e as u64);
                return sgx_status_t::SGX_ERROR_UNEXPECTED;
            }
        };

        key_manager.set_consensus_seed(key_manager.get_consensus_seed()?.genesis, new_consensus_seed)?;
    }

    if let Err(_e) = key_manager.generate_consensus_master_keys() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(_e) = key_manager.create_registration_key() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let kp = key_manager.seed_exchange_key().unwrap();
    if let Err(status) =
        attest_from_key(&kp.current, SEED_EXCH_CERTIFICATE_SAVE_PATH, api_key_slice)
    {
        return status;
    }

    let kp = key_manager.get_consensus_io_exchange_keypair().unwrap();
    if let Err(status) = attest_from_key(&kp.current, IO_CERTIFICATE_SAVE_PATH, api_key_slice) {
        return status;
    }

    public_key.copy_from_slice(
        &key_manager
            .seed_exchange_key()
            .unwrap()
            .current
            .get_pubkey(),
    );

    trace!(
        "ecall_init_bootstrap consensus_seed_exchange_keypair public key: {:?}",
        hex::encode(public_key)
    );

    sgx_status_t::SGX_SUCCESS
}

///
///  `ecall_init_node`
///
/// This function is called during initialization of __non__ bootstrap nodes.
///
/// It receives the master public key (pk_io) and uses it, and its node key (generated in [ecall_key_gen])
/// to decrypt the seed.
///
/// The seed was encrypted using Diffie-Hellman in the function [ecall_get_encrypted_seed]
///
/// This function happens off-chain, so if we panic for some reason it _can_ be acceptable,
///  though probably not recommended
///
/// 15/10/22 - this is now called during node startup and will evaluate whether or not a node is valid
///
/// # Safety
///  Something should go here
///
#[no_mangle]
pub unsafe extern "C" fn ecall_init_node(
    master_cert: *const u8,
    master_cert_len: u32,
    encrypted_seed: *const u8,
    encrypted_seed_len: u32,
    api_key: *const u8,
    api_key_len: u32,
) -> sgx_status_t {
    validate_const_ptr!(
        master_cert,
        master_cert_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );

    validate_const_ptr!(
        encrypted_seed,
        encrypted_seed_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );

    validate_const_ptr!(
        api_key,
        api_key_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );

    let api_key_slice = slice::from_raw_parts(api_key, api_key_len as usize);

    let cert_slice = slice::from_raw_parts(master_cert, master_cert_len as usize);

    if (encrypted_seed_len as usize) != ENCRYPTED_SEED_SIZE {
        warn!(
            "Got encrypted seed with the wrong size: {:?}",
            encrypted_seed_len
        );
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    // validate this node is patched and updated

    // generate temporary key for attestation
    let temp_key_result = KeyPair::new();

    if temp_key_result.is_err() {
        error!("Failed to generate temporary key for attestation");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    // this validates the cert and handles the "what if it fails" inside as well
    let res = create_attestation_certificate(
        temp_key_result.as_ref().unwrap(),
        SIGNATURE_TYPE,
        api_key_slice,
        None,
    );
    if res.is_err() {
        error!("Error starting node, might not be updated",);
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let encrypted_seed_slice = slice::from_raw_parts(encrypted_seed, encrypted_seed_len as usize);

    let mut encrypted_seed = [0u8; ENCRYPTED_SEED_SIZE];
    encrypted_seed.copy_from_slice(encrypted_seed_slice);

    if encrypted_seed[0] as u32 != EXPECTED_SEED_SIZE {
        error!("Got encrypted seed of different size than expected: {}", encrypted_seed[0]);
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    // public keys in certificates don't have 0x04, so we'll copy it here
    let mut target_public_key: [u8; PUBLIC_KEY_SIZE] = [0u8; PUBLIC_KEY_SIZE];

    // validate master certificate - basically test that we're on the correct network
    let pk = match verify_ra_cert(cert_slice, Some(SigningMethod::MRSIGNER)) {
        Err(e) => {
            debug!("Error validating master certificate - {:?}", e);
            error!("Error validating network parameters. Are you on the correct network? (error code 0x01)");
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
        Ok(res) => res,
    };

    // just make sure the of the public key isn't messed up
    if pk.len() != PUBLIC_KEY_SIZE {
        error!(
            "Got public key from certificate with the wrong size: {:?}",
            pk.len()
        );
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }
    target_public_key.copy_from_slice(&pk);

    trace!(
        "ecall_init_node target public key is: {:?}",
        target_public_key
    );

    let mut key_manager = Keychain::new();

    // even though key is overwritten later we still want to explicitly remove it in case we increase the security version
    // to make sure that it is resealed using the new svn
    if let Err(_e) = key_manager.reseal_registration_key() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    };

    let delete_res = key_manager.delete_consensus_seed();
    if delete_res {
        debug!("Successfully removed consensus seed");
    } else {
        debug!("Failed to remove consensus seed. Didn't exist?");
    }

    let mut single_seed_bytes = [0u8; SINGLE_ENCRYPTED_SEED_SIZE];
    single_seed_bytes.copy_from_slice(&encrypted_seed[1..(SINGLE_ENCRYPTED_SEED_SIZE + 1)]);

    trace!("Target public key is: {:?}", target_public_key);
    let genesis_seed = match decrypt_seed(&key_manager, target_public_key, single_seed_bytes) {
        Ok(result) => result,
        Err(status) => return status,
    };

    let new_consensus_seed;

    #[cfg(feature = "use_seed_service")]
    {
        trace!("HERE {}", line!());
        new_consensus_seed = match get_next_consensus_seed_from_service(
            &mut key_manager,
            0,
            genesis_seed,
            api_key_slice,
            KEY_MANAGER.get_registration_key().unwrap(),
            crate::APP_VERSION
        ) {
            Ok(s) => s,
            Err(e) => {
                error!("Consensus seed failure: {}", e as u64);
                return sgx_status_t::SGX_ERROR_UNEXPECTED;
            }
        };
    }

    #[cfg(not(feature = "use_seed_service"))]
    {
        single_seed_bytes.copy_from_slice(&encrypted_seed[(SINGLE_ENCRYPTED_SEED_SIZE + 1)..(SINGLE_ENCRYPTED_SEED_SIZE * 2 + 1)]);
        new_consensus_seed = match decrypt_seed(&key_manager, target_public_key, single_seed_bytes) {
            Ok(result) => result,
            Err(status) => return status,
        };
    }


    // TODO get current seed from seed server
    if let Err(_e) = key_manager.set_consensus_seed(genesis_seed, new_consensus_seed) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(_e) = key_manager.generate_consensus_master_keys() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

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
 * # Safety
 * Something should go here
 */
pub unsafe extern "C" fn ecall_get_attestation_report(
    api_key: *const u8,
    api_key_len: u32,
) -> sgx_status_t {
    // validate_const_ptr!(spid, spid_len as usize, sgx_status_t::SGX_ERROR_UNEXPECTED);
    // let spid_slice = slice::from_raw_parts(spid, spid_len as usize);

    validate_const_ptr!(
        api_key,
        api_key_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );
    let api_key_slice = slice::from_raw_parts(api_key, api_key_len as usize);

    let kp = KEY_MANAGER.get_registration_key().unwrap();
    trace!(
        "ecall_get_attestation_report key pk: {:?}",
        &kp.get_pubkey().to_vec()
    );
    let (_private_key_der, cert) =
        match create_attestation_certificate(&kp, SIGNATURE_TYPE, api_key_slice, None) {
            Err(e) => {
                warn!("Error in create_attestation_certificate: {:?}", e);
                return e;
            }
            Ok(res) => res,
        };

    //let path_prefix = ATTESTATION_CERT_PATH.to_owned();
    if let Err(status) = write_to_untrusted(cert.as_slice(), ATTESTATION_CERT_PATH.as_str()) {
        return status;
    }

    print_local_report_info(cert.as_slice());

    sgx_status_t::SGX_SUCCESS
}

///
/// This function generates the registration_key, which is used in the attestation and registration
/// process
///
#[no_mangle]
pub unsafe extern "C" fn ecall_get_new_consensus_seed(seed_id: u32) -> sgx_types::sgx_status_t {
    sgx_status_t::SGX_SUCCESS
}

///
/// This function generates the registration_key, which is used in the attestation and registration
/// process
///
#[no_mangle]
pub unsafe extern "C" fn ecall_key_gen(
    public_key: &mut [u8; PUBLIC_KEY_SIZE],
) -> sgx_types::sgx_status_t {
    if let Err(_e) = validate_mut_slice(public_key) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let mut key_manager = Keychain::new();
    if let Err(_e) = key_manager.create_registration_key() {
        error!("Failed to create registration key");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    };

    let reg_key = key_manager.get_registration_key();

    if reg_key.is_err() {
        error!("Failed to unlock node key. Please make sure the file is accessible or reinitialize the node");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let pubkey = reg_key.unwrap().get_pubkey();
    public_key.clone_from_slice(&pubkey);
    trace!("ecall_key_gen key pk: {:?}", public_key.to_vec());
    sgx_status_t::SGX_SUCCESS
}

pub fn attest_from_key(kp: &KeyPair, save_path: &str, api_key: &[u8]) -> SgxResult<()> {
    let (_, cert) = match create_attestation_certificate(kp, SIGNATURE_TYPE, api_key, None) {
        Err(e) => {
            error!("Error in create_attestation_certificate: {:?}", e);
            return Err(e);
        }
        Ok(res) => res,
    };

    if let Err(status) = write_to_untrusted(cert.as_slice(), save_path) {
        return Err(status);
    }

    Ok(())
}

fn create_socket_to_service(host_name: &str) -> Result<c_int, CryptoError> {
    use std::net::ToSocketAddrs;

    let mut addr: Option<SocketAddr> = None;

    const SERVICE_PORT: u16 = 3000;
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
    let (_, cert) = match create_attestation_certificate(&kp, SIGNATURE_TYPE, api_key, None) {
        Err(_) => {
            trace!("Failed to get certificate from intel for seed service");
            return Err(CryptoError::IntelCommunicationError);
        }
        Ok(res) => res,
    };

    let serialized_cert = base64::encode(cert);

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

    let serialized_cert = base64::encode(cert);

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
fn get_next_consensus_seed_from_service(
    key_manager: &mut Keychain,
    retries: u8,
    genesis_seed: Seed,
    api_key: &[u8],
    kp: KeyPair,
    seed_id: u16
) -> Result<Seed, CryptoError> {
    let mut opt_seed: Result<Seed, CryptoError> = Err(CryptoError::DecryptionError);

    match retries {
        0 => {
            trace!("Looping consensus seed lookup forever");
            loop {
                if let Ok(seed) = try_get_consensus_seed_from_service(
                    seed_id,
                    api_key,
                    kp,
                ) {
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

#[cfg(not(feature = "SGX_MODE_HW"))]
fn print_local_report_info(_cert: &[u8]) {}

#[cfg(feature = "SGX_MODE_HW")]
fn print_local_report_info(cert: &[u8]) {
    let report = match AttestationReport::from_cert(cert) {
        Ok(r) => r,
        Err(_) => {
            error!("Error parsing report");
            return;
        }
    };

    let node_auth_result = NodeAuthResult::from(&report.sgx_quote_status);
    // print
    match verify_quote_status(&report, &report.advisory_ids) {
        Err(status) => match status {
            NodeAuthResult::SwHardeningAndConfigurationNeeded => {
                println!("Platform status is SW_HARDENING_AND_CONFIGURATION_NEEDED. This means is updated but requires further BIOS configuration");
            }
            NodeAuthResult::GroupOutOfDate => {
                println!("Platform status is GROUP_OUT_OF_DATE. This means that one of the system components is missing a security update");
            }
            _ => {
                println!("Platform status is {:?}", status);
            }
        },
        _ => println!("Platform Okay!"),
    }

    // print platform blob info
    match node_auth_result {
        NodeAuthResult::GroupOutOfDate | NodeAuthResult::SwHardeningAndConfigurationNeeded => unsafe {
            print_platform_info(&report)
        },
        _ => {}
    }
}

#[cfg(feature = "SGX_MODE_HW")]
unsafe fn print_platform_info(report: &AttestationReport) {
    if let Some(platform_info) = &report.platform_info_blob {
        let mut update_info = sgx_update_info_bit_t::default();
        let mut rt = sgx_status_t::default();
        let res = ocall_get_update_info(
            &mut rt as *mut sgx_status_t,
            platform_info[4..].as_ptr() as *const sgx_platform_info_t,
            1,
            &mut update_info,
        );

        if res != sgx_status_t::SGX_SUCCESS {
            println!("res={:?}", res);
            return;
        }

        if rt != sgx_status_t::SGX_SUCCESS {
            if update_info.ucodeUpdate != 0 {
                println!("Processor Firmware Update (ucodeUpdate). A security upgrade for your computing\n\
                            device is required for this application to continue to provide you with a high degree of\n\
                            security. Please contact your device manufacturer’s support website for a BIOS update\n\
                            for this system");
            }

            if update_info.csmeFwUpdate != 0 {
                println!("Intel Manageability Engine Update (csmeFwUpdate). A security upgrade for your\n\
                            computing device is required for this application to continue to provide you with a high\n\
                            degree of security. Please contact your device manufacturer’s support website for a\n\
                            BIOS and/or Intel® Manageability Engine update for this system");
            }

            if update_info.pswUpdate != 0 {
                println!("Intel SGX Platform Software Update (pswUpdate). A security upgrade for your\n\
                              computing device is required for this application to continue to provide you with a high\n\
                              degree of security. Please visit this application’s support website for an Intel SGX\n\
                              Platform SW update");
            }
        }
    }
}
