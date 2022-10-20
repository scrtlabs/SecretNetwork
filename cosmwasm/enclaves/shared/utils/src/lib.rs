#![cfg_attr(not(target_env = "sgx"), no_std)]
#![cfg_attr(target_env = "sgx", feature(rustc_private))]

extern crate sgx_trts;
extern crate sgx_types;
#[cfg(not(target_env = "sgx"))]
#[macro_use]
extern crate sgx_tstd as std;

pub mod logger;
pub mod macros;
pub mod oom_handler;
pub mod pointers;
pub mod recursion_depth;
mod results;
pub mod storage;
