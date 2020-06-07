use crate::utils::UnwrapOrSgxErrorUnexpected;

use sgx_types::*;
use std::io::Write;
use std::untrusted::fs::File;

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
