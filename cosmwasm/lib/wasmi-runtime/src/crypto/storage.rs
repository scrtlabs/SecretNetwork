use crate::crypto::traits::SealedKey;
use crate::crypto::{AESKey, KeyPair, Seed, SECRET_KEY_SIZE};

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
        Ok(Self::new_from_slice(&buf))
    }
}

impl SealedKey for Seed {
    fn seal(&self, filepath: &str) -> Result<(), EnclaveError> {
        seal(self.get(), filepath)
    }

    fn unseal(filepath: &str) -> Result<Self, EnclaveError> {
        let buf = open(filepath)?;
        Ok(Self::new_from_slice(&buf))
    }
}

impl SealedKey for KeyPair {
    fn seal(&self, filepath: &str) -> Result<(), EnclaveError> {
        // Files are automatically closed when they go out of scope.
        //let mut buf = [0u8; 32];

        //buf.copy_from_slice();

        seal(&self.get_privkey(), filepath)
    }

    fn unseal(filepath: &str) -> Result<Self, EnclaveError> {
        let buf = open(filepath)?;

        KeyPair::new_from_slice(buf).map_err(|err| EnclaveError::FailedUnseal)
    }
}

fn seal(data: &[u8; 32], filepath: &str) -> Result<(), EnclaveError> {
    let mut file = SgxFile::create(filepath).map_err(|err| {
        error!(
            "[Enclave] Dramatic error while trying to open {} to write: {:?}",
            filepath, err
        );
        EnclaveError::FailedUnseal
    })?;

    file.write_all(data).map_err(|err| {
        error!(
            "[Enclave] Dramatic error while trying to write to {}: {:?}",
            filepath, err
        );
        EnclaveError::FailedUnseal
    })
}

fn open(filepath: &str) -> Result<[u8; SECRET_KEY_SIZE], EnclaveError> {
    let mut file = SgxFile::open(filepath).map_err(|err| {
        error!(
            "[Enclave] Dramatic error while trying to open {} to read: {:?}",
            filepath, err
        );
        EnclaveError::FailedUnseal
    })?;

    let mut buf = [0_u8; SECRET_KEY_SIZE];
    let n = file.read(&mut buf).map_err(|err| {
        error!(
            "[Enclave] Dramatic error while trying to read from {}: {:?}",
            filepath, err
        );
        EnclaveError::FailedUnseal
    })?;

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

    use super::{open, seal};

    // todo: fix test vectors to actually work
    fn test_seal() {
        let key = b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";

        if let Err(e) = seal(key, "file") {
            error!("Failed to seal data: {:?}", e)
        };
    }

    // todo: fix test vectors to actually work
    fn test_open() {
        let key = b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";

        if let Err(e) = seal(key, "file") {
            error!("Failed to seal data: {:?}", e)
            // todo: fail
        };

        data = match open("file") {
            Err(e) => {
                error!("Failed to open data: {:?}", e)
                // todo: fail
            }
            Ok(res) => res,
        };

        assert_eq!(data, key);
    }
}
