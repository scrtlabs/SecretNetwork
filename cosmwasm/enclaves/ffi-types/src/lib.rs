#![no_std]
#![allow(unused)]

mod types;

pub use types::{
    Ctx, EnclaveBuffer, EnclaveError, HandleResult, HealthCheckResult, InitResult, NodeAuthResult,
    OcallReturn, QueryResult, RuntimeConfiguration, UntrustedVmError, UserSpaceBuffer,
};

// 1 byte for length, 48 bytes for each potential encrypted seed
pub const INPUT_ENCRYPTED_SEED_SIZE: u32 = 97;
// 48 bytes for each potential encrypted seed
pub const OUTPUT_ENCRYPTED_SEED_SIZE: u32 = 96;

pub const SINGLE_ENCRYPTED_SEED_SIZE: usize = 48;
pub const NEWLY_FORMED_SINGLE_ENCRYPTED_SEED_SIZE: usize = SINGLE_ENCRYPTED_SEED_SIZE + 1;
pub const NEWLY_FORMED_DOUBLE_ENCRYPTED_SEED_SIZE: usize = (2 * SINGLE_ENCRYPTED_SEED_SIZE) + 1;
pub const PUBLIC_KEY_SIZE: usize = 32;
