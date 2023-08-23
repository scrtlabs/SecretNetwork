use sgx_types::{
    sgx_attributes_t, sgx_launch_token_t, sgx_misc_attribute_t, sgx_status_t, SgxResult,
};
use sgx_urts::SgxEnclave;
use std::path::Path;

pub fn init_enclave(enclave_file: &str, enclave_debug: i32) -> SgxResult<SgxEnclave> {
    let mut launch_token: sgx_launch_token_t = [0; 1024];
    let mut launch_token_updated: i32 = 0;
    // call sgx_create_enclave to initialize an enclave instance
    // Debug Support: set 2nd parameter to 1
    let debug: i32 = enclave_debug;
    let mut misc_attr = sgx_misc_attribute_t {
        secs_attr: sgx_attributes_t { flags: 0, xfrm: 0 },
        misc_select: 0,
    };

    // Search only in the current directory
    let enclave_file_path = Path::new(".").join(enclave_file);
    if !enclave_file_path.exists() {
        println!(
            "Cannot find the enclave file {:?}",
            enclave_file_path.to_str()
        );
        return Err(sgx_status_t::SGX_ERROR_INVALID_ENCLAVE);
    }

    SgxEnclave::create(
        enclave_file_path,
        debug,
        &mut launch_token,
        &mut launch_token_updated,
        &mut misc_attr,
    )
}
