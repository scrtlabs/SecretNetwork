use crate::results::UnwrapOrSgxErrorUnexpected;

use sgx_types::*;
use std::io::Write;
use std::untrusted::fs::File;

pub fn write_to_untrusted(bytes: &[u8], filepath: &str) -> SgxResult<()> {
    let mut f = File::create(filepath)
        .sgx_error_with_log(&format!("Creating file '{}' failed", filepath))?;
    f.write_all(bytes)
        .sgx_error_with_log("[Enclave] Writing File failed!")
}
