use crate::contract_validation::ContractKey;
use alloc::collections::btree_map::Entry;
use alloc::collections::BTreeMap;
use enclave_utils::kv_cache::KvCache;
use std::sync::SgxMutex;

lazy_static::lazy_static! {
    pub static ref BLOCK_CACHE: SgxMutex<CacheMap> =
        SgxMutex::new(CacheMap::default());
}

#[derive(Default)]
pub struct CacheMap(pub BTreeMap<ContractKey, KvCache>);

impl CacheMap {
    pub fn insert(&mut self, key: ContractKey, kv_cache: KvCache) {
        match self.0.entry(key) {
            Entry::Occupied(mut entry) => {
                // If the key is already present in the map, merge the old and new KvCache
                entry.get_mut().merge(kv_cache);
            }
            Entry::Vacant(entry) => {
                // If the key is not present in the map, insert the new KvCache
                entry.insert(kv_cache);
            }
        }
    }
    pub fn get(&self, key: &ContractKey) -> Option<&KvCache> {
        self.0.get(key)
    }

    pub fn get_or_insert(&mut self, key: ContractKey) -> &mut KvCache {
        self.0.entry(key).or_insert_with(KvCache::new)
    }

    pub fn clear(&mut self) {
        self.0.clear();
    }
}

pub fn clear_block_cache() {
    let mut cache = BLOCK_CACHE.lock().unwrap();

    cache.clear();
}
