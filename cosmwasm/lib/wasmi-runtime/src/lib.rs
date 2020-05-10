#![crate_type = "staticlib"]

#![cfg_attr(not(target_env = "sgx"), no_std)]
#![cfg_attr(target_env = "sgx", feature(rustc_private))]

#[cfg(not(target_env = "sgx"))]
#[macro_use]
extern crate sgx_tstd as std;

extern crate sgx_types;
extern crate sgx_rand;
extern crate sgx_tcrypto;
extern crate sgx_tse;
extern crate sgx_trts;

mod node_reg;
mod cert;
mod hex;
mod attestation;
mod contract_operations;
mod errors;
pub mod exports;
mod gas;
pub mod imports;
pub mod logger;
mod results;
mod runtime;
mod consts;
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
