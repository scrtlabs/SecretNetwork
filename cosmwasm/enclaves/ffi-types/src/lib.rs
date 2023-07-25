#![no_std]
#![allow(unused)]

mod types;

pub use types::{
    Ctx, EnclaveBuffer, EnclaveError, HandleResult, HealthCheckResult, InitResult, NodeAuthResult,
    OcallReturn, QueryResult, RuntimeConfiguration, SdkBeginBlockerResult, UntrustedVmError,
    UserSpaceBuffer,
};

// On input, the encrypted seed is expected to contain 3 values:
//  The first byte will be the size of the input (48/96)
//  The next 48 bytes are the first seed
//  The next 48 bytes represent an optional second seed
// On output (When authenticating a node or retreiving the seed) we ALWAYS return 96 bytes that represent both of the seeds (Without the size indicator)
pub const INPUT_ENCRYPTED_SEED_SIZE: u32 = 97;
pub const OUTPUT_ENCRYPTED_SEED_SIZE: u32 = 96;

pub const SINGLE_ENCRYPTED_SEED_SIZE: usize = 48;
pub const NEWLY_FORMED_SINGLE_ENCRYPTED_SEED_SIZE: usize = SINGLE_ENCRYPTED_SEED_SIZE + 1;
pub const NEWLY_FORMED_DOUBLE_ENCRYPTED_SEED_SIZE: usize = (2 * SINGLE_ENCRYPTED_SEED_SIZE) + 1;
pub const PUBLIC_KEY_SIZE: usize = 32;
