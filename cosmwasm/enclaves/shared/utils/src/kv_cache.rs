// use serde::Serialize;
use std::collections::BTreeMap;

const PSEUDO_GAS_STORE_PER_BYTE: u64 = 5_000;

#[derive(Default, Clone)]
pub struct KvCache {
    writeable_cache: BTreeMap<Vec<u8>, Vec<u8>>,
    readable_cache: BTreeMap<Vec<u8>, Vec<u8>>,
    /// used to track pseudo gas for inserts - this helps avoid situations where the write cache gets
    /// so big that the flush to chain state goes OOM instead of out of gas
    gas_tracker: u64,
}

impl KvCache {
    pub fn new() -> Self {
        Self::default()
    }

    /// this is used to store data that needs to be written to chain state at the end of execution
    pub fn write(&mut self, k: &[u8], v: &[u8]) -> (Option<Vec<u8>>, u64) {
        self.gas_tracker += PSEUDO_GAS_STORE_PER_BYTE * v.len() as u64;

        (
            self.writeable_cache.insert(k.to_vec(), v.to_vec()),
            PSEUDO_GAS_STORE_PER_BYTE * v.len() as u64,
        )
    }

    /// this is used to store data that is read often, but not modified - for example contract settings
    pub fn store_in_ro_cache(&mut self, k: &[u8], v: &[u8]) -> Option<Vec<u8>> {
        self.readable_cache.insert(k.to_vec(), v.to_vec())
    }
    pub fn read(&self, key: &[u8]) -> Option<Vec<u8>> {
        // first to to read from the writeable cache - this will be more updated
        if let Some(value) = self.writeable_cache.get(key) {
            Some(value.clone())
        }
        // if no hit in the writeable cache, try the readable one
        else {
            self.readable_cache.get(key).cloned()
        }
    }

    pub fn remove(&mut self, key: &[u8]) {
        self.writeable_cache.remove(key);
        self.readable_cache.remove(key);
    }

    pub fn drain_gas_tracker(&mut self) -> u64 {
        let gas_used = self.gas_tracker;
        self.gas_tracker = 0;
        gas_used
    }

    pub fn flush(&mut self) -> Vec<(Vec<u8>, Vec<u8>)> {
        let items: Vec<(Vec<u8>, Vec<u8>)> =
            self.writeable_cache.drain_filter(|_k, _v| true).collect();

        self.readable_cache.clear();

        items
    }
}
