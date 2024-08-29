use sgx_types::*;
use std::io;
use std::io::{Read, Write};
use std::vec::Vec;
use std::string::String;

#[cfg(feature = "attestation_server")]
use crate::key_manager::{keys::RegistrationKey, UNSEALED_KEY_MANAGER};
#[cfg(not(feature = "attestation_server"))]
use crate::key_manager::keys::RegistrationKey;

pub mod helpers;
pub mod auth;

/// Initializes new TLS client with report of Remote Attestation
pub fn perform_master_key_request(
    hostname: String,
    socket_fd: c_int,
    qe_target_info: Option<&sgx_target_info_t>,
    quote_size: Option<u32>,
    is_dcap: bool,
) -> SgxResult<()> {
    let (key_der, cert_der) = helpers::create_tls_cert_and_keys(qe_target_info, quote_size)?;
    let client_config = helpers::construct_client_config(key_der, cert_der, is_dcap);

    // Prepare TLS connection
    let (mut sess, mut conn) =
        helpers::create_client_session_stream(hostname, socket_fd, client_config)?;
    let mut tls = rustls::Stream::new(&mut sess, &mut conn);

    // Generate temporary registration key used for master key encryption during transfer
    let reg_key = RegistrationKey::random()?;

    // Send public key, derived from Registration key, to Attestation server
    tls.write(reg_key.public_key().as_bytes()).map_err(|err| {
        println!(
            "[Enclave] Cannot send public key to Attestation server. Reason: {:?}",
            err
        );
        sgx_status_t::SGX_ERROR_UNEXPECTED
    })?;

    // Wait for Attestation server response
    let mut response = Vec::new();
    match tls.read_to_end(&mut response) {
        Err(ref err) if err.kind() == io::ErrorKind::ConnectionAborted => {
            println!("[Enclave] Attestation Client: connection aborted");
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
        Err(e) => {
            println!(
                "[Enclave] Attestation Client: error in read_to_end: {:?}",
                e
            );
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
        _ => {}
    };

    // Decrypt and seal master key
    helpers::decrypt_and_seal_master_key(&reg_key, &response)?;

    Ok(())
}

#[cfg(feature = "attestation_server")]
/// Initializes new TLS server to share master key
pub fn perform_epoch_keys_provisioning(
    socket_fd: c_int,
    qe_target_info: Option<&sgx_target_info_t>,
    quote_size: Option<u32>,
    is_dcap: bool,
) -> SgxResult<()> {
    let (key_der, cert_der) = helpers::create_tls_cert_and_keys(qe_target_info, quote_size)?;
    let server_config = helpers::construct_server_config(key_der, cert_der, is_dcap);

    // Prepare TLS connection
    let (mut sess, mut conn) = helpers::create_server_session_stream(socket_fd, server_config)?;
    let mut tls = rustls::Stream::new(&mut sess, &mut conn);

    // Read client registration public key
    let mut client_public_key = [0u8; 32];
    tls.read(&mut client_public_key).map_err(|err| {
        println!(
            "[Enclave] Attestation Server: error in read_to_end: {:?}",
            err
        );
        sgx_status_t::SGX_ERROR_UNEXPECTED
    })?;

    // Generate registration key for ECDH
    let registration_key = RegistrationKey::random()?;

    // Unseal key manager to get access to master key
    let encrypted_epoch_data = match &*UNSEALED_KEY_MANAGER {
        Some(key_manager) => {
            key_manager
                .encrypt_epoch_data(&registration_key, client_public_key.to_vec())
                .map_err(|err| {
                    println!(
                        "[Enclave] Cannot encrypt epoch data to share it. Reason: {:?}",
                        err
                    );
                    sgx_status_t::SGX_ERROR_UNEXPECTED
                })?
        },
        None => {
            println!("[Enclave] Cannot unseal key manager data");
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    };

    // Send encrypted epoch data to client
    tls.write(&encrypted_epoch_data).map_err(|err| {
        println!(
            "[Enclave] Cannot send encrypted epoch data to client. Reason: {:?}",
            err
        );
        sgx_status_t::SGX_ERROR_UNEXPECTED
    })?;

    Ok(())
}

#[cfg(not(feature = "attestation_server"))]
/// Initializes new TLS server to share master key
pub fn perform_epoch_keys_provisioning(
    _socket_fd: c_int,
    _qe_target_info: Option<&sgx_target_info_t>,
    _quote_size: Option<u32>,
    _is_dcap: bool,
) -> SgxResult<()> {
    println!("[Enclave] Attestation Server is unaccessible");
    Err(sgx_status_t::SGX_ERROR_UNEXPECTED)
}