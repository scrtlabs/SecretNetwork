//
// pub fn retry_quote(eid: sgx_enclave_id_t, spid: &str, times: usize) -> Result<String, Error> {
//     let mut quote = String::new();
//     for _ in 0..times {
//         quote = match produce_quote(eid, spid) {
//             Ok(q) => q,
//             Err(e) => {
//                 println!("problem with quote, trying again: {:?}", e);
//                 continue;
//             }
//         };
//
//         if !quote.chars().all(|cur_c| cur_c == 'A') {
//             return Ok(quote);
//         } else {
//             sleep(time::Duration::new(5, 0));
//         }
//     }
//     Err(Error::SdkErr { inner: sgx_status_t::SGX_ERROR_SERVICE_UNAVAILABLE }.into())
// }
