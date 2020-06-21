#![no_std]
#![allow(unused)]

mod types;

pub use types::{
    CryptoError, Ctx, EnclaveBuffer, EnclaveError, HandleResult, InitResult, QueryResult,
    UserSpaceBuffer,
};

pub const ENCRYPTED_SEED_SIZE: usize = 48;
pub const PUBLIC_KEY_SIZE: usize = 32;
