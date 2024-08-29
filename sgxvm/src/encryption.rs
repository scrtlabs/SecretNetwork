use deoxysii::{DeoxysII, NONCE_SIZE, TAG_SIZE};
use sgx_types::{sgx_read_rand, sgx_status_t};

use crate::{error::Error, key_manager::PUBLIC_KEY_SIZE};
use k256::sha2::{Digest, Sha256 as kSha256};
use rand_chacha::rand_core::{RngCore, SeedableRng};
use std::vec::Vec;

use crate::key_manager::{PRIVATE_KEY_SIZE, UNSEALED_KEY_MANAGER};

pub const FUNCTION_SELECTOR_LEN: usize = 4;
pub const ZERO_FUNCTION_SELECTOR: [u8; 4] = [0u8; 4];
pub const PUBLIC_KEY_ONLY_DATA_LEN: usize = 36;
pub const ENCRYPTED_DATA_LEN: usize = 79;
// 2 byte epoch prefix + ENCRYPTED_DATA_LEN
pub const PREFIXED_ENCRYPTED_DATA_LEN: usize = 81;
pub const DEFAULT_STORAGE_VALUE: [u8; 32] = [0u8; 32];

/// Encrypts given storage cell value using specific storage key for provided contract address
/// * contract_address - Address of the contract. Used to derive unique storage encryption key for state of this smart contract
/// * value - Raw storage value to encrypt
pub fn encrypt_storage_cell(
    contract_address: Vec<u8>,
    block_number: u64,
    encryption_salt: Vec<u8>,
    value: Vec<u8>,
) -> Result<Vec<u8>, Error> {
    match &*UNSEALED_KEY_MANAGER {
        Some(key_manager) => {
            match key_manager.get_state_key_by_block(block_number) {
                Some((epoch_number, state_key)) => {
                    let encrypted_value = state_key.encrypt(contract_address, encryption_salt, value)?;
                    let mut output = epoch_number.to_be_bytes().to_vec();
                    output.extend(encrypted_value);
                    Ok(output)
                },
                None => Err(Error::encryption_err("There no keys stored in Epoch Manager")),
            }
        }
        None => Err(Error::encryption_err("Cannot unseal master key")),
    }
}

/// Decrypts given storage cell value using specific storage key for provided contract address
/// * contract_address - Address of the contract. Used to derive unique storage encryption key for state of this smart contract
/// * value - Encrypted storage value
pub fn decrypt_storage_cell(
    contract_address: Vec<u8>,
    encrypted_value: Vec<u8>,
) -> Result<Vec<u8>, Error> {
    // It there is 32-byte zeroed vector, it means that storage slot was not initialized
    // In this case we return default value
    if encrypted_value == DEFAULT_STORAGE_VALUE.to_vec() {
        return Ok(encrypted_value);
    }

    let (epoch_number, encrypted_data) = match encrypted_value.len() {
        ENCRYPTED_DATA_LEN => (0u16, encrypted_value),
        PREFIXED_ENCRYPTED_DATA_LEN => {
            let prefix = &encrypted_value[..2];
            let epoch_number: u16 = ((prefix[0] as u16) << 8) | prefix[1] as u16;
            (epoch_number, encrypted_value[2..].to_vec())
        }
        _ => {
            return Err(Error::encryption_err(format!(
                "Invalid encrypted value length. Expected 79 or 81, Got: {:?}",
                encrypted_value.len()
            )));
        }
    };

    // Find appropriate key in EpochManager
    let state_key = match &*UNSEALED_KEY_MANAGER {
        Some(key_manager) => key_manager.get_state_key_by_epoch(epoch_number),
        None => {
            return Err(Error::encryption_err("Cannot get access to key manager"));
        }
    };

    match state_key {
        Some(key) => key.decrypt(contract_address, encrypted_data),
        None => Err(Error::encryption_err("Key for epoch not found")),
    }
}

/// Extracts user public and encrypted data from provided tx `data` field.
/// If data starts with 0x00000000 prefix and has 36 bytes length, it means that there is only public key and no ciphertext.
/// If data has length of 78 and more bytes, we handle it as encrypted data
/// * tx_data - `data` field of transaction
pub fn extract_public_key_and_data(tx_data: Vec<u8>) -> Result<(Vec<u8>, Vec<u8>, Vec<u8>), Error> {
    // Check if provided tx data starts with `ZERO_FUNCTION_SELECTOR`
    // and has length of 36 bytes (4 prefix | 32 public key)
    if tx_data.len() == PUBLIC_KEY_ONLY_DATA_LEN && tx_data[..4] == ZERO_FUNCTION_SELECTOR {
        let public_key = &tx_data[FUNCTION_SELECTOR_LEN..PUBLIC_KEY_ONLY_DATA_LEN];
        // Return extracted public key and empty ciphertext
        return Ok((public_key.to_vec(), Vec::default(), Vec::default()));
    }

    // Otherwise check if tx data has length of 79
    // or more bytes (32 public key | 15 nonce | 16 ad | 16+ ciphertext)
    // If it is not, throw an ECDH error
    if tx_data.len() < ENCRYPTED_DATA_LEN {
        return Err(Error::ecdh_err("Wrong public key size"));
    }

    // Extract public key & encrypted data
    let public_key = &tx_data[..PUBLIC_KEY_SIZE];
    let encrypted_data = &tx_data[PUBLIC_KEY_SIZE..];
    let nonce = &encrypted_data[..NONCE_SIZE];

    Ok((public_key.to_vec(), encrypted_data.to_vec(), nonce.to_vec()))
}

