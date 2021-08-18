#![no_std]
#![allow(unused)]

mod types;

pub use types::{
    Ctx, EnclaveBuffer, EnclaveError, HandleResult, HealthCheckResult, InitResult, NodeAuthResult,
    OcallReturn, QueryResult, RuntimeConfiguration, UntrustedVmError, UserSpaceBuffer,
};

pub const ENCRYPTED_SEED_SIZE: usize = 48;
pub const PUBLIC_KEY_SIZE: usize = 32;
