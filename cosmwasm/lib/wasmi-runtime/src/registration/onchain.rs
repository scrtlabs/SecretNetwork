///
/// These functions run on-chain and must be deterministic across all nodes
///
use log::*;
use sgx_types::{sgx_status_t, SgxResult};
use std::panic;

use crate::consts::ENCRYPTED_SEED_SIZE;
use crate::crypto::PUBLIC_KEY_SIZE;
use crate::storage::write_to_untrusted;
use crate::utils::{validate_const_ptr, validate_mut_ptr};

use super::cert::verify_ra_cert;
use super::seed_exchange::encrypt_seed;

///
/// `ecall_authenticate_new_node`
///
/// This call is used to help new nodes register in the network. The function will authenticate the
/// new node, based on a received certificate. If the node is authenticated successfully, the seed
/// will be encrypted and shared with the registering node.
///
/// The seed is encrypted with a key derived from the secret master key of the chain, and the public
/// key of the requesting chain
///
/// This function happens on-chain, so any panic here might cause the chain to go boom
///
/// # Safety
/// Safety first
#[no_mangle]
pub unsafe extern "C" fn ecall_authenticate_new_node(
    cert: *const u8,
    cert_len: u32,
    seed: &mut [u8; ENCRYPTED_SEED_SIZE],
) -> sgx_status_t {
    if let Err(_e) = validate_mut_ptr(seed.as_mut_ptr(), seed.len()) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(_e) = validate_const_ptr(cert, cert_len as usize) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }
    let cert_slice = std::slice::from_raw_parts(cert, cert_len as usize);

    let result = panic::catch_unwind(|| -> SgxResult<Vec<u8>> {
        // verify certificate, and return the public key in the extra data of the report
        let pk = match verify_ra_cert(cert_slice) {
            Err(e) => {
                error!("Error in validating certificate: {:?}", e);
                if let Err(status) = write_to_untrusted(cert_slice, "failed_cert.der") {
                    return Err(status);
                }
                return Err(e);
            }
            Ok(res) => res,
        };

        // just make sure the length isn't wrong for some reason (certificate may be malformed)
        if pk.len() != PUBLIC_KEY_SIZE {
            error!(
                "Got public key from certificate with the wrong size: {:?}",
                pk.len()
            );
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }

        let mut target_public_key: [u8; 32] = [0u8; 32];
        target_public_key.copy_from_slice(&pk);
        debug!(
            "ecall_get_encrypted_seed target_public_key key pk: {:?}",
            &target_public_key.to_vec()
        );

        let res: Vec<u8> = match encrypt_seed(target_public_key) {
            Ok(result) => result,
            Err(status) => return Err(status),
        };

        Ok(res)
    });
    if let Ok(res) = result {
        match res {
            Ok(res) => {
                seed.copy_from_slice(&res);
                sgx_status_t::SGX_SUCCESS
            }
            Err(e) => e,
        }
    } else {
        error!("Enclave call ecall_authenticate_new_node panic!");
        sgx_status_t::SGX_ERROR_UNEXPECTED
    }
}
