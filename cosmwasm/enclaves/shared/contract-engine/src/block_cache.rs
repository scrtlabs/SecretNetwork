use crate::contract_validation::ContractKey;
use alloc::collections::BTreeMap;
use enclave_utils::kv_cache::KvCache;
use std::sync::SgxMutex;

lazy_static::lazy_static! {
    pub static ref BLOCK_CACHE: SgxMutex<BTreeMap<ContractKey, KvCache>> =
        SgxMutex::new(BTreeMap::default());
}

pub fn clear_block_cache() {
    let mut cache = BLOCK_CACHE.lock().unwrap();

    cache.clear();
}
