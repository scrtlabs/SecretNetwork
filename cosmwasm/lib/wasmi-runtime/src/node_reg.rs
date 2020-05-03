// Functions re:node registration will be implemented here
use sgx_types::{sgx_status_t, SgxError, SgxResult};

/*
 *
 */
pub fn init_seed(
    pk: &[u8; 64],  // public key
    encrypted_key: &[u8; 32], // encrypted key
) -> Result<sgx_status_t, SgxError> {
    println!("yo yo yo");
    return Ok(sgx_status_t::SGX_SUCCESS);
}