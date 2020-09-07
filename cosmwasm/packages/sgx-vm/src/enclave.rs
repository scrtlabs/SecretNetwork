use std::{
    env,
    path::{Path, PathBuf},
};

use sgx_types::{
    sgx_attributes_t, sgx_launch_token_t, sgx_misc_attribute_t, sgx_status_t, SgxResult,
};
use sgx_urts::SgxEnclave;

use lazy_static::lazy_static;
use log::*;

static ENCLAVE_FILE: &str = "librust_cosmwasm_enclave.signed.so";

#[cfg(feature = "production")]
const ENCLAVE_DEBUG: i32 = 0;

#[cfg(not(feature = "production"))]
const ENCLAVE_DEBUG: i32 = 1;

fn init_enclave() -> SgxResult<SgxEnclave> {
    let mut launch_token: sgx_launch_token_t = [0; 1024];
    let mut launch_token_updated: i32 = 0;
    // call sgx_create_enclave to initialize an enclave instance
    // Debug Support: set 2nd parameter to 1
    let debug: i32 = ENCLAVE_DEBUG;
    let mut misc_attr = sgx_misc_attribute_t {
        secs_attr: sgx_attributes_t { flags: 0, xfrm: 0 },
        misc_select: 0,
    };

    // Step : try to create a .enigma folder for storing all the files
    // Create a directory, returns `io::Result<()>`
    let enclave_directory = env::var("SCRT_ENCLAVE_DIR").unwrap_or_else(|_| '.'.to_string());

    let path = Path::new(&enclave_directory);

    let mut enclave_file_path: PathBuf = path.join(ENCLAVE_FILE);

    trace!(
        "Looking for the enclave file in {:?}",
        enclave_file_path.to_str()
    );

    if !enclave_file_path.exists() {
        enclave_file_path = Path::new("/lib").join(ENCLAVE_FILE);

        trace!(
            "Looking for the enclave file in {:?}",
            enclave_file_path.to_str()
        );
        if !enclave_file_path.exists() {
            enclave_file_path = Path::new("/usr/lib").join(ENCLAVE_FILE);

            trace!(
                "Looking for the enclave file in {:?}",
                enclave_file_path.to_str()
            );
            if !enclave_file_path.exists() {
                enclave_file_path = Path::new("/usr/local/lib").join(ENCLAVE_FILE);
            }
        }
    }

    if !enclave_file_path.exists() {
        warn!(
            "Cannot find the enclave file. Try pointing the SCRT_ENCLAVE_DIR envirinment variable to the directory that has {:?}",
            ENCLAVE_FILE
        );
        return Err(sgx_status_t::SGX_ERROR_INVALID_ENCLAVE);
    }

    SgxEnclave::create(
        enclave_file_path,
        debug,
        &mut launch_token,
        &mut launch_token_updated,
        &mut misc_attr,
    )
}

lazy_static! {
    static ref SGX_ENCLAVE: SgxResult<SgxEnclave> = init_enclave();
}

/// Use this method when trying to get access to the enclave.
/// You can unwrap the result when you are certain that the enclave
/// must have been initialized if you even reached that point in the code.
pub fn get_enclave() -> SgxResult<&'static SgxEnclave> {
    SGX_ENCLAVE.as_ref().map_err(|status| *status)
}
