// similar trick to get the IDE to use sgx_tstd even when it doesn't know we're targeting SGX
#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;
// This annotation is here to trick the IDE into ignoring the extern crate, and instead pull in sgx_types from our
// Cargo.toml. By importing sgx_types using `extern crate` but without letting it resolve in Cargo.toml when compiling
// to SGX, we make the compiler pull it in from the target root, which contains the sgx_types listed in Xargo.toml.
// This in turn silences errors about using the same types from two versions of the same crate.
// (go ahead, try to remove this block and change the Cargo.toml import to a normal one)
#[cfg(target_env = "sgx")]
extern crate sgx_types;

use ctor::*;
use log::LevelFilter;

use crate::logger::*;

mod macros;

pub mod exports;
pub mod imports;
pub mod logger;
mod oom_handler;
pub mod registration;

mod consts;
mod wasm;
//mod contract_operations;
//mod contract_validation;
mod cosmwasm;
mod crypto;
// mod errors;
// mod gas;
mod results;
//mod runtime;
mod storage;
mod utils;

mod tests;

static LOGGER: SimpleLogger = SimpleLogger;

#[cfg(all(not(feature = "production"), feature = "SGX_MODE_HW"))]
#[ctor]
fn init_logger() {
    log::set_logger(&LOGGER)
        .map(|()| log::set_max_level(LevelFilter::Info))
        .unwrap();
}

#[cfg(all(feature = "production", feature = "SGX_MODE_HW"))]
#[ctor]
fn init_logger() {
    log::set_logger(&LOGGER)
        .map(|()| log::set_max_level(LevelFilter::Warn))
        .unwrap();
}

#[cfg(not(feature = "SGX_MODE_HW"))]
#[ctor]
fn init_logger() {
    log::set_logger(&LOGGER)
        .map(|()| log::set_max_level(LevelFilter::Trace))
        .unwrap();
}
