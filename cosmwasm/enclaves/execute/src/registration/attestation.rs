use core::slice;
use enclave_crypto::consts::{ATTESTATION_CERT_PATH, SIGNATURE_TYPE};
use enclave_crypto::{KeyPair, Keychain, KEY_MANAGER, PUBLIC_KEY_SIZE};
use enclave_utils::pointers::validate_mut_slice;
use enclave_utils::storage::write_to_untrusted;
use enclave_utils::validate_const_ptr;
use log::{error, trace, warn};
use sgx_types::sgx_status_t;

use secret_attestation_token::AttestationType;

#[no_mangle]
/**
 * `ecall_generate_authentication_material`
 *
 * Creates the attestation certificate to be used to authenticate with the blockchain. The output of this
 * function is an X.509 certificate signed by the enclave, which contains the enclave quote signed by Intel.
 *
 * This quote can either be a DCAP or EPID quote type, and in the future other non-sgx attestation types as well
 *
 * Verifying functions will verify the public key bytes sent in the extra data of the __report__ (which
 * may or may not match the public key of the __certificate__ -- depending on implementation choices)
 *
 * This x509 certificate can be used in the future for mutual-RA cross-enclave TLS channels, or for
 * other creative usages.
 * # Safety
 * Something should go here
 */
pub unsafe extern "C" fn ecall_generate_authentication_material(
    api_key: *const u8,
    api_key_len: u32,
    auth_type: u8,
) -> sgx_status_t {
    validate_const_ptr!(
        api_key,
        api_key_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );
    let api_key_slice = slice::from_raw_parts(api_key, api_key_len as usize);

    let keypair: KeyPair = KEY_MANAGER.get_registration_key().unwrap();
    trace!(
        "ecall_get_attestation_report key pk: {:?}",
        &keypair.get_pubkey().to_vec()
    );

    let registration_public_key = &keypair.get_pubkey();

    let auth_material = match auth_type.into() {
        AttestationType::SgxEpid | AttestationType::SgxSw => {
            match epid::generate_authentication_material(
                registration_public_key,
                SIGNATURE_TYPE,
                api_key_slice,
                auth_type.into(),
                None,
            ) {
                Err(e) => {
                    warn!("Error in create_attestation_certificate: {:?}", e);
                    return e;
                }
                Ok(res) => res,
            }
        }
        #[cfg(feature = "dcap")]
        AttestationType::SgxDcap => {
            match dcap::generate_authentication_material(
                registration_public_key,
                SIGNATURE_TYPE,
                api_key_slice,
            ) {
                Err(e) => {
                    warn!("Error in create_attestation_certificate: {:?}", e);
                    return e;
                }
                Ok(res) => res,
            }
        }
        _ => {
            panic!("Unsupported authentication type")
        }
    };

    let as_json = serde_json::to_string(&auth_material);

    if as_json.is_err() {
        error!("Failed to serialize auth material to json");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(status) =
    write_to_untrusted(as_json.unwrap().as_bytes(), &ATTESTATION_CERT_PATH as &str)
    {
        return status;
    }

    sgx_status_t::SGX_SUCCESS
}

///
/// This function generates the registration_key, which is used in the attestation and registration
/// process
///
#[no_mangle]
pub unsafe extern "C" fn ecall_generate_registration_key(
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
    trace!(
        "ecall_generate_registration_key key pk: {:?}",
        public_key.to_vec()
    );
    sgx_status_t::SGX_SUCCESS
}
