use enclave_crypto::{sha_256, KEY_MANAGER};

pub fn create_proof(height: u64, random: &[u8], block_hash: &[u8]) -> [u8; 32] {
    let mut data = vec![];

    let irs = KEY_MANAGER.initial_randomness_seed.unwrap();

    data.extend_from_slice(&height.to_be_bytes());
    data.extend_from_slice(random);
    data.extend_from_slice(block_hash);
    data.extend_from_slice(irs.get());
    // let mut hasher = fSha256::new();
    // hasher.update(&height.to_be_bytes());
    // hasher.update(random);
    // hasher.update(block_hash);
    // hasher.update(IRS.get());

    sha_256(data.as_slice())
}
