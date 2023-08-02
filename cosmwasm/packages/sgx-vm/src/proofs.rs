use sgx_types::*;

use enclave_ffi_types::SdkBeginBlockerResult;
use log::{debug, error, warn};

use crate::enclave::ENCLAVE_DOORBELL;

const RETRY_LIMIT: i32 = 3;

extern "C" {
    pub fn ecall_app_begin_blocker(
        eid: sgx_enclave_id_t,
        retval: *mut SdkBeginBlockerResult,
        in_roots: *const u8,
        in_roots_len: u32,
        in_compute_root: *const u8,
        in_compute_root_len: u32,
    ) -> sgx_status_t;
}

pub fn untrusted_submit_store_roots(roots: &[u8], compute_root: &[u8]) -> SgxResult<()> {
    debug!("Hello from just before - untrusted_submit_store_roots");

    for _ in 0..RETRY_LIMIT {
        let (retval, status) = submit_store_roots_impl(roots, compute_root)?;

        if status != sgx_status_t::SGX_SUCCESS {
            return Err(status);
        } else if retval == SdkBeginBlockerResult::Success {
            return Ok(());
        } else if retval == SdkBeginBlockerResult::Failure {
            warn!("Validator set read by enclave was mismatched with current height.. retrying");
            std::thread::sleep(std::time::Duration::from_millis(500));
        } else {
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    }

    error!("Validator timed out while waiting for correct validator set");
    Err(sgx_status_t::SGX_ERROR_UNEXPECTED) // or any appropriate error
}

fn submit_store_roots_impl(
    roots: &[u8],
    compute_root: &[u8],
) -> SgxResult<(SdkBeginBlockerResult, sgx_status_t)> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    let eid = enclave.geteid();
    let mut retval = SdkBeginBlockerResult::Success;

    // let status = unsafe { ecall_get_encrypted_seed(eid, &mut retval, cert, cert_len, & mut seed) };
    let status = unsafe {
        ecall_app_begin_blocker(
            eid,
            &mut retval,
            roots.as_ptr(),
            roots.len() as u32,
            compute_root.as_ptr(),
            compute_root.len() as u32,
        )
    };
    Ok((retval, status))
}
