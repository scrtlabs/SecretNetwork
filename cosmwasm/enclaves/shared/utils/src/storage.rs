use crate::results::UnwrapOrSgxErrorUnexpected;

// use sgx_types::*;
use crate::sgx_status_t;
use std::fs::File;
use std::io::Write;

pub fn write_to_untrusted(bytes: &[u8], filepath: &str) -> Result<(), sgx_status_t> {
    let mut f = File::create(filepath)
        .sgx_error_with_log(&format!("Creating file '{}' failed", filepath))?;
    f.write_all(bytes)
        .sgx_error_with_log("[Enclave] Writing File failed!")
}