/// Decrypts transaction data using derived shared secret
/// * encrypted_data - Encrypted data
/// * public_key - Public key provided by user
pub fn decrypt_transaction_data(
    encrypted_data: Vec<u8>,
    public_key: Vec<u8>,
    block_number: u64,
) -> Result<Vec<u8>, Error> {
    match &*UNSEALED_KEY_MANAGER {
        Some(key_manager) => {
            match key_manager.get_tx_key_by_block(block_number) {
                Some((_, tx_key)) => tx_key.decrypt(public_key, encrypted_data),
                None => Err(Error::encryption_err("There no keys stored in Epoch Manager")),
            }
        }
        None => Err(Error::encryption_err("Cannot unseal master key")),
    }
}

/// Encrypts transaction data or response
/// * data - Raw transaction data or node response
/// * public_key - Public key provided by user
pub fn encrypt_transaction_data(
    data: Vec<u8>,
    user_public_key: Vec<u8>,
    nonce: Vec<u8>,
    block_number: u64,
) -> Result<Vec<u8>, Error> {
    if user_public_key.len() != PUBLIC_KEY_SIZE {
        return Err(Error::ecdh_err("Wrong public key size"));
    }

    if nonce.len() != NONCE_SIZE {
        return Err(Error::ecdh_err("Wrong nonce size"));
    }

    match &*UNSEALED_KEY_MANAGER {
        Some(key_manager) => {
            match key_manager.get_tx_key_by_block(block_number) {
                Some((_, tx_key)) => tx_key.encrypt(user_public_key, data, nonce),
                None => Err(Error::encryption_err("There no keys stored in Epoch Manager")),
            }
        }
        None => Err(Error::encryption_err("Cannot unseal master key")),
    }
}

/// Encrypts provided plaintext using DEOXYS-II
/// * encryption_key - Encryption key which will be used for encryption
/// * plaintext - Data to encrypt
/// * encryption_salt - Arbitrary data which will be used as seed for derivation of nonce and ad fields
pub fn encrypt_deoxys(
    encryption_key: &[u8; PRIVATE_KEY_SIZE],
    plaintext: Vec<u8>,
    encryption_salt: Option<Vec<u8>>,
) -> Result<Vec<u8>, Error> {
    // Derive encryption salt if provided
    let encryption_salt = encryption_salt.and_then(|salt| {
        let mut hasher = kSha256::new();
        hasher.update(salt);
        let mut encryption_salt = [0u8; 32];
        encryption_salt.copy_from_slice(&hasher.finalize());
        Some(encryption_salt)
    });

    let nonce = match encryption_salt {
        // If salt was not provided, generate random nonce field
        None => {
            let mut nonce_buffer = [0u8; NONCE_SIZE];
            let result = unsafe { sgx_read_rand(&mut nonce_buffer as *mut u8, NONCE_SIZE) };
            match result {
                sgx_status_t::SGX_SUCCESS => nonce_buffer,
                _ => {
                    return Err(Error::encryption_err(format!(
                        "Cannot generate nonce: {:?}",
                        result.as_str()
                    )))
                }
            }
        }
        // Otherwise use encryption_salt as seed for nonce generation
        Some(encryption_salt) => {
            let mut rng = rand_chacha::ChaCha8Rng::from_seed(encryption_salt);
            let mut nonce = [0u8; NONCE_SIZE];
            rng.fill_bytes(&mut nonce);
            nonce
        }
    };

    let ad = [0u8; TAG_SIZE];

    // Construct cipher
    let cipher = DeoxysII::new(encryption_key);
    // Encrypt storage value
    let ciphertext = cipher.seal(&nonce, plaintext, ad);
    // Return concatenated nonce and ciphertext
    Ok([nonce.as_slice(), ad.as_slice(), &ciphertext].concat())
}

/// Decrypt DEOXYS-II encrypted ciphertext
pub fn decrypt_deoxys(
    encryption_key: &[u8; PRIVATE_KEY_SIZE],
    encrypted_value: Vec<u8>,
) -> Result<Vec<u8>, Error> {
    // 15 bytes nonce | 16 bytes tag size | >=16 bytes ciphertext
    if encrypted_value.len() < 47 {
        return Err(Error::decryption_err("corrupted ciphertext"));
    }

    // Extract nonce from encrypted value
    let nonce = &encrypted_value[..NONCE_SIZE];
    let nonce: [u8; 15] = match nonce.try_into() {
        Ok(nonce) => nonce,
        Err(_) => {
            return Err(Error::decryption_err("cannot extract nonce"));
        }
    };

    // Extract additional data
    let ad = &encrypted_value[NONCE_SIZE..NONCE_SIZE + TAG_SIZE];

    // Extract ciphertext
    let ciphertext = encrypted_value[NONCE_SIZE + TAG_SIZE..].to_vec();
    // Construct cipher
    let cipher = DeoxysII::new(encryption_key);
    // Decrypt ciphertext
    cipher.open(&nonce, ciphertext, ad).map_err(|err| {
        println!("[KeyManager] Cannot decrypt value. Reason: {:?}", err);
        Error::decryption_err("Cannot decrypt value")
    })
}
