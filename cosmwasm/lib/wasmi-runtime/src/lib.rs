#![crate_type = "staticlib"]
#![cfg_attr(not(target_env = "sgx"), no_std)]
#![cfg_attr(target_env = "sgx", feature(rustc_private))]

#[cfg(not(target_env = "sgx"))]
#[macro_use]
extern crate sgx_tstd as std;

extern crate sgx_rand;
extern crate sgx_tcrypto;
extern crate sgx_trts;
extern crate sgx_tse;
extern crate sgx_types;

mod attestation;
mod cert;
mod consts;
mod contract_operations;
mod encryption;
mod errors;
pub mod exports;
mod gas;
mod hex;
pub mod imports;
mod keys;
pub mod logger;
mod results;
mod runtime;
mod storage;
mod utils;

use ctor::*;
use log::LevelFilter;

use crate::logger::*;

static LOGGER: SimpleLogger = SimpleLogger;

#[ctor]
fn init_logger() {
    log::set_logger(&LOGGER).map(|()| log::set_max_level(LevelFilter::Trace));
}
