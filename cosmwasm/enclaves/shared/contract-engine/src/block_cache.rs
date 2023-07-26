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

#[cfg(feature = "test")]
pub mod tests {
    use crate::block_cache::CacheMap;
    use crate::contract_validation::ContractKey;
    use enclave_utils::kv_cache::KvCache;

    pub fn test_insert_into_cachemap() {
        let mut cm = CacheMap::default();
        let mut kv_cache = KvCache::new();

        let key0: ContractKey = [0u8; 64];

        kv_cache.store_in_ro_cache(b"a", b"b");
        kv_cache.store_in_ro_cache(b"x", b"y");

        cm.insert(key0, kv_cache);

        let kv2 = cm.get(&key0);

        assert!(kv2.is_some());

        let value = kv2.unwrap().read(b"a");

        assert!(value.is_some());

        assert_eq!(value.unwrap().as_slice(), b"b");
    }

    pub fn test_clear_cachemap() {
        let mut cm = CacheMap::default();
        let mut kv_cache = KvCache::new();

        let key0: ContractKey = [0u8; 64];

        kv_cache.store_in_ro_cache(b"a", b"b");
        kv_cache.store_in_ro_cache(b"x", b"y");

        cm.insert(key0, kv_cache);

        let kv2 = cm.get(&key0);

        assert!(kv2.is_some());

        let value = kv2.unwrap().read(b"a");

        assert!(value.is_some());

        assert_eq!(value.unwrap().as_slice(), b"b");

        cm.clear();

        let kv2 = cm.get(&key0);

        assert!(kv2.is_none())
    }

    pub fn test_merge_into_cachemap() {
        let mut cm = CacheMap::default();
        let mut kv_cache = KvCache::new();
        let mut kv_cache2 = KvCache::new();

        let key0: ContractKey = [0u8; 64];

        kv_cache.write(b"test", b"this");
        kv_cache.write(b"toast", b"bread");
        kv_cache.store_in_ro_cache(b"a", b"b");
        kv_cache.store_in_ro_cache(b"1", b"2");
        kv_cache2.write(b"test", b"other");
        kv_cache2.write(b"food", b"pizza");
        kv_cache2.store_in_ro_cache(b"a", b"c");
        kv_cache2.store_in_ro_cache(b"xy", b"zw");

        cm.insert(key0.clone(), kv_cache);
        cm.insert(key0, kv_cache2);

        let kv2 = cm.get(&key0);

        assert!(kv2.is_some());

        let value = kv2.unwrap().read(b"a");

        assert!(value.is_some());

        assert_eq!(value.unwrap().as_slice(), b"c");

        let value = kv2.unwrap().read(b"1");

        assert!(value.is_some());

        assert_eq!(value.unwrap().as_slice(), b"2");

        let value = kv2.unwrap().read(b"xy");

        assert!(value.is_some());

        assert_eq!(value.unwrap().as_slice(), b"zw");

        let value = kv2.unwrap().read(b"test");

        assert!(value.is_some());

        assert_eq!(value.unwrap().as_slice(), b"other");

        let value = kv2.unwrap().read(b"food");

        assert!(value.is_some());

        assert_eq!(value.unwrap().as_slice(), b"pizza");

        let value = kv2.unwrap().read(b"toast");

        assert!(value.is_some());

        assert_eq!(value.unwrap().as_slice(), b"bread");
    }
}
