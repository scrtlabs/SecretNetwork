use enclave_ffi_types::{EnclaveBuffer, RuntimeConfiguration};
use std::env;
use std::ops::Deref;
use std::path::Path;
use std::time::Duration;

use sgx_types::{
    sgx_attributes_t, sgx_enclave_id_t, sgx_launch_token_t, sgx_misc_attribute_t, sgx_status_t,
    SgxResult,
};
use sgx_urts::SgxEnclave;

use lazy_static::lazy_static;
use log::*;
use parking_lot::{Mutex, ReentrantMutex, ReentrantMutexGuard};

use crate::wasmi::imports;

static ENCLAVE_FILE: &str = "librust_cosmwasm_enclave.signed.so";

#[cfg(feature = "production")]
const ENCLAVE_DEBUG: i32 = 0;

#[cfg(not(feature = "production"))]
const ENCLAVE_DEBUG: i32 = 1;

struct EnclaveMutex {
    enclave: ReentrantMutex<SgxEnclave>,
}

impl EnclaveMutex {
    fn new() -> SgxResult<EnclaveMutex> {
        let enclave = ReentrantMutex::new(init_enclave()?);
        Ok(Self { enclave })
    }

    fn get_enclave(&'static self, timeout: Duration) -> Option<EnclaveGuard> {
        let guard = self.enclave.try_lock_for(timeout);
        guard.map(|guard| EnclaveGuard { guard })
    }
}

pub struct EnclaveGuard {
    guard: ReentrantMutexGuard<'static, SgxEnclave>,
}

impl Deref for EnclaveGuard {
    type Target = SgxEnclave;

    fn deref(&self) -> &Self::Target {
        self.guard.deref()
    }
}

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
    static ref SGX_ENCLAVE_MUTEX: SgxResult<EnclaveMutex> = EnclaveMutex::new();

    /// This variable indicates if the enclave configuration has already been set
    static ref SGX_ENCLAVE_CONFIGURED: Mutex<bool> = Mutex::new(false);
}

/// This const determines how many seconds we wait when trying to get access to the enclave
/// before giving up.
const ENCLAVE_LOCK_TIMEOUT: u64 = 6;

/// Use this method when trying to get access to the enclave.
/// You can unwrap the result when you are certain that the enclave
/// must have been initialized if you even reached that point in the code.
/// If `Ok(None)` is returned, that means that the enclave is currently busy.
pub fn get_enclave() -> SgxResult<Option<EnclaveGuard>> {
    let mutex = SGX_ENCLAVE_MUTEX.as_ref().map_err(|status| *status)?;
    let maybe_guard = mutex.get_enclave(Duration::from_secs(ENCLAVE_LOCK_TIMEOUT));
    Ok(maybe_guard)
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
    let mut configured = SGX_ENCLAVE_CONFIGURED.lock();
    if *configured {
        return Ok(());
    }
    *configured = true;
    drop(configured);

    let enclave = get_enclave()?
        .expect("This function should only be called once when the node is initializing");

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

/// This is a safe wrapper for allocating buffers inside the enclave.
///
/// It must be called after the enclave has been initialized, and can not be called
/// while another thread is using the enclave, or it will panic.
pub(super) fn allocate_enclave_buffer(buffer: &[u8]) -> SgxResult<EnclaveBuffer> {
    let ptr = buffer.as_ptr();
    let len = buffer.len();
    let mut enclave_buffer = EnclaveBuffer::default();

    let enclave_id = get_enclave()
        .expect("If we got here, surely the enclave has been loaded")
        .expect("If we got here, surely we are the thread that holds the enclave")
        .geteid();

    trace!(
        target: module_path!(),
        "allocate_enclave_buffer() called with len: {:?} enclave_id: {:?}",
        len,
        enclave_id,
    );

    match unsafe { imports::ecall_allocate(enclave_id, &mut enclave_buffer, ptr, len) } {
        sgx_status_t::SGX_SUCCESS => Ok(enclave_buffer),
        failure_status => Err(failure_status),
    }
}
