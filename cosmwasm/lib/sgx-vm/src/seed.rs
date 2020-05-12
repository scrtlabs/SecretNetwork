use sgx_types::*;

extern "C" {
    pub fn ecall_init_seed(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        public_key: *const u8,
        public_key_len: u32,
        encrypted_seed: *const u8,
        encrypted_seed_len: u32
    ) -> sgx_status_t;

    pub fn ecall_init_bootstrap(eid: sgx_enclave_id_t,
                                retval: *mut sgx_status_t,
                                public_key: &mut [u8; 64]) -> sgx_status_t;
}

pub fn inner_init_seed(eid: sgx_enclave_id_t,
                 public_key: *const u8,
                 public_key_len: u32,
                 encrypted_seed: *const u8,
                 encrypted_seed_len: u32) -> SgxResult<sgx_status_t> {

    let mut ret = sgx_status_t::SGX_SUCCESS;

    let status = unsafe { ecall_init_seed(eid, &mut ret, public_key, public_key_len,
                                              encrypted_seed, encrypted_seed_len) };

    if status != sgx_status_t::SGX_SUCCESS  {
        return Err(status);
    }

    if ret != sgx_status_t::SGX_SUCCESS {
        return Err(ret);
    }

    Ok(sgx_status_t::SGX_SUCCESS)
}


pub fn inner_init_bootstrap(eid: sgx_enclave_id_t) -> SgxResult<[u8; 64]> {
    let mut retval = sgx_status_t::SGX_SUCCESS;
    let mut public_key = [0u8; 64];
   // let status = unsafe { ecall_get_encrypted_seed(eid, &mut retval, cert, cert_len, & mut seed) };
    let status = unsafe { ecall_init_bootstrap(eid, &mut retval, &mut public_key) };

    if status != sgx_status_t::SGX_SUCCESS  {
        return Err(status);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        return Err(retval);
    }

    Ok(public_key)
}
