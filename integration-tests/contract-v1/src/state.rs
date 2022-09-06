use cosmwasm_std::{Storage};
use cosmwasm_storage::{
    singleton, singleton_read, ReadonlySingleton, Singleton,
};

pub const CHANNEL_KEY: &[u8] = b"channel";
pub const ACK_KEY: &[u8] = b"ack";
pub const RECEIVE_KEY: &[u8] = b"receive";

pub fn channel_store(storage: &mut dyn Storage) -> Singleton<String> {
    singleton(storage, CHANNEL_KEY)
}

pub fn channel_store_read(storage: &dyn Storage) -> ReadonlySingleton<String> {
    singleton_read(storage, CHANNEL_KEY)
}

pub fn ack_store(storage: &mut dyn Storage) -> Singleton<String> {
    singleton(storage, ACK_KEY)
}

pub fn ack_store_read(storage: &dyn Storage) -> ReadonlySingleton<String> {
    singleton_read(storage, ACK_KEY)
}

pub fn receive_store(storage: &mut dyn Storage) -> Singleton<String> {
    singleton(storage, RECEIVE_KEY)
}

pub fn receive_store_read(storage: &dyn Storage) -> ReadonlySingleton<String> {
    singleton_read(storage, RECEIVE_KEY)
}
