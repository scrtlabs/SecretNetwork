// Functions re:node registration will be implemented here
use sgx_types::{sgx_status_t, SgxError, SgxResult};


/*
 *
 */
pub fn init_seed(
    public_key: &[u8],
    encrypted_seed: &[u8]
) -> sgx_status_t {

    println!("yo yo yo");
    println!("key: 0x{:?}", encrypted_seed);



    return sgx_status_t::SGX_SUCCESS;
}