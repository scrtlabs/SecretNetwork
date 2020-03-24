use thiserror::Error;

#[derive(Debug, Error)]
enum Error {
    #[error(transparent)]
    CBindgenError {
        #[from]
        source: cbindgen::Error,
    },
}

use std::env;

fn main() -> Result<(), Error> {
    let crate_dir = env::var("CARGO_MANIFEST_DIR").unwrap();

    cbindgen::generate(crate_dir)?.write_to_file("enclave-ffi.h");

    Ok(())
}
