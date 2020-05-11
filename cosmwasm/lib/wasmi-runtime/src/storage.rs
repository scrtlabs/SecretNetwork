use crate::keys::KeyPair;
use crate::keys::{SECRET_KEY_SIZE, UNCOMPRESSED_PUBLIC_KEY_SIZE};
use crate::utils::UnwrapOrSgxErrorUnexpected;
use enclave_ffi_types::EnclaveError;
use log::*;
use sgx_types::*;
use std::fmt;
use std::fs::File;
use std::io::{Error, Read, Write};
use std::sgxfs::SgxFile;

pub const SEED_SEALING_PATH: &str = "./.sgx_secrets/seed.sealed";
pub const NODE_SK_SEALING_PATH: &str = "./.sgx_secrets/node_sk_key.sealed";

pub trait SealedKey
where
    Self: std::marker::Sized,
{
    fn seal(&self, filepath: &str) -> Result<(), EnclaveError>;
    fn unseal(filepath: &str) -> Result<Self, EnclaveError>;
}

impl SealedKey for KeyPair {
    fn seal(&self, filepath: &str) -> Result<(), EnclaveError> {
        // Files are automatically closed when they go out of scope.
        let mut file = SgxFile::create(filepath).map_err(|err| {
            error!(
                "[Enclave] Dramatic error while trying to open {} to write: {:?}",
                filepath, err
            );
            EnclaveError::FailedUnseal
        })?;

        file.write_all(&self.get_privkey()).map_err(|err| {
            error!(
                "[Enclave] Dramatic error while trying to write to {}: {:?}",
                filepath, err
            );
            EnclaveError::FailedUnseal
        })
    }

    fn unseal(filepath: &str) -> Result<Self, EnclaveError> {
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

pub fn write_to_untrusted(bytes: &[u8], filepath: &str) -> SgxResult<sgx_status_t> {
    File::create(filepath)
        .map(|f| _write(bytes, f))
        .sgx_error_with_log(&format!("Creating file '{}' failed", filepath))?
}

fn _write<F: Write>(bytes: &[u8], mut file: F) -> SgxResult<sgx_status_t> {
    file.write_all(bytes)
        .sgx_error_with_log("[Enclave] Writing File failed!")?;

    Ok(sgx_status_t::SGX_SUCCESS)
}
