mod enclave;
mod enclave_api;
mod types;

use lazy_static::lazy_static;
use sgx_types::sgx_status_t;

use crate::{enclave_api::ecall_get_attestation_report, types::EnclaveDoorbell};

static ENCLAVE_FILE: &str = "check_hw_enclave.so";
const TCS_NUM: u8 = 8;

lazy_static! {
    static ref ENCLAVE_DOORBELL: EnclaveDoorbell = EnclaveDoorbell::new(ENCLAVE_FILE, TCS_NUM);
}

fn main() -> Result<(), sgx_status_t> {
    println!("Creating enclave instance..");

    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;

    let enclave = enclave_access_token.enclave?;

    let api_key_bytes = include_bytes!("../../ias_keys/production/api_key.txt");

    let eid = enclave.geteid();
    let mut retval = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_get_attestation_report(
            eid,
            &mut retval,
            api_key_bytes.as_ptr(),
            api_key_bytes.len() as u32,
            1, // boolean
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        println!("could not generate attestation report");
        return Err(status);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        println!("could not generate attestation report");
        return Err(retval);
    }

    Ok(())
}
