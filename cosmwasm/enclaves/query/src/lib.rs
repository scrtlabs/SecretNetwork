// Trick to get the IDE to use sgx_tstd even when it doesn't know we're targeting SGX
#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;

extern crate sgx_types;

use ctor::*;
use enclave_utils::logger::get_log_level;

// Force linking to all the ecalls/ocalls in this package
pub use enclave_contract_engine;

#[cfg(feature = "production")]
#[ctor]
fn init_logger() {
    let default_log_level = log::Level::Warn;
    simple_logger::init_with_level(get_log_level(default_log_level)).unwrap();
}

#[cfg(all(not(feature = "production"), not(feature = "test")))]
#[ctor]
fn init_logger() {
    let default_log_level = log::Level::Trace;
    simple_logger::init_with_level(get_log_level(default_log_level)).unwrap();
}
