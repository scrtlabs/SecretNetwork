#![no_std]
#![allow(unused)]

mod types;

pub use types::{
    Ctx, EnclaveBuffer, EnclaveError, HandleResult, HealthCheckResult, InitResult, MigrateResult,
    NodeAuthResult, OcallReturn, QueryResult, RuntimeConfiguration, UntrustedVmError,
    UpdateAdminResult, UserSpaceBuffer,
};

pub const SINGLE_ENCRYPTED_SEED_SIZE: usize = 48;
pub const PUBLIC_KEY_SIZE: usize = 32;
