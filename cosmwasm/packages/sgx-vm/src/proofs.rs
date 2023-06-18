use sgx_types::*;

use log::{debug, error, warn};

use crate::enclave::ENCLAVE_DOORBELL;

extern "C" {
    pub fn ecall_submit_store_roots(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        in_roots: *const u8,
        in_roots_len: u32,
        in_compute_root: *const u8,
        in_compute_root_len: u32,
    ) -> sgx_status_t;
}

pub fn untrusted_submit_store_roots(roots: &[u8], compute_root: &[u8]) -> SgxResult<()> {
    debug!("Hello from just before - untrusted_submit_store_roots");

    const RETRY_LIMIT: i32 = 3;

    let mut retry_count = 0;

    // this is here so we can
    loop {
        let (retval, status) = submit_store_roots_impl(roots, compute_root)?;
        if status != sgx_status_t::SGX_SUCCESS {
            return Err(status);
        } else if retval != sgx_status_t::SGX_SUCCESS {
            if retval == sgx_status_t::SGX_ERROR_FILE_RECOVERY_NEEDED {
                warn!(
                    "Validator set read by enclave was mismatched with current height.. retrying"
                );
                // retry with
                std::thread::sleep(std::time::Duration::from_millis(500));
                retry_count += 1;

                if retry_count == RETRY_LIMIT {
                    error!("Validator timed out while waiting for correct validator set");
                    return Err(retval);
                }
            } else {
                return Err(retval);
            }
        } else {
            return Ok(());
        }
    }
}

fn submit_store_roots_impl(
    roots: &[u8],
    compute_root: &[u8],
) -> SgxResult<(sgx_status_t, sgx_status_t)> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    let eid = enclave.geteid();
    let mut retval = sgx_status_t::SGX_SUCCESS;

    // let status = unsafe { ecall_get_encrypted_seed(eid, &mut retval, cert, cert_len, & mut seed) };
    let status = unsafe {
        ecall_submit_store_roots(
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
