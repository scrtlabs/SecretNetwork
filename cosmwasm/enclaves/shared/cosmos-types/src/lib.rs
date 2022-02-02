// Trick to get the IDE to use sgx_tstd even when it doesn't know we're targeting SGX
#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;

pub mod multisig;
pub mod single_address;
pub mod traits;
pub mod types;
