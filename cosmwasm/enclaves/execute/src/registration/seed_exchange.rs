use log::*;

use sgx_types::{sgx_status_t, SgxResult};

use enclave_crypto::consts::ENCRYPTED_SEED_SIZE;
use enclave_crypto::{
    AESKey, Keychain, SIVEncryptable, Seed, KEY_MANAGER, PUBLIC_KEY_SIZE, SEED_KEY_SIZE,
};
use enclave_ffi_types::SINGLE_ENCRYPTED_SEED_SIZE;

pub enum SeedType {
    Genesis,
    Current
}

pub fn encrypt_seed(new_node_pk: [u8; PUBLIC_KEY_SIZE], seed_type: SeedType) -> SgxResult<Vec<u8>> {
    let shared_enc_key = KEY_MANAGER
        .seed_exchange_key()
        .unwrap()
        .current
        .diffie_hellman(&new_node_pk);

    let authenticated_data: Vec<&[u8]> = vec![&new_node_pk];

    let seed_to_share = match seed_type {
        SeedType::Genesis => {
            KEY_MANAGER.get_consensus_seed().unwrap().genesis.clone()
        }
        SeedType::Current => {
            KEY_MANAGER.get_consensus_seed().unwrap().current.clone()
        }
    };

    // encrypt the seed using the symmetric key derived in the previous stage
    // genesis seed is passed in registration
    // TODO get current seed from the seed server

    trace!(
        "Public keys on encryption {:?} {:?}",
        KEY_MANAGER
            .seed_exchange_key()
            .unwrap()
            .current
            .get_pubkey(),
        new_node_pk
    );
    let res = match AESKey::new_from_slice(&shared_enc_key).encrypt_siv(
        seed_to_share.as_slice(),
        Some(&authenticated_data),
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
        Err(_e) => return Err(sgx_status_t::SGX_ERROR_UNEXPECTED),
    };

    Ok(res)
}

///
/// master_pk: [seed_exch_publickey] - Public key that is written on-chain at genesis
///
pub fn decrypt_seed(
    key_manager: &Keychain,
    master_pk: [u8; PUBLIC_KEY_SIZE],
    encrypted_seed: [u8; SINGLE_ENCRYPTED_SEED_SIZE],
) -> SgxResult<Seed> {
    // create shared encryption key using ECDH
    let shared_enc_key = key_manager
        .get_registration_key()
        .map_err(|_e| {
            error!("Failed to unlock node key. Please make sure the file is accessible or reinitialize the node");
            sgx_status_t::SGX_ERROR_UNEXPECTED
        })?
        .diffie_hellman(&master_pk);

    let mut genesis_seed = Seed::default();

    // Create AD of encryption
    let my_public_key = key_manager.get_registration_key().unwrap().get_pubkey();
    let authenticated_data: Vec<&[u8]> = vec![&my_public_key];

    trace!(
        "Public keys on decryption: {:?} {:?}",
        key_manager.get_registration_key().unwrap().get_pubkey(),
        master_pk
    );

    // decrypt
    genesis_seed
        .as_mut()
        .copy_from_slice(&match AESKey::new_from_slice(&shared_enc_key)
            .decrypt_siv(&encrypted_seed, Some(&authenticated_data))
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
            Err(_e) => return Err(sgx_status_t::SGX_ERROR_UNEXPECTED),
        });
    Ok(genesis_seed)
}
