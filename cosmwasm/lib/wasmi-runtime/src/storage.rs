use std::fs::File;
use std::io::{Error, Write};
use std::sgxfs::SgxFile;

pub const NODE_SK_SEALING_PATH: &str = "./.sgx_secrets/sk_node.sealed";

pub fn seal(bytes: &[u8], filepath: &str) -> Result<(), Error> {
    // Files are automatically closed when they go out of scope.
    let mut file = SgxFile::create(filepath)?;

    file.write_all(bytes)
}
use log::*;

use sgx_types::*;

use crate::utils::UnwrapOrSgxErrorUnexpected;

fn _write<F: Write>(bytes: &[u8], mut file: F) -> SgxResult<sgx_status_t> {
    file.write_all(bytes)
        .sgx_error_with_log("[Enclave] Writing File failed!")?;

    Ok(sgx_status_t::SGX_SUCCESS)
}

pub fn write_to_untrusted(bytes: &[u8], filepath: &str) -> SgxResult<sgx_status_t> {
    File::create(filepath)
        .map(|f| _write(bytes, f))
        .sgx_error_with_log(&format!("Creating file '{}' failed", filepath))?
}
