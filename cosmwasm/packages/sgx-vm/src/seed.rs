use enclave_ffi_types::HealthCheckResult;
use sgx_types::*;

use log::info;

use crate::enclave::ENCLAVE_DOORBELL;

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
        spid: *const u8,
        spid_len: u32,
        api_key: *const u8,
        api_key_len: u32,
    ) -> sgx_status_t;

    pub fn ecall_key_gen(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        public_key: &mut [u8; 32],
    ) -> sgx_status_t;

    /// Trigger a query method in a wasm contract
    pub fn ecall_health_check(
        eid: sgx_enclave_id_t,
        retval: *mut HealthCheckResult,
    ) -> sgx_status_t;
}

pub fn untrusted_health_check() -> SgxResult<HealthCheckResult> {
    //info!("Initializing enclave..");

    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(false) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    //debug!("Initialized enclave successfully!");

    let eid = enclave.geteid();
    let mut ret = HealthCheckResult::default();

    let status = unsafe { ecall_health_check(eid, &mut ret) };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    Ok(ret)
}

pub fn untrusted_init_node(master_cert: &[u8], encrypted_seed: &[u8]) -> SgxResult<()> {
    info!("Initializing enclave..");

    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(false) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    info!("Initialized enclave successfully!");

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

    Ok(())
}

pub fn untrusted_key_gen() -> SgxResult<[u8; 32]> {
    info!("Initializing enclave..");

    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(false) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    info!("Initialized enclave successfully!");

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

pub fn untrusted_init_bootstrap(spid: &[u8], api_key: &[u8]) -> SgxResult<[u8; 32]> {
    info!("Hello from just before initializing - untrusted_init_bootstrap");

    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(false) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    info!("Hello from just after initializing - untrusted_init_bootstrap");

    let eid = enclave.geteid();
    let mut retval = sgx_status_t::SGX_SUCCESS;
    let mut public_key = [0u8; 32];
    // let status = unsafe { ecall_get_encrypted_seed(eid, &mut retval, cert, cert_len, & mut seed) };
    let status = unsafe {
        ecall_init_bootstrap(
            eid,
            &mut retval,
            &mut public_key,
            spid.as_ptr(),
            spid.len() as u32,
            api_key.as_ptr(),
            api_key.len() as u32,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        return Err(retval);
    }

    Ok(public_key)
}
