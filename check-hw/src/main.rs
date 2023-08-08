mod enclave;
mod enclave_api;
mod types;

use clap::App;
use lazy_static::lazy_static;
use sgx_types::sgx_status_t;

use crate::{enclave_api::ecall_check_patch_level, types::EnclaveDoorbell};

use enclave_ffi_types::NodeAuthResult;

const ENCLAVE_FILE_TESTNET: &str = "check_hw_testnet_enclave.so";
const ENCLAVE_FILE_MAINNET: &str = "check_hw_enclave.so";
const TCS_NUM: u8 = 1;

lazy_static! {
    static ref ENCLAVE_DOORBELL: EnclaveDoorbell = {
        let is_testnet = std::env::args().any(|arg| arg == "--testnet");
        let enclave_file = if is_testnet {
            ENCLAVE_FILE_TESTNET
        } else {
            ENCLAVE_FILE_MAINNET
        };
        EnclaveDoorbell::new(enclave_file, TCS_NUM)
    };
}

fn main() {
    let matches = App::new("Check HW")
        .version("1.0")
        .arg("--testnet 'Run in testnet mode'")
        .get_matches();

    let is_testnet = matches.is_present("testnet");

    println!("Creating enclave instance..");

    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY);

    if let Err(e) = enclave_access_token {
        println!(
            "Failed to get enclave access token: {:?} (is enclave currently running or busy?)",
            e
        );
        return;
    }

    let enclave = enclave_access_token.unwrap().enclave;

    if let Err(e) = enclave {
        println!("Failed to start enclave: {:?}", e);
        return;
    }

    let api_key_bytes = if is_testnet {
        include_bytes!("../../ias_keys/develop/api_key.txt")
    } else {
        include_bytes!("../../ias_keys/production/api_key.txt")
    };

    let eid = enclave.unwrap().geteid();
    let mut retval = NodeAuthResult::Success;
    let status = unsafe {
        ecall_check_patch_level(
            eid,
            &mut retval,
            api_key_bytes.as_ptr(),
            api_key_bytes.len() as u32,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        println!(
            "Failed to run hardware verification test (is the correct enclave in the correct path?)"
        );
        return;
    }

    if retval != NodeAuthResult::Success {
        println!("Failed to verify platform. Please see errors above for more info on what needs to be fixed before you can run a mainnet node. \n\
        If you require assistance or more information, please contact us on Discord or Telegram. In addition, you may use the documentation available at \
        https://docs.scrt.network
        ");
        return;
    } else {
        println!("Platform verification successful! You are able to run a mainnet Secret node")
    }
}
