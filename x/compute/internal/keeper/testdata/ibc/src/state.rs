use cosmwasm_std::{Storage};
use cosmwasm_storage::{
    singleton, singleton_read, ReadonlySingleton, Singleton,
};

pub const COUNT_KEY: &[u8] = b"count";

pub fn count(storage: &mut dyn Storage) -> Singleton<u64> {
    singleton(storage, COUNT_KEY)
}

pub fn count_read(storage: &dyn Storage) -> ReadonlySingleton<u64> {
    singleton_read(storage, COUNT_KEY)
}


