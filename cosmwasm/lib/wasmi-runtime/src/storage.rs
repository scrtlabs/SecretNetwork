use std::io::{Error, Write};
use std::sgxfs::SgxFile;

pub const NODE_SK_SEALING_PATH: &str = "./.sgx_secrets/sk_node.sealed";

pub fn seal(bytes: &[u8], filepath: &str) -> Result<(), Error> {
    // Files are automatically closed when they go out of scope.
    let mut file = SgxFile::create(filepath)?;

    file.write_all(bytes)
}
