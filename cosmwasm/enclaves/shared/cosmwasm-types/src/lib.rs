// Trick to get the IDE to use sgx_tstd even when it doesn't know we're targeting SGX
#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;

pub mod coins;
pub mod consts;
pub mod encoding;
pub mod math;
pub mod query;
pub mod std_error;
pub mod system_error;
pub mod types;
