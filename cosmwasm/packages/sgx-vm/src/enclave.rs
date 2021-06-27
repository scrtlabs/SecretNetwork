use std::{env, path::Path, sync::Mutex};

use enclave_ffi_types::RuntimeConfiguration;
use sgx_types::{
    sgx_attributes_t, sgx_enclave_id_t, sgx_launch_token_t, sgx_misc_attribute_t, sgx_status_t,
    SgxResult,
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

    let mut enclave_file_path = None;
    let dirs = [
        enclave_directory.as_str(),
        "/lib",
        "/usr/lib",
        "/usr/local/lib",
    ];
    for dir in dirs.iter() {
        let candidate = Path::new(dir).join(ENCLAVE_FILE);
        trace!("Looking for the enclave file in {:?}", candidate.to_str());
        if candidate.exists() {
            enclave_file_path = Some(candidate);
            break;
        }
    }

    let enclave_file_path = enclave_file_path.ok_or_else(|| {
        warn!(
            "Cannot find the enclave file. Try pointing the SCRT_ENCLAVE_DIR environment variable to the directory that has {:?}",
            ENCLAVE_FILE
        );
        sgx_status_t::SGX_ERROR_INVALID_ENCLAVE
    })?;

    SgxEnclave::create(
        enclave_file_path,
        debug,
        &mut launch_token,
        &mut launch_token_updated,
        &mut misc_attr,
    )
}

#[allow(clippy::mutex_atomic)]
lazy_static! {
    static ref SGX_ENCLAVE: SgxResult<SgxEnclave> = init_enclave();
    /// This variable indicates if the enclave configuration has already been set
    static ref SGX_ENCLAVE_CONFIGURED: Mutex<bool> = Mutex::new(false);
}

/// Use this method when trying to get access to the enclave.
/// You can unwrap the result when you are certain that the enclave
/// must have been initialized if you even reached that point in the code.
pub fn get_enclave() -> SgxResult<&'static SgxEnclave> {
    SGX_ENCLAVE.as_ref().map_err(|status| *status)
}

extern "C" {
    pub fn ecall_configure_runtime(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        config: RuntimeConfiguration,
    ) -> sgx_status_t;
}

pub struct EnclaveRuntimeConfig {
    pub module_cache_size: u8,
}

impl EnclaveRuntimeConfig {
    fn to_ffi_type(&self) -> RuntimeConfiguration {
        RuntimeConfiguration {
            module_cache_size: self.module_cache_size,
        }
    }
}

pub fn configure_enclave(config: EnclaveRuntimeConfig) -> SgxResult<()> {
    let mut configured = SGX_ENCLAVE_CONFIGURED.lock().unwrap();
    if *configured {
        return Ok(());
    }
    *configured = true;
    drop(configured);

    let enclave = get_enclave()?;

    let mut retval = sgx_status_t::SGX_SUCCESS;

    let status =
        unsafe { ecall_configure_runtime(enclave.geteid(), &mut retval, config.to_ffi_type()) };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        return Err(retval);
    }

    Ok(())
}
