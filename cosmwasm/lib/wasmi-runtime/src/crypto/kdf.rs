use crate::crypto::traits::Kdf;
use crate::crypto::keys::AESKey;

impl Kdf for AESKey {
    fn derive_key_from_this(&self) -> Self {
        return AESKey::new_from_slice(self.get())
    }
}
