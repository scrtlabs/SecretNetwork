#[cfg(feature = "build_headers")]
use std::env;
#[cfg(feature = "build_headers")]
use std::path::PathBuf;

#[cfg(feature = "build_headers")]
use thiserror::Error;

#[cfg(feature = "build_headers")]
#[derive(Debug, Error)]
enum Error {
    #[error(transparent)]
    CBindgenError {
        #[from]
        source: cbindgen::Error,
    },
    #[error("{path}")]
    BadOutDir { path: PathBuf },
}
#[cfg(feature = "build_headers")]
fn main() -> Result<(), Error> {
    let crate_dir = env::var("CARGO_MANIFEST_DIR").unwrap();
    // This is a directory under the `target` directory of the crate building us.
    let out_dir = PathBuf::from(env::var("OUT_DIR").unwrap());
    // This path will point to a file under the `target/headers` directory of whoever's building us.
    let header_path = {
        let mut path = out_dir.clone();
        while path.file_name() != Some(&std::ffi::OsString::from("target")) {
            // If for some reason we scanned the entire path and failed to find the `target` directory, return an error
            if !path.pop() {
                return Err(Error::BadOutDir { path: out_dir });
            }
        }
        path.push("headers");
        path.push("enclave-ffi-types.h"); // This should always equal the crate name
        path
    };

    cbindgen::generate(crate_dir)?.write_to_file(header_path);

    Ok(())
}

#[cfg(not(feature = "build_headers"))]
fn main() {}
