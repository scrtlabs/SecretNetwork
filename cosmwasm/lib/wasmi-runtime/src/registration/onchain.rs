///
/// These functions run on-chain and must be deterministic across all nodes
///
use log::*;
use sgx_types::sgx_status_t;

use crate::consts::ENCRYPTED_SEED_SIZE;
use crate::crypto::{Keychain, PUBLIC_KEY_SIZE};
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
#[no_mangle]
pub extern "C" fn ecall_authenticate_new_node(
    cert: *const u8,
    cert_len: u32,
    seed: &mut [u8; ENCRYPTED_SEED_SIZE],
) -> sgx_status_t {
    if let Err(e) = validate_mut_ptr(seed.as_mut_ptr(), seed.len()) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(e) = validate_const_ptr(cert, cert_len as usize) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let cert_slice = unsafe { std::slice::from_raw_parts(cert, cert_len as usize) };
    let key_manager = Keychain::new();

    // verify certificate, and return the public key in the extra data of the report
    let pk = match verify_ra_cert(cert_slice) {
        Err(e) => {
            error!("Error in validating certificate: {:?}", e);
            return e;
        }
        Ok(res) => res,
    };

    // just make sure the length isn't wrong for some reason (certificate may be malformed)
    if pk.len() != PUBLIC_KEY_SIZE {
        error!(
            "Got public key from certificate with the wrong size: {:?}",
            pk.len()
        );
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let mut target_public_key: [u8; 32] = [0u8; 32];
    target_public_key.copy_from_slice(&pk);
    debug!(
        "ecall_get_encrypted_seed target_public_key key pk: {:?}",
        &target_public_key.to_vec()
    );

    let res: Vec<u8> = match encrypt_seed(&key_manager, target_public_key) {
        Ok(result) => result,
        Err(status) => return status,
    };

    seed.copy_from_slice(&res);

    sgx_status_t::SGX_SUCCESS
}
