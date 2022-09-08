// use serde::Serialize;
use std::collections::BTreeMap;

// #[derive(Serialize, Clone, Debug, PartialEq)]
// pub struct MultiKv {
//     keys: Vec<Vec<u8>>,
//     values: Vec<Vec<u8>>,
// }

pub struct KvCache(BTreeMap<Vec<u8>, Vec<u8>>, BTreeMap<Vec<u8>, Vec<u8>>);

impl KvCache {
    pub fn new() -> Self {
        Self {
            // 0 is cache that we need to write at the end of the tx (new keys)
            0: Default::default(),
            // 1 is just used for a read cache (for reading multiple keys in a row)
            1: Default::default(),
        }
    }

    pub fn write(&mut self, k: &[u8], v: &[u8]) -> Option<Vec<u8>> {
        //trace!("************ Cache insert ***********");

        self.0.insert(k.to_vec(), v.to_vec())
    }

    pub fn write_cache_only(&mut self, k: &[u8], v: &[u8]) -> Option<Vec<u8>> {
        //trace!("************ Cache insert ***********");

        self.1.insert(k.to_vec(), v.to_vec())
    }
    pub fn read(&self, key: &[u8]) -> Option<Vec<u8>> {
        // first to to read from the writeable cache - this will be more updated
        if let Some(value) = self.0.get(key) {
            Some(value.clone())
        }
        // if no hit in the writeable cache, try the readable one
        else if let Some(value) = self.1.get(key) {
            Some(value.clone())
        } else {
            // trace!("************ Cache miss ***********");
            None
        }
    }

    pub fn flush(&mut self) -> Vec<(Vec<u8>, Vec<u8>)> {
        // let mut keys = vec![];
        // let mut values = vec![];
        //
        // for (k, v) in self.0 {
        //     keys.push(k);
        //     values.push(v);
        // }
        //
        // &self.0.clear();
        //
        // MultiKv { keys, values }

        let items: Vec<(Vec<u8>, Vec<u8>)> = self.0.drain_filter(|_k, _v| true).collect();

        self.1.clear();

        items
    }
}

impl Default for KvCache {
    fn default() -> Self {
        Self::new()
    }
}
