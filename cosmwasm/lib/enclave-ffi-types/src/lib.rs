#![no_std]

mod types;

pub use types::{
    CryptoError, Ctx, EnclaveBuffer, EnclaveError, HandleResult, InitResult, QueryResult,
    UserSpaceBuffer,
};

pub const ENCRYPTED_SEED_SIZE: usize = 48;
