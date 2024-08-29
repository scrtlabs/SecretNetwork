use sgx_types::*;
use parking_lot::{Condvar, Mutex};
use sgx_urts::SgxEnclave;
use std::env;
use std::ops::Deref;
use std::time::Duration;

#[cfg(not(feature = "attestation_server"))]
static ENCLAVE_FILE: &'static str = "v1.0.3_enclave.signed.so";

#[cfg(feature = "attestation_server")]
static ENCLAVE_FILE: &'static str = "v1.0.3_attestation_enclave.signed.so";

#[cfg(feature = "attestation_server")]
const ENCLAVE_LOCK_TIMEOUT: u64 = 6 * 50;

#[cfg(not(feature = "attestation_server"))]
const ENCLAVE_LOCK_TIMEOUT: u64 = 6 * 5;

pub struct EnclaveDoorbell {
    enclave: SgxResult<SgxEnclave>,
    condvar: Condvar,
    /// Amount of tasks allowed to use the enclave at the same time.
    count: Mutex<u8>,
}

impl EnclaveDoorbell {
    pub fn new() -> Self {
        println!("[Enclave Doorbell] Setting up enclave doorbell");

        let mut launch_token: sgx_launch_token_t = [0; 1024];
        let mut launch_token_updated: i32 = 0;
        let debug = 0;
        let mut misc_attr = sgx_misc_attribute_t {
            secs_attr: sgx_attributes_t { flags: 0, xfrm: 0 },
            misc_select: 0,
        };
    
        let enclave_home = env::var("ENCLAVE_HOME").unwrap_or_else(|_| {
            let dir_path = String::from(
                std::env::home_dir()
                    .expect("Please specify ENCLAVE_HOME env variable explicitly")
                    .to_str()
                    .unwrap(),
            );
            format!("{}/.swisstronik-enclave", dir_path)
        });
        let enclave_path = format!("{}/{}", enclave_home, ENCLAVE_FILE);
    
        println!(
            "[Enclave Doorbell] Creating enclave. Enclave location: {:?}",
            enclave_path
        );
    
        let enclave = SgxEnclave::create(
            enclave_path,
            debug,
            &mut launch_token,
            &mut launch_token_updated,
            &mut misc_attr,
        );

        Self {
            enclave,
            condvar: Condvar::new(),
            count: Mutex::new(8),
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

// NEVER add Clone or Copy
pub struct EnclaveAccessToken {
    doorbell: &'static EnclaveDoorbell,
    enclave: SgxResult<&'static SgxEnclave>,
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

impl Deref for EnclaveAccessToken {
    type Target = SgxResult<&'static SgxEnclave>;

    fn deref(&self) -> &Self::Target {
        &self.enclave
    }
}

impl Drop for EnclaveAccessToken {
    fn drop(&mut self) {
        if self.query_depth == 1 {
            let mut count = self.doorbell.count.lock();
            *count += 1;
            drop(count);
            self.doorbell.condvar.notify_one();
        }
    }
}
