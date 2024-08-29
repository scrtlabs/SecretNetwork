use rustls::{ClientConfig, ClientSession, ServerConfig, ServerSession};
use sgx_tcrypto::*;
use sgx_types::*;
use std::sync::Arc;
use std::vec::Vec;
use std::{net::TcpStream, string::String};

use crate::attestation::consts::{ENCRYPTED_KEY_SIZE, PUBLIC_KEY_SIZE};
use crate::attestation::{
    cert::gen_ecc_cert,
    consts::QUOTE_SIGNATURE_TYPE,
    dcap::get_qe_quote,
    dcap::utils::encode_quote_with_collateral,
    utils::create_attestation_report,
};
use crate::attestation::tls::auth::{ClientAuth, ServerAuth};
use crate::key_manager::{KeyManager, keys::RegistrationKey};

/// Prepares config for client side of TLS connection
pub(super) fn construct_client_config(key_der: Vec<u8>, cert_der: Vec<u8>, is_dcap: bool) -> ClientConfig {
    let mut cfg = rustls::ClientConfig::new();
    let certs = vec![rustls::Certificate(cert_der)];
    let privkey = rustls::PrivateKey(key_der);

    cfg.set_single_client_cert(certs, privkey).unwrap();
    cfg.dangerous()
        .set_certificate_verifier(Arc::new(ServerAuth::new(true, is_dcap)));
    cfg.versions.clear();
    cfg.versions.push(rustls::ProtocolVersion::TLSv1_2);
    cfg
}

/// Prepares config for server side of TLS connection
pub(super) fn construct_server_config(key_der: Vec<u8>, cert_der: Vec<u8>, is_dcap: bool) -> ServerConfig {
    let mut cfg = rustls::ServerConfig::new(Arc::new(ClientAuth::new(true, is_dcap)));
    let certs = vec![rustls::Certificate(cert_der)];
    let privkey = rustls::PrivateKey(key_der);

    cfg.set_single_cert_with_ocsp_and_sct(certs, privkey, vec![], vec![])
        .unwrap();

    cfg
}

/// Creates TLS session stream for client
pub(super) fn create_client_session_stream(
    hostname: String,
    socket_fd: c_int,
    cfg: ClientConfig,
) -> SgxResult<(ClientSession, TcpStream)> {
    let dns_name = webpki::DNSNameRef::try_from_ascii_str(&hostname).map_err(|err| {
        println!(
            "[Enclave] Cannot construct correct DNS name. Reason: {:?}",
            err
        );
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    })?;

    let sess = rustls::ClientSession::new(&Arc::new(cfg), dns_name);
    let conn = TcpStream::new(socket_fd).map_err(|err| {
        println!("[Enclave] Cannot start TcpStream. Reason: {:?}", err);
        sgx_status_t::SGX_ERROR_UNEXPECTED
    })?;

    Ok((sess, conn))
}

/// Creates TLS session stream for server
pub(super) fn create_server_session_stream(
    socket_fd: c_int,
    cfg: ServerConfig,
) -> SgxResult<(ServerSession, TcpStream)> {
    let sess = ServerSession::new(&Arc::new(cfg));
    let conn = TcpStream::new(socket_fd).map_err(|err| {
        println!("[Enclave] Cannot start TcpStream. Reason: {:?}", err);
        sgx_status_t::SGX_ERROR_UNEXPECTED
    })?;
    Ok((sess, conn))
}

/// Decrypts and seals received master key
pub(super) fn decrypt_and_seal_master_key(
    reg_key: &RegistrationKey,
    attn_server_response: &Vec<u8>,
) -> SgxResult<()> {
    // Validate response size. It should be equal or more 90 bytes
    // 32 public key | 16 nonce | ciphertext
    if attn_server_response.len() < ENCRYPTED_KEY_SIZE {
        println!("[Enclave] Wrong response size from Attestation Server");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    // Extract public key and nonce + ciphertext
    println!("[Enclave] Attestation Client: decrypting epochs data");
    let public_key = &attn_server_response[..PUBLIC_KEY_SIZE];
    let encrypted_epochs_data = &attn_server_response[PUBLIC_KEY_SIZE..];

    // Construct key manager from encrypted epoch data
    let km = KeyManager::decrypt_epoch_data(
        reg_key,
        public_key.to_vec(),
        encrypted_epochs_data.to_vec(),
    )
    .map_err(|err| {
        println!(
            "[Enclave] Cannot construct from encrypted epoch data. Reason: {:?}",
            err
        );
        sgx_status_t::SGX_ERROR_UNEXPECTED
    })?;

    // Seal decrypted master key
    println!("[Enclave] Attestation Client: sealing epoch data key");
    km.seal()?;
    println!("[Enclave] Epoch data successfully sealed");

    Ok(())
}

/// Creates keys and certificate for TLS connection
pub(super) fn create_tls_cert_and_keys(
    qe_target_info: Option<&sgx_target_info_t>,
    quote_size: Option<u32>,
) -> SgxResult<(Vec<u8>, Vec<u8>)> {
    let ecc_handle = SgxEccHandle::new();
    let _ = ecc_handle.open();
    let (prv_k, pub_k) = ecc_handle.create_key_pair()?;

    let payload = match (qe_target_info, quote_size) {
        (Some(qe_target_info), Some(quote_size)) => {
            let (qe_quote, qe_collateral) = get_qe_quote(&pub_k, qe_target_info, quote_size)?;
            let encoded_payload = encode_quote_with_collateral(qe_quote, qe_collateral);
            base64::encode(&encoded_payload[..])
        }
        _ => {
            let signed_report = create_attestation_report(&pub_k, QUOTE_SIGNATURE_TYPE)?;
            serde_json::to_string(&signed_report).map_err(|err| {
                println!(
                    "Error serializing report. May be malformed, or badly encoded: {:?}",
                    err
                );
                sgx_status_t::SGX_ERROR_UNEXPECTED
            })?
        }
    };

    let (key_der, cert_der) = gen_ecc_cert(payload, &prv_k, &pub_k, &ecc_handle)?;
    let _ = ecc_handle.close();

    Ok((key_der, cert_der))
}
