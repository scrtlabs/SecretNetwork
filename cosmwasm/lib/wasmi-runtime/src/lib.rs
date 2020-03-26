#![cfg_attr(sgx, no_std)]

// #[cfg(sgx)]
// use sgx_tstd as std;

mod contract_operations;
pub mod exports;
pub mod imports;
mod results;
