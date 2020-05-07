#![crate_type = "staticlib"]

#![cfg_attr(not(target_env = "sgx"), no_std)]
#![cfg_attr(target_env = "sgx", feature(rustc_private))]

#[cfg(not(target_env = "sgx"))]
#[macro_use]
extern crate sgx_tstd as std;

// extern crate base64;
// extern crate bit_vec;
// extern crate chrono;
// extern crate httparse;
// extern crate itertools;
// extern crate num_bigint;
// extern crate rustls;
// extern crate webpki;
// extern crate webpki_roots;
// extern crate yasna;

extern crate sgx_types;
extern crate sgx_rand;
extern crate sgx_tcrypto;
extern crate sgx_tse;

mod hex;
mod quote;
mod contract_operations;
mod errors;
pub mod exports;
mod gas;
pub mod imports;
pub mod logger;
// mod node_reg;
mod results;
mod runtime;
// mod storage;

use ctor::*;
use log::LevelFilter;

use crate::logger::*;

static LOGGER: SimpleLogger = SimpleLogger;

#[ctor]
fn init_logger() {
    log::set_logger(&LOGGER).map(|()| log::set_max_level(LevelFilter::Trace));
}
