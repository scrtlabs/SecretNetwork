use log::trace;
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
        trace!("************ Cache insert ***********");

        self.0.insert(k.to_vec(), v.to_vec())
    }

    pub fn write_cache_only(&mut self, k: &[u8], v: &[u8]) -> Option<Vec<u8>> {
        trace!("************ Cache insert ***********");

        self.1.insert(k.to_vec(), v.to_vec())
    }
    pub fn read(&self, k: &[u8]) -> Option<Vec<u8>> {
        // first to to read from the writeable cache - this will be more updated
        let x = self.0.get(k);
        if x.is_some() {
            trace!("************ Cache hit ***********");

            return Some(x.unwrap().clone());
        }

        // if no hit in the writeable cache, try the readable one
        let x = self.1.get(k);
        if x.is_some() {
            trace!("************ Cache hit from RO cache ***********");

            return Some(x.unwrap().clone());
        }

        trace!("************ Cache miss ***********");

        return None;
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

        let items: Vec<(Vec<u8>, Vec<u8>)> = self.0.drain_filter(|_k, _v| true == true).collect();

        self.1.clear();

        items
    }
}
