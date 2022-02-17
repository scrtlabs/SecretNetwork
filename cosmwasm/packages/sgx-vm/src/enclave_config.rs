use lazy_static::lazy_static;

use parking_lot::Mutex;
use sgx_types::{sgx_enclave_id_t, sgx_status_t, SgxResult};

use enclave_ffi_types::RuntimeConfiguration;

use crate::enclave::ENCLAVE_DOORBELL;

#[allow(clippy::mutex_atomic)]
lazy_static! {
    /// This variable indicates if the enclave configuration has already been set
    static ref SGX_ENCLAVE_CONFIGURED: Mutex<bool> = Mutex::new(false);
}

extern "C" {
    pub fn ecall_configure_runtime(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        config: RuntimeConfiguration,
    ) -> sgx_status_t;
}

#[cfg(feature = "query-node")]
extern "C" {
    pub fn ecall_configure_runtime_qe(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        config: RuntimeConfiguration,
    ) -> sgx_status_t;
}

pub struct EnclaveRuntimeConfig {
    pub module_cache_size: u8,
}

impl EnclaveRuntimeConfig {
    fn to_ffi_type(&self) -> RuntimeConfiguration {
        RuntimeConfiguration {
            module_cache_size: self.module_cache_size,
        }
    }
}

pub fn configure_enclave(config: EnclaveRuntimeConfig) -> SgxResult<()> {
    let mut configured = SGX_ENCLAVE_CONFIGURED.lock();
    if *configured {
        return Ok(());
    }
    *configured = true;
    drop(configured);

    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(false) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    let mut retval = sgx_status_t::SGX_SUCCESS;

    let status =
        unsafe { ecall_configure_runtime(enclave.geteid(), &mut retval, config.to_ffi_type()) };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        return Err(retval);
    }

    #[cfg(feature = "query-node")]
    {
        use crate::enclave::QUERY_ENCLAVE_DOORBELL;

        // Bind the token to a local variable to ensure its
        // destructor runs in the end of the function
        let enclave_access_token = QUERY_ENCLAVE_DOORBELL
            .get_access(false) // This can never be recursive
            .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
        let enclave = (*enclave_access_token)?;

        let status = unsafe {
            ecall_configure_runtime_qe(enclave.geteid(), &mut retval, config.to_ffi_type())
        };

        if status != sgx_status_t::SGX_SUCCESS {
            return Err(status);
        }

        if retval != sgx_status_t::SGX_SUCCESS {
            return Err(retval);
        }
    }

    Ok(())
}
