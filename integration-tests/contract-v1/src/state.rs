use cosmwasm_std::{Storage};
use cosmwasm_storage::{
    singleton, singleton_read, ReadonlySingleton, Singleton,
};

pub const CHANNEL_KEY: &[u8] = b"channel";

pub fn channel_store(storage: &mut dyn Storage) -> Singleton<String> {
    singleton(storage, CHANNEL_KEY)
}

pub fn channel_store_read(storage: &dyn Storage) -> ReadonlySingleton<String> {
    singleton_read(storage, CHANNEL_KEY)
}
