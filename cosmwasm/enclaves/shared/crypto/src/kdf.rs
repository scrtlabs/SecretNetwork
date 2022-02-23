use crate::traits::Kdf;
use crate::{AESKey, Seed, SECRET_KEY_SIZE};

use ring::hkdf;

// Bitcoin halving block hash https://www.blockchain.com/btc/block/000000000000000000024bead8df69990852c202db0e0097c1a12ea637d7e96d
const KDF_SALT: [u8; 32] = [
    0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x4b, 0xea, 0xd8, 0xdf, 0x69, 0x99,
    0x08, 0x52, 0xc2, 0x02, 0xdb, 0x0e, 0x00, 0x97, 0xc1, 0xa1, 0x2e, 0xa6, 0x37, 0xd7, 0xe9, 0x6d,
];

impl Kdf<AESKey> for AESKey {
    fn derive_key_from_this(&self, data: &[u8]) -> Self {
        let mut input_bytes: Vec<u8> = self.get().to_vec();
        input_bytes.extend_from_slice(data);

        derive_key(&input_bytes, &[])
    }
}

impl Kdf<AESKey> for Seed {
    //
    fn derive_key_from_this(&self, data: &[u8]) -> AESKey {
        let mut input_bytes: Vec<u8> = self.as_slice().to_vec();
        input_bytes.extend_from_slice(data);

        derive_key(&input_bytes, &[b"seed"])
    }
}

fn derive_key(input_bytes: &[u8], info: &[&[u8]]) -> AESKey {
    let salt = hkdf::Salt::new(hkdf::HKDF_SHA256, &KDF_SALT);

    let prk = salt.extract(input_bytes);

    let okm = prk.expand(info, My(SECRET_KEY_SIZE)).unwrap();

    let mut result: AESKey = AESKey::default();

    let _ = okm.fill(result.as_mut());

    result
}

/// https://github.com/briansmith/ring/blob/master/tests/hkdf_tests.rs
/// Generic newtype wrapper that lets us implement traits for externally-defined
/// types.
#[derive(Debug, PartialEq)]
struct My<T: core::fmt::Debug + PartialEq>(T);

impl hkdf::KeyType for My<usize> {
    fn len(&self) -> usize {
        self.0
    }
}

impl From<hkdf::Okm<'_, My<usize>>> for My<Vec<u8>> {
    fn from(okm: hkdf::Okm<My<usize>>) -> Self {
        let mut r = vec![0u8; okm.len().0];
        okm.fill(&mut r).unwrap();
        My(r)
    }
}

#[cfg(feature = "test")]
pub mod tests {
    // use crate::crypto::CryptoError;
    // use crate::crypto::{Kdf, KeyPair, Seed};

    // commented since this is all outdated
    // // todo: fix test vectors to actually work
    // pub fn test_derive_key() {
    //     let seed = Seed::new_from_slice(&[10u8; 32]);
    //
    //     let kdf1 = seed.derive_key_from_this(&1.to_be_bytes());
    //     let kdf2 = seed.derive_key_from_this(&2.to_be_bytes());
    //
    //     assert_eq!(kdf1, b"SOME VALUE");
    //     assert_eq!(kdf2, b"SOME VALUE");
    // }
}
