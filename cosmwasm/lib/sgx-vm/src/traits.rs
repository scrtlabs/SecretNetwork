//! This module exists because we need to modify the `Storage` trait, defined originally in `cosmwasm`,
//! in order to allow downcasting trait objects of this trait. Since the implementations of this trait outside and
//! inside the wasm runtime are completely separate, we can replace the original trait for this one in the code of
//! cosmwasm-sgx-vm.
use cosmwasm::traits::Api;
use downcast_rs::{impl_downcast, Downcast};

// Extern holds all external dependencies of the contract,
// designed to allow easy dependency injection at runtime
#[derive(Clone)]
pub struct Extern<S: Storage, A: Api> {
    pub storage: S,
    pub api: A,
}

// ReadonlyStorage is access to the contracts persistent data store
//pub trait ReadonlyStorage: Clone {
pub trait ReadonlyStorage: Downcast {
    fn get(&self, key: &[u8]) -> Option<Vec<u8>>;
}

// Storage extends ReadonlyStorage to give mutable access
pub trait Storage: ReadonlyStorage {
    fn set(&mut self, key: &[u8], value: &[u8]);
}
impl_downcast!(Storage);
