use log::*;

use sgx_types::{sgx_status_t, SgxResult};

use crate::consts::ENCRYPTED_SEED_SIZE;
use crate::crypto::{AESKey, Keychain, SIVEncryptable, PUBLIC_KEY_SIZE, SEED_KEY_SIZE};

pub fn encrypt_seed(
    key_manager: &Keychain,
    new_node_pk: [u8; PUBLIC_KEY_SIZE],
) -> SgxResult<Vec<u8>> {
    let shared_enc_key = key_manager
        .seed_exchange_key()
        .unwrap()
        .diffie_hellman(&new_node_pk);

    let mut authenticated_data: Vec<&[u8]> = Vec::default();
    authenticated_data.push(&new_node_pk);
    // encrypt the seed using the symmetric key derived in the previous stage
    let res = match AESKey::new_from_slice(&shared_enc_key).encrypt_siv(
        &key_manager.get_consensus_seed().unwrap().get().to_vec(),
        &authenticated_data,
    ) {
        Ok(r) => {
            if r.len() != ENCRYPTED_SEED_SIZE {
                error!(
                    "Seed encryption failed. Got seed of unexpected length: {:?}",
                    r.len()
                );
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }
            r
        }
        Err(e) => return Err(sgx_status_t::SGX_ERROR_UNEXPECTED),
    };

    Ok(res)
}

///
/// master_pk: [seed_exch_publickey] - Public key that is written on-chain at genesis
///
pub fn decrypt_seed(
    key_manager: &Keychain,
    master_pk: [u8; PUBLIC_KEY_SIZE],
    encrypted_seed: [u8; ENCRYPTED_SEED_SIZE],
) -> SgxResult<Vec<u8>> {
    // create shared encryption key using ECDH
    let shared_enc_key = key_manager
        .get_registration_key()
        .unwrap()
        .diffie_hellman(&master_pk);

    // Create AD of encryption
    let my_public_key = key_manager.get_registration_key().unwrap().get_pubkey();
    let mut authenticated_data: Vec<&[u8]> = Vec::default();
    authenticated_data.push(&my_public_key);

    // decrypt
    let res = match AESKey::new_from_slice(&shared_enc_key)
        .decrypt_siv(&encrypted_seed, &authenticated_data)
    {
        Ok(r) => {
            if r.len() != SEED_KEY_SIZE {
                error!(
                    "Init failed! Decrypted seed has invalid length - {:?}",
                    r.len()
                );
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }
            r
        }
        Err(e) => return Err(sgx_status_t::SGX_ERROR_UNEXPECTED),
    };

    Ok(res)
}
