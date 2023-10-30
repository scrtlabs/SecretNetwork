use std::time::Duration;

use parking_lot::{Condvar, Mutex};
use sgx_types::SgxResult;
use sgx_urts::SgxEnclave;

use crate::enclave::init_enclave;

/// This const determines how many seconds we wait when trying to get access to the enclave
/// before giving up.
const ENCLAVE_LOCK_TIMEOUT: u64 = 6 * 5;

pub struct EnclaveDoorbell {
    enclave: SgxResult<SgxEnclave>,
    condvar: Condvar,
    /// Amount of tasks allowed to use the enclave at the same time.
    count: Mutex<u8>,
}

impl EnclaveDoorbell {
    pub fn new(enclave_file: &str, count: u8, debug: i32) -> Self {
        // info!("Setting up enclave doorbell for up to {} threads", count);
        Self {
            enclave: init_enclave(enclave_file, debug),
            condvar: Condvar::new(),
            count: Mutex::new(count),
        }
    }

    fn wait_for(&'static self, duration: Duration, query_depth: u32) -> Option<EnclaveAccessToken> {
        if query_depth == 1 {
            let mut count = self.count.lock();
            if *count == 0 {
                // try to wait for other tasks to complete
                let wait = self.condvar.wait_for(&mut count, duration);
                // double check that the count is nonzero, so there's an available slot in the enclave.
                if wait.timed_out() || *count == 0 {
                    return None;
                }
            }
            *count -= 1;
        }
        Some(EnclaveAccessToken::new(self, query_depth))
    }

    pub fn get_access(&'static self, query_depth: u32) -> Option<EnclaveAccessToken> {
        self.wait_for(Duration::from_secs(ENCLAVE_LOCK_TIMEOUT), query_depth)
    }
}

#[allow(dead_code)]
pub struct EnclaveAccessToken {
    doorbell: &'static EnclaveDoorbell,
    pub enclave: SgxResult<&'static SgxEnclave>,
    query_depth: u32,
}

impl EnclaveAccessToken {
    fn new(doorbell: &'static EnclaveDoorbell, query_depth: u32) -> Self {
        let enclave = doorbell.enclave.as_ref().map_err(|status| *status);
        Self {
            doorbell,
            enclave,
            query_depth,
        }
    }
}
