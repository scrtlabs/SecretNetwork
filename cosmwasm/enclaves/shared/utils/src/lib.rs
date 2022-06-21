#![feature(try_reserve)]
#![feature(alloc_error_hook)]
#![feature(backtrace)]
#![allow(non_camel_case_types)]
// #[cfg(not(target_env = "sgx"))]
// extern crate sgx_tstd as std;
//
// extern crate sgx_types;

pub mod logger;
pub mod macros;
pub mod oom_handler;
pub mod pointers;
pub mod recursion_depth;
mod results;
pub use results::sgx_status_t;
pub mod storage;
