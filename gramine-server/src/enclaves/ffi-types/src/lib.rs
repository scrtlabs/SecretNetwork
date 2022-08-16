#![no_std]
#![allow(unused)]

// pub mod errors;
mod error;
pub use error::sgx_status_t;
mod types;
pub use types::{
    Ctx, EnclaveBuffer, EnclaveError, HandleResult, HealthCheckResult, InitResult, NodeAuthResult,
    OcallReturn, QueryResult, RuntimeConfiguration, SgxError, SgxResult, UntrustedVmError,
    UserSpaceBuffer,
};

pub const ENCRYPTED_SEED_SIZE: usize = 48;
pub const PUBLIC_KEY_SIZE: usize = 32;
