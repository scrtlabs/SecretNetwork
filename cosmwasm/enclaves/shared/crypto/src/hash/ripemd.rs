use ripemd160::{Digest, Ripemd160};

pub const RIPEMD160_HASH_SIZE: usize = 20;

pub fn ripemd160(data: &[u8]) -> [u8; RIPEMD160_HASH_SIZE] {
    let mut hasher = Ripemd160::new();
    hasher.update(data);
    let hash = hasher.finalize().to_vec();

    let mut result = [0u8; RIPEMD160_HASH_SIZE];

    result.copy_from_slice(hash.as_ref());

    result
}
