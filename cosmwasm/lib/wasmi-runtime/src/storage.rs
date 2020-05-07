// use std::io::{Error, Write};
// use std::sgxfs::SgxFile;
//
// pub fn seal(bytes: &[u8], filepath: &str) -> Result<(), Error> {
//     // Files are automatically closed when they go out of scope.
//     let mut file = SgxFile::create(filepath)?;
//
//     file.write_all(bytes)
// }
//
// // fn _write<F: Write>(bytes: &[u8], mut file: F) -> Result<sgx_status_t, ()> {
// //     file.write_all(bytes);
//
// //     Ok(sgx_status_t::SGX_SUCCESS)
// // }
