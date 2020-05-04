// Functions re:node registration will be implemented here
use sgx_types::{sgx_status_t, SgxError, SgxResult};
use hex::decode;
//use crate::document_storage_t::{load_sealed_document, save_sealed_document, SealedDocumentStorage};
//use std::path::PathBuf;
//use crate::errors::{WasmEngineError};
// use serde_json::{from_value, Error, Value};
// use serde::{Serialize, Deserialize};

// #[derive(Debug, PartialEq, Clone, Serialize, Deserialize, Default)]
// pub struct Storage {
//     pub json: Value
// }
//
// impl Storage {
//     pub fn new() -> Storage {
//         let json = serde_json::from_str("{}").unwrap();
//         Storage { json }
//     }
// }
//
// pub trait IOInterface<E, U> {
//     fn read_key<T>(&self, key: &str) -> Result<T, Error> where for<'de> T: Deserialize<'de>;
//     fn write_key(&mut self, key: &str, value: &Value) -> Result<(), E>;
//     fn remove_key(&mut self, key: &str);
// }
//
// impl IOInterface<WasmEngineError, u8> for Storage {
//     fn read_key<T>(&self, key: &str) -> Result<T, Error>
//         where for<'de> T: Deserialize<'de> {
//         from_value(self.json[key].clone())
//     }
//
//     fn write_key(&mut self, key: &str, value: &Value) -> Result<(), WasmEngineError> {
//         self.json[key] = value.clone();
//         Ok(())
//     }
//
//     fn remove_key(&mut self, key: &str) {
//         if let Some(ref mut v) = self.json.as_object_mut() {
//             v.remove(key);
//         }
//     }
// }

// #[derive(Debug, PartialEq, Clone, Serialize, Deserialize)]
// pub struct EncryptedStorage {
//     pub encrypted: String,
// }
// //
// impl<'a> Encryption<&'a Key, WasmEngineError, EncryptedStorage, [u8; 12]> for Storage {
//     fn encrypt(self, key: &Key, _iv: Option<[u8; 12]>) -> Result<EncryptedContractState<u8>, WasmEngineError> {
//         let serialized = serde_json::to_string(&self).unwrap();
//         let enc = symmetric::encrypt_with_nonce(&serialized, key, _iv)?;
//         Ok(EncryptedStorage { encrypted: enc })
//     }
//
//     fn decrypt(enc: EncryptedStorage, key: &StateKey) -> Result<Storage, WasmEngineError> {
//         let dec = symmetric::decrypt(&enc.json, key)?;
//         let deserialized: Storage = serde_json::from_str(&dec).unwrap();
//         Ok(deserialized)
//     }
// }

// #[derive(Copy, Clone, Default, Debug)]
// pub struct SealedStorage<K, V> {
//     pub storage: SealedDocumentStorage<IndexMap<K, V>>,
//     savepath: PathBuf,
//
// }
//
// impl<K, V> SealedStorage<K, V> {
//     pub fn get(self, key: K, value: V) {
//         let doc = load_sealed_document(self.savepath)
//         self.storage.unseal();
//     }
//
//     pub fn
// }



/*
 *
 */
pub fn init_seed(
    pk: &[u8; 64],  // public key
    encrypted_key: &[u8; 32], // encrypted key
) -> Result<sgx_status_t, SgxError> {
    println!("yo yo yo");

    //
    println!("key: 0x{:?}", encrypted_key);

    return Ok(sgx_status_t::SGX_SUCCESS);
}