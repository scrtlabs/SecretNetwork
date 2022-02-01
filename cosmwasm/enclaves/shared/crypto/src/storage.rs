use crate::ed25519::Ed25519PrivateKey;
use crate::traits::SealedKey;
use crate::{AESKey, KeyPair, Seed, SECRET_KEY_SIZE};
use enclave_ffi_types::EnclaveError;
use log::*;
use std::io::{Read, Write};
use std::sgxfs::SgxFile;

impl SealedKey for AESKey {
    fn seal(&self, filepath: &str) -> Result<(), EnclaveError> {
        seal(self.get(), filepath)
    }

    fn unseal(filepath: &str) -> Result<Self, EnclaveError> {
        let buf = open(filepath)?;
        Ok(Self::from(buf))
    }
}

impl SealedKey for Seed {
    fn seal(&self, filepath: &str) -> Result<(), EnclaveError> {
        seal(&self.as_slice(), filepath)
    }

    fn unseal(filepath: &str) -> Result<Self, EnclaveError> {
        let buf = open(filepath)?;
        Ok(Self::from(buf))
    }
}

impl SealedKey for KeyPair {
    fn seal(&self, filepath: &str) -> Result<(), EnclaveError> {
        // Files are automatically closed when they go out of scope.
        seal(&self.get_privkey(), filepath)
    }

    fn unseal(filepath: &str) -> Result<Self, EnclaveError> {
        let buf = open(filepath)?;
        Ok(KeyPair::from(buf))
    }
}

fn seal(data: &[u8; 32], filepath: &str) -> Result<(), EnclaveError> {
    let mut file = SgxFile::create(filepath).map_err(|_err| {
        error!("error creating file {}: {:?}", filepath, _err);
        EnclaveError::FailedSeal
    })?;

    file.write_all(data).map_err(|_err| {
        error!("error writing to path {}: {:?}", filepath, _err);
        EnclaveError::FailedSeal
    })
}

fn open(filepath: &str) -> Result<Ed25519PrivateKey, EnclaveError> {
    let mut file = SgxFile::open(filepath).map_err(|_err| EnclaveError::FailedUnseal)?;

    let mut buf = Ed25519PrivateKey::default();
    let n = file
        .read(buf.as_mut())
        .map_err(|_err| EnclaveError::FailedUnseal)?;

    if n < SECRET_KEY_SIZE {
        error!(
            "[Enclave] Dramatic read from {} ended prematurely (n = {} < SECRET_KEY_SIZE = {})",
            filepath, n, SECRET_KEY_SIZE
        );
        return Err(EnclaveError::FailedUnseal);
    }
    Ok(buf)
}

#[cfg(feature = "test")]
pub mod tests {

    // use super::{open, seal};
    // use log::*;

    // // todo: fix test vectors to actually work
    // pub fn test_seal() {
    //     let key = b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
    //
    //     if let Err(e) = seal(key, "file") {
    //         error!("Failed to seal data: {:?}", e)
    //     };
    // }

    // // todo: fix test vectors to actually work
    // pub fn test_open() {
    //     let key = b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
    //
    //     if let Err(e) = seal(key, "file") {
    //         error!("Failed to seal data: {:?}", e)
    //         // todo: fail
    //     };
    //
    //     let data = open("file").expect(&format!("Failed to open data: {:?}", e));
    //
    //     assert_eq!(data, key);
    // }
}
