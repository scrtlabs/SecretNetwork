use sgx_types::*;

use log::{debug, error, warn};

use crate::enclave::ENCLAVE_DOORBELL;

extern "C" {
    pub fn ecall_submit_block_signatures(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        in_header: *const u8,
        in_header_len: u32,
        in_commit: *const u8,
        in_commit_len: u32,
        in_txs: *const u8,
        in_txs_len: u32,
        in_encrypted_random: *const u8,
        in_encrypted_random_len: u32,
        decrypted_random: &mut [u8; 32],
        // in_validator_set: *const u8,
        // in_validator_set_len: u32,
        // in_next_validator_set: *const u8,
        // in_next_validator_set_len: u32,
    ) -> sgx_status_t;
}

pub fn untrusted_submit_block_signatures(
    header: &[u8],
    commit: &[u8],
    txs: &[u8],
    encrypted_random: &[u8],
) -> SgxResult<[u8; 32]> {
    debug!("Hello from just before - untrusted_submit_block_signatures");

    const RETRY_LIMIT: i32 = 3;

    let mut retry_count = 0;

    // this is here so we can
    loop {
        let (retval, decrypted, status) =
            submit_block_signature_impl(header, commit, txs, encrypted_random)?;
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
            return Ok(decrypted);
        }
    }
}

fn submit_block_signature_impl(
    header: &[u8],
    commit: &[u8],
    txs: &[u8],
    encrypted_random: &[u8],
) -> SgxResult<(sgx_status_t, [u8; 32], sgx_status_t)> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    debug!("Hello from just after - untrusted_submit_block_signatures");

    let eid = enclave.geteid();
    let mut retval = sgx_status_t::SGX_SUCCESS;

    // unused if random feature is not turned on
    let mut random_decrypted = [0u8; 32];

    // let status = unsafe { ecall_get_encrypted_seed(eid, &mut retval, cert, cert_len, & mut seed) };
    let status = unsafe {
        ecall_submit_block_signatures(
            eid,
            &mut retval,
            header.as_ptr(),
            header.len() as u32,
            commit.as_ptr(),
            commit.len() as u32,
            txs.as_ptr(),
            txs.len() as u32,
            encrypted_random.as_ptr(),
            encrypted_random.len() as u32,
            &mut random_decrypted
            // val_set.as_ptr(),
            // val_set.len() as u32,
            // next_val_set.as_ptr(),
            // next_val_set.len() as u32,
        )
    };
    Ok((retval, random_decrypted, status))
}
