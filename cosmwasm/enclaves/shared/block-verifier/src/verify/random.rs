use enclave_crypto::{sha_256, KEY_MANAGER};

pub fn create_proof(height: u64, random: &[u8], block_hash: &[u8]) -> [u8; 32] {
    let irs = KEY_MANAGER.initial_randomness_seed.unwrap();

    let height_bytes = height.to_be_bytes();
    let irs_bytes = irs.get();

    let data_len = height_bytes.len() + random.len() + block_hash.len() + irs_bytes.len();
    let mut data = Vec::with_capacity(data_len);

    data.extend_from_slice(&height_bytes);
    data.extend_from_slice(random);
    data.extend_from_slice(block_hash);
    data.extend_from_slice(irs_bytes);

    sha_256(data.as_slice())
}
