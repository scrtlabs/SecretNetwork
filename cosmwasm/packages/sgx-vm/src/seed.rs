use sgx_types::*;

use log::info;

use crate::enclave::get_enclave;

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

pub fn untrusted_init_node(master_cert: &[u8], encrypted_seed: &[u8]) -> SgxResult<sgx_status_t> {
    info!("Hello from just before initializing - init_node");
    let enclave = get_enclave()?;
    info!("Hello from just after initializing - init_node");

    let eid = enclave.geteid();
    let mut ret = sgx_status_t::SGX_SUCCESS;

    let status = unsafe {
        ecall_init_node(
            eid,
            &mut ret,
            master_cert.as_ptr(),
            master_cert.len() as u32,
            encrypted_seed.as_ptr(),
            encrypted_seed.len() as u32,
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

pub fn untrusted_key_gen() -> SgxResult<[u8; 32]> {
    info!("Hello from just before initializing - untrusted_key_gen");
    let enclave = get_enclave()?;
    info!("Hello from just after initializing - untrusted_key_gen");

    let eid = enclave.geteid();
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

pub fn untrusted_init_bootstrap() -> SgxResult<[u8; 32]> {
    info!("Hello from just before initializing - untrusted_init_bootstrap");
    let enclave = get_enclave()?;
    info!("Hello from just after initializing - untrusted_init_bootstrap");

    let eid = enclave.geteid();
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
