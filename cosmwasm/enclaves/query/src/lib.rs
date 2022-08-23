// Trick to get the IDE to use sgx_tstd even when it doesn't know we're targeting SGX
#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;

extern crate sgx_types;

#[allow(unused_imports)]
use ctor::*;

// Force linking to all the ecalls/ocalls in this package
pub use enclave_contract_engine;

#[cfg(feature = "production")]
#[ctor]
fn init_logger() {
    simple_logger::init_with_level(log::Level::Warn).unwrap();
}

#[cfg(all(not(feature = "production"), not(feature = "test")))]
#[ctor]
fn init_logger() {
    simple_logger::init_with_level(log::Level::Trace).unwrap();
}
