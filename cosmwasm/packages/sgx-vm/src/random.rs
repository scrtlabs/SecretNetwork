use sgx_types::*;

use log::debug;

use crate::enclave::ENCLAVE_DOORBELL;

extern "C" {
    pub fn ecall_submit_block_signatures(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        in_header: *const u8,
        in_header_len: u32,
        in_commit: *const u8,
        in_commit_len: u32,
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
    encrypted_random: &[u8],
    // val_set: &[u8],
    // next_val_set: &[u8],
) -> SgxResult<[u8; 32]> {
    debug!("Hello from just before - untrusted_submit_block_signatures");

    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    debug!("Hello from just after - untrusted_submit_block_signatures");

    let eid = enclave.geteid();
    let mut retval = sgx_status_t::SGX_SUCCESS;

    let mut decrypted = [0u8; 32];

    // let status = unsafe { ecall_get_encrypted_seed(eid, &mut retval, cert, cert_len, & mut seed) };
    let status = unsafe {
        ecall_submit_block_signatures(
            eid,
            &mut retval,
            header.as_ptr(),
            header.len() as u32,
            commit.as_ptr(),
            commit.len() as u32,
            encrypted_random.as_ptr(),
            encrypted_random.len() as u32,
            &mut decrypted
            // val_set.as_ptr(),
            // val_set.len() as u32,
            // next_val_set.as_ptr(),
            // next_val_set.len() as u32,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        return Err(retval);
    }

    Ok(decrypted)
}
