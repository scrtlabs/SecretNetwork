use crate::crypto::keys::{AESKey, KeyPair};
use crate::crypto::keys::{SECRET_KEY_SIZE, UNCOMPRESSED_PUBLIC_KEY_SIZE};
use crate::crypto::traits::SealedKey;

use enclave_ffi_types::EnclaveError;
use log::*;
use sgx_types::*;
use std::fmt;
use std::fs::File;
use std::io::{Error, Read, Write};
use std::sgxfs::SgxFile;


impl SealedKey for AESKey {
    fn seal(&self, filepath: &str) -> Result<(), EnclaveError> {
        seal(self.get(), filepath)
    }

    fn unseal(filepath: &str) -> Result<Self, EnclaveError> {
        let mut buf = open(filepath)?;
        Ok(Self::new_from_slice(&buf))
    }
}

impl SealedKey for KeyPair {
    fn seal(&self, filepath: &str) -> Result<(), EnclaveError> {
        // Files are automatically closed when they go out of scope.
        let mut buf = [0u8; 32];

        buf.copy_from_slice(self.get_privkey());

        seal(&buf, filepath)
    }

    fn unseal(filepath: &str) -> Result<Self, EnclaveError> {
        let mut buf = open(filepath)?;

        KeyPair::new_from_slice(&buf).map_err(|err| {
            error!(
                "{}",
                format!(
                    "[Enclave] Dramatic error while trying to init secret key from bytes: {:?}",
                    err
                )
            );
            EnclaveError::FailedUnseal
        })
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
