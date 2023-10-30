#![cfg(feature = "random")]

use enclave_crypto::{AESKey, Hmac};
use log::trace;

pub fn create_random_proof(
    key: &AESKey,
    height: u64,
    random: &[u8],
    block_hash: &[u8],
) -> [u8; 32] {
    trace!(
        "Height: {:?}\nRandom: {:?}\nApphash: {:?}",
        height,
        random,
        block_hash
    );

    let height_bytes = height.to_be_bytes();

    let data_len = height_bytes.len() + random.len() + block_hash.len();
    let mut data = Vec::with_capacity(data_len);

    data.extend_from_slice(&height_bytes);
    data.extend_from_slice(random);
    data.extend_from_slice(block_hash);

    key.sign_sha_256(&data)
}

pub fn create_legacy_proof(
    key: &AESKey,
    height: u64,
    random: &[u8],
    block_hash: &[u8],
) -> [u8; 32] {
    let mut data = vec![];
    data.extend_from_slice(&height.to_be_bytes());
    data.extend_from_slice(random);
    data.extend_from_slice(block_hash);
    data.extend_from_slice(key.get());

    enclave_crypto::sha_256(data.as_slice())
}
