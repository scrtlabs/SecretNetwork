use super::PRIVATE_KEY_SIZE;
use deoxysii::NONCE_SIZE;
use hmac::{Hmac, Mac, NewMac as _};
use sgx_types::*;

pub fn derive_key(master_key: &[u8; PRIVATE_KEY_SIZE], info: &[u8]) -> [u8; PRIVATE_KEY_SIZE] {
    let mut kdf = Hmac::<sha2::Sha256>::new_from_slice(info).expect("Unable to create KDF"); // TODO: Handle error
    kdf.update(master_key);
    let mut derived_key = [0u8; PRIVATE_KEY_SIZE];
    let digest = kdf.finalize();
    derived_key.copy_from_slice(&digest.into_bytes()[..PRIVATE_KEY_SIZE]);

    derived_key
}

/// Generates random 32 bytes slice using `sgx_read_rand` function
pub fn random_bytes32() -> SgxResult<[u8; 32]> {
    let mut buffer = [0u8; 32];
    let res = unsafe { sgx_read_rand(&mut buffer as *mut u8, 32) };

    if res != sgx_status_t::SGX_SUCCESS {
        println!(
            "[Enclave] Cannot generate random 32 bytes. Reason: {:?}",
            res
        );
        return Err(res);
    }

    Ok(buffer)
}

/// Generates random 32 bytes slice using `sgx_read_rand` function
pub fn random_nonce() -> SgxResult<[u8; NONCE_SIZE]> {
    let mut buffer = [0u8; NONCE_SIZE];
    let res = unsafe { sgx_read_rand(&mut buffer as *mut u8, NONCE_SIZE as usize) };

    if res != sgx_status_t::SGX_SUCCESS {
        println!(
            "[Enclave] Cannot generate random bytes for nonce. Reason: {:?}",
            res
        );
        return Err(res);
    }

    Ok(buffer)
}
