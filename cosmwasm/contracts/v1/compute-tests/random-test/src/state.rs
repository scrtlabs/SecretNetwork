use cosmwasm_std::{Storage};
use cosmwasm_storage::{
    singleton, singleton_read, ReadonlySingleton, Singleton,
};

pub const EXPIRATION_KEY: &[u8] = b"expire";

pub fn expiration(storage: &mut dyn Storage) -> Singleton<u64> {
    singleton(storage, EXPIRATION_KEY)
}

pub fn expiration_read(storage: &dyn Storage) -> ReadonlySingleton<u64> {
    singleton_read(storage, EXPIRATION_KEY)
}

