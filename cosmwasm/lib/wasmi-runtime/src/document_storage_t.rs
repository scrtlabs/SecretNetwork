// use sgx_tseal::SgxSealedData;
// use sgx_types::{sgx_attributes_t, sgx_sealed_data_t, sgx_status_t};
// use sgx_types::marker::ContiguousMemory;
// use std::io::{Read, Write};
// use std::path::PathBuf;
// use std::string::*;
// use std::untrusted::fs;
// use std::untrusted::fs::{File, remove_file};
//
// use common::errors_t::{EnclaveError, EnclaveError::*, EnclaveSystemError::*};
//
// pub const SEAL_LOG_SIZE: usize = 2048;
//
// #[derive(Copy, Clone, Default, Debug)]
// pub struct SealedDocumentStorage<T: ?Sized> {
//     pub version: u32,
//     pub data: T,
// }
//
// unsafe impl<T> ContiguousMemory for SealedDocumentStorage<T> {}
//
// impl<T> SealedDocumentStorage<T> where
//     T: Copy {
//     /// Safe seal
//     /// param: the_data : clear text to be sealed
//     /// param: sealed_log_out : the output of the sealed data
//     ///
//     /// The flags are from here: https://github.com/intel/linux-sgx/blob/master/common/inc/sgx_attributes.h#L38
//     /// additional is a part of AES-GCM that you can authenticate data with the MAC without encrypting it.
//     pub fn seal(&self, sealed_log_out: &mut [u8; SEAL_LOG_SIZE]) -> Result<(), EnclaveError> {
//         let additional: [u8; 0] = [0_u8; 0];
//         let attribute_mask = sgx_attributes_t { flags: 0xffff_ffff_ffff_fff3, xfrm: 0 };
//         let sealed_data = SgxSealedData::<Self>::seal_data_ex(
//             sgx_types::SGX_KEYPOLICY_MRSIGNER, //key policy
//             attribute_mask,
//             0, //misc mask
//             &additional,
//             &self,
//         )?;
//         let sealed_log = sealed_log_out.as_mut_ptr();
//         let sealed_log_size: usize = SEAL_LOG_SIZE;
//         to_sealed_log(&sealed_data, sealed_log, sealed_log_size as u32);
//         Ok(())
//     }
//
//     /// Unseal sealed log
//     /// param: sealed_log_in : the encrypted blob
//     pub fn unseal(sealed_log_in: &mut [u8]) -> Result<Option<Self>, EnclaveError> {
//         let sealed_log_size: usize = SEAL_LOG_SIZE;
//         let sealed_log = sealed_log_in.as_mut_ptr();
//         let sealed_data = match from_sealed_log::<Self>(sealed_log, sealed_log_size as u32) {
//             Some(data) => data,
//             None => {
//                 return Err(SystemError(OcallError { command: "unseal".to_string(), err: "No data in sealed log".to_string() }));
//             }
//         };
//         let unsealed_result = sealed_data.unseal_data();
//         match unsealed_result {
//             Ok(unsealed_data) => {
//                 let udata = unsealed_data.get_decrypt_txt();
//                 Ok(Some(*udata))
//             }
//             Err(err) => {
//                 // TODO: Handle this. It can causes panic in Simulation Mode until deleting the file.
//                 if err != sgx_status_t::SGX_ERROR_MAC_MISMATCH {
//                     return Err(SystemError(OcallError { command: "unseal".to_string(), err: format!("{:?}", err) }));
//                 }
//                 Ok(None)
//             }
//         }
//     }
// }
//
// /// This casts sealed_log from *u8 to *sgx_sealed_data_t which aren't aligned the same way.
// fn to_sealed_log<T: Copy + ContiguousMemory>(sealed_data: &SgxSealedData<T>, sealed_log: *mut u8,
//                                              sealed_log_size: u32, ) -> Option<*mut sgx_sealed_data_t> {
//     unsafe { sealed_data.to_raw_sealed_data_t(sealed_log as *mut sgx_sealed_data_t, sealed_log_size) }
// }
//
// // This casts a *sgx_sealed_data_t to *u8 which aren't aligned the same way.
// fn from_sealed_log<'a, T: Copy + ContiguousMemory>(sealed_log: *mut u8, sealed_log_size: u32) -> Option<SgxSealedData<'a, T>> {
//     unsafe { SgxSealedData::<T>::from_raw_sealed_data_t(sealed_log as *mut sgx_sealed_data_t, sealed_log_size) }
// }
//
// /// Save new sealed document
// pub fn save_sealed_document(path: &PathBuf, sealed_document: &[u8]) -> Result<(), EnclaveError> {
//     // TODO: handle error
//     let mut file = match File::create(path) {
//         Ok(opt) => opt,
//         Err(err) => {
//             return Err(SystemError(OcallError { command: "save_sealed_document".to_string(), err: format!("{:?}", err) }));
//         }
//     };
//     match file.write_all(&sealed_document) {
//         Ok(_) => println!("Sealed document: {:?} written successfully.", path),
//         Err(err) => {
//             return Err(SystemError(OcallError { command: "sealed_document".to_string(), err: format!("{:?}", err) }));
//         }
//     }
//     Ok(())
// }
//
// /// Check if sealed document exists
// pub fn is_document(path: &PathBuf) -> bool {
//     match fs::metadata(path) {
//         Ok(metadata) => metadata.is_file(),
//         Err(_) => false,
//     }
// }
//
// /// Load bytes of a sealed document in the provided mutable byte array
// pub fn load_sealed_document(path: &PathBuf, sealed_document: &mut [u8]) -> Result<(), EnclaveError> {
//     let mut file = match File::open(path) {
//         Ok(opt) => opt,
//         Err(err) => {
//             return Err(SystemError(OcallError { command: "load_sealed_document".to_string(), err: format!("{:?}", err) }));
//         }
//     };
//     match file.read(sealed_document) {
//         Ok(_) => println!("Sealed document: {:?} loaded successfully.", path),
//         Err(err) => {
//             return Err(SystemError(OcallError { command: "load_sealed_document".to_string(), err: format!("{:?}", err) }));
//         }
//     };
//     Ok(())
// }
//
// //#[cfg(debug_assertions)]
// pub mod tests {
//     use super::*;
//
// //use std::untrusted::fs::*;
//
//     /* Test functions */
//     pub fn test_document_sealing_storage() {
//         // generate mock data
//         let doc: SealedDocumentStorage<[u8; 32]> = SealedDocumentStorage {
//             version: 0x1234,
//             data: [b'i'; 32],
//         };
//         // seal data
//         let mut sealed_log_in: [u8; SEAL_LOG_SIZE] = [0; SEAL_LOG_SIZE];
//         doc.seal(&mut sealed_log_in).expect("Unable to seal document");
//         // save sealed_log to file
//         let p = PathBuf::from("seal_test.sealed");
//         save_sealed_document(&p, &sealed_log_in).expect("Unable to save sealed document");
//         // load sealed_log from file
//         let mut sealed_log_out: [u8; SEAL_LOG_SIZE] = [0; SEAL_LOG_SIZE];
//         load_sealed_document(&p, &mut sealed_log_out).expect("Unable to load sealed document");
//         // unseal data
//         let unsealed_doc = SealedDocumentStorage::<[u8; 32]>::unseal(&mut sealed_log_out).expect("Unable to unseal document").unwrap();
//         // compare data
//         assert_eq!(doc.data, unsealed_doc.data);
//         // delete the file
//         let f = remove_file(&p);
//         assert!(f.is_ok());
//     }
// }
