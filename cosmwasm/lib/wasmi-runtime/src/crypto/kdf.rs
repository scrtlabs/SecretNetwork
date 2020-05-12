use crate::crypto::keys::{Seed, SECRET_KEY_SIZE};
use crate::crypto::traits::Kdf;

impl Kdf for Seed {
    fn derive_key_from_this(&self, counter: u32) -> [u8; SECRET_KEY_SIZE] {
        self.get().clone()
    }
}
