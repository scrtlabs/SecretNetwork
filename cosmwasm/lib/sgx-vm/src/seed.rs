use sgx_types::*;

extern "C" {
    pub fn ecall_init_node(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        master_cert: *const u8,
        master_cert_len: u32,
        encrypted_seed: *const u8,
        encrypted_seed_len: u32,
    ) -> sgx_status_t;

    pub fn ecall_init_bootstrap(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        public_key: &mut [u8; 32],
    ) -> sgx_status_t;

    pub fn ecall_key_gen(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        public_key: &mut [u8; 32],
    ) -> sgx_status_t;
}

pub fn inner_init_node(
    eid: sgx_enclave_id_t,
    master_cert: *const u8,
    master_cert_len: u32,
    encrypted_seed: *const u8,
    encrypted_seed_len: u32,
) -> SgxResult<sgx_status_t> {
    let mut ret = sgx_status_t::SGX_SUCCESS;

    let status = unsafe {
        ecall_init_node(
            eid,
            &mut ret,
            master_cert,
            master_cert_len,
            encrypted_seed,
            encrypted_seed_len,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if ret != sgx_status_t::SGX_SUCCESS {
        return Err(ret);
    }

    Ok(sgx_status_t::SGX_SUCCESS)
}

pub fn inner_key_gen(eid: sgx_enclave_id_t) -> SgxResult<[u8; 32]> {
    let mut retval = sgx_status_t::SGX_SUCCESS;
    let mut public_key = [0u8; 32];
    // let status = unsafe { ecall_get_encrypted_seed(eid, &mut retval, cert, cert_len, & mut seed) };
    let status = unsafe { ecall_key_gen(eid, &mut retval, &mut public_key) };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        return Err(retval);
    }

    Ok(public_key)
}

pub fn inner_init_bootstrap(eid: sgx_enclave_id_t) -> SgxResult<[u8; 32]> {
    let mut retval = sgx_status_t::SGX_SUCCESS;
    let mut public_key = [0u8; 32];
    // let status = unsafe { ecall_get_encrypted_seed(eid, &mut retval, cert, cert_len, & mut seed) };
    let status = unsafe { ecall_init_bootstrap(eid, &mut retval, &mut public_key) };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        return Err(retval);
    }

    Ok(public_key)
}
