use crate::results::UnwrapOrSgxErrorUnexpected;

use sgx_types::*;
use std::io::Write;
use std::untrusted::fs;
use std::untrusted::fs::File;

pub fn write_to_untrusted(bytes: &[u8], filepath: &str) -> SgxResult<()> {
    let mut f = File::create(filepath)
        .sgx_error_with_log(&format!("Creating file '{}' failed", filepath))?;
    f.write_all(bytes)
        .sgx_error_with_log("[Enclave] Writing File failed!")
}

pub fn rewrite_on_untrusted(bytes: &[u8], filepath: &str) -> SgxResult<()> {
    let is_path_exists = match fs::try_exists(filepath) {
        Ok(b) => b,
        Err(_) => false,
    };

    if is_path_exists {
        fs::remove_file(filepath)
            .sgx_error_with_log(&format!("Removing existing file '{}' failed", filepath))?;
    }

    write_to_untrusted(bytes, filepath)
}
