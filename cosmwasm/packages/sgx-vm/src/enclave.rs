use std::ops::Deref;
use std::time::Duration;
use std::{env, path::Path};

use sgx_types::{
    sgx_attributes_t, sgx_launch_token_t, sgx_misc_attribute_t, sgx_status_t, SgxResult,
};
use sgx_urts::SgxEnclave;

use lazy_static::lazy_static;
use log::*;
use parking_lot::{Condvar, Mutex};

#[cfg(feature = "production")]
const ENCLAVE_DEBUG: i32 = 0;

#[cfg(not(feature = "production"))]
const ENCLAVE_DEBUG: i32 = 1;

fn init_enclave(enclave_file: &str) -> SgxResult<SgxEnclave> {
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
        let candidate = Path::new(dir).join(enclave_file);
        trace!("Looking for the enclave file in {:?}", candidate.to_str());
        if candidate.exists() {
            enclave_file_path = Some(candidate);
            break;
        }
    }

    let enclave_file_path = enclave_file_path.ok_or_else(|| {
        warn!(
            "Cannot find the enclave file. Try pointing the SCRT_ENCLAVE_DIR environment variable to the directory that has {:?}",
            enclave_file
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

static ENCLAVE_FILE: &str = "librust_cosmwasm_enclave.signed.so";
#[cfg(feature = "query-node")]
static QUERY_ENCLAVE_FILE: &str = "librust_cosmwasm_query_enclave.signed.so";
/// This const determines how many seconds we wait when trying to get access to the enclave
/// before giving up.
const ENCLAVE_LOCK_TIMEOUT: u64 = 6 * 5;
#[cfg(feature = "query-node")]
const TCS_NUM: u8 = 8;
#[cfg(not(feature = "query-node"))]
const TCS_NUM: u8 = 1;
lazy_static! {
    pub static ref ENCLAVE_DOORBELL: EnclaveDoorbell = EnclaveDoorbell::new(ENCLAVE_FILE, TCS_NUM);
}
#[cfg(feature = "query-node")]
lazy_static! {
    pub static ref QUERY_ENCLAVE_DOORBELL: EnclaveDoorbell = EnclaveDoorbell::new(
        QUERY_ENCLAVE_FILE,
        std::cmp::min(TCS_NUM, num_cpus::get() as u8)
    );
}

/// This struct manages the access to the enclave.
///
/// It effectively works as a custom, non-generic Semaphore. We need to make sure that the enclave
/// is not entered more than TCS_NUM times at once, except that entering it recursively from the
/// same thread is always permitted.
/// `EnclaveDoorbell` and `EnclaveAccessToken` help control this behavior.
/// To ensure that goroutines don't change threads between recursive accesses to the enclave,
/// we use `runtime.LockOSThread()` and `runtime.UnlockOSThread()` before leaving Go-land.
pub struct EnclaveDoorbell {
    enclave: SgxResult<SgxEnclave>,
    condvar: Condvar,
    /// Amount of tasks allowed to use the enclave at the same time.
    count: Mutex<u8>,
}

impl EnclaveDoorbell {
    fn new(enclave_file: &str, count: u8) -> Self {
        info!("Setting up enclave doorbell for up to {} threads", count);
        Self {
            enclave: init_enclave(enclave_file),
            condvar: Condvar::new(),
            count: Mutex::new(count),
        }
    }

    fn wait_for(&'static self, duration: Duration, recursive: bool) -> Option<EnclaveAccessToken> {
        // eprintln!("Query Token creation. recursive: {}", recursive);
        if !recursive {
            let mut count = self.count.lock();
            // eprintln!(
            //     "The current count of tasks is {}/{}, attempting to increase.",
            //     TCS_NUM - *count,
            //     TCS_NUM
            // );
            if *count == 0 {
                // eprintln!("Waiting for other tasks to complete");
                // try to wait for other tasks to complete
                let wait = self.condvar.wait_for(&mut count, duration);
                // double check that the count is nonzero, so there's an available slot in the enclave.
                if wait.timed_out() || *count == 0 {
                    return None;
                }
            }
            // eprintln!("Increasing available tasks");
            *count -= 1;
        }
        Some(EnclaveAccessToken::new(self, recursive))
    }

    pub fn get_access(&'static self, recursive: bool) -> Option<EnclaveAccessToken> {
        self.wait_for(Duration::from_secs(ENCLAVE_LOCK_TIMEOUT), recursive)
    }
}

// NEVER add Clone or Copy
pub struct EnclaveAccessToken {
    doorbell: &'static EnclaveDoorbell,
    enclave: SgxResult<&'static SgxEnclave>,
    recursive: bool,
}

impl EnclaveAccessToken {
    fn new(doorbell: &'static EnclaveDoorbell, recursive: bool) -> Self {
        let enclave = doorbell.enclave.as_ref().map_err(|status| *status);
        Self {
            doorbell,
            enclave,
            recursive,
        }
    }
}

impl Deref for EnclaveAccessToken {
    type Target = SgxResult<&'static SgxEnclave>;

    fn deref(&self) -> &Self::Target {
        &self.enclave
    }
}

impl Drop for EnclaveAccessToken {
    fn drop(&mut self) {
        // eprintln!("Query Token destruction. recursive: {}", self.recursive);
        if !self.recursive {
            let mut count = self.doorbell.count.lock();
            // eprintln!(
            //     "The current count of tasks is {}/{}, attempting to decrease.",
            //     TCS_NUM - *count,
            //     TCS_NUM
            // );
            *count += 1;
            drop(count);
            self.doorbell.condvar.notify_one();
        }
    }
}
