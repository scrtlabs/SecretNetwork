use crate::results::UnwrapOrSgxErrorUnexpected;

use std::io::{Read, Write};
use std::sgxfs::SgxFile;

use sgx_types::*;
use std::untrusted::fs;
use std::untrusted::fs::File;

pub const SCRT_SGX_STORAGE_ENV_VAR: &str = "SCRT_SGX_STORAGE";
pub const DEFAULT_SGX_SECRET_PATH: &str = "/opt/secret/.sgx_secrets/";

pub fn write_to_untrusted(bytes: &[u8], filepath: &str) -> SgxResult<()> {
    let mut f = File::create(filepath)
        .sgx_error_with_log(&format!("Creating file '{}' failed", filepath))?;
    f.write_all(bytes)
        .sgx_error_with_log("Writing File failed!")
}

pub fn seal(data: &[u8], filepath: &str) -> SgxResult<()> {
    let mut file = SgxFile::create(filepath)
        .sgx_error_with_log(&format!("Creating sealed file '{}' failed", filepath))?;

    file.write_all(data)
        .sgx_error_with_log("Writing sealed file failed!")
}

pub fn unseal(filepath: &str) -> SgxResult<Vec<u8>> {
    let mut file = SgxFile::open(filepath)
        .sgx_error_with_log(&format!("Opening sealed file '{}' failed", filepath))?;

    let mut output = vec![];
    file.read_to_end(&mut output)
        .sgx_error_with_log(&format!("Reading sealed file '{}' failed", filepath))?;

    Ok(output)
}

pub fn rewrite_on_untrusted(bytes: &[u8], filepath: &str) -> SgxResult<()> {
    let is_path_exists = fs::try_exists(filepath).unwrap_or(false);

    if is_path_exists {
        fs::remove_file(filepath)
            .sgx_error_with_log(&format!("Removing existing file '{}' failed", filepath))?;
    }

    write_to_untrusted(bytes, filepath)
}
