use super::VmError;
use snafu::{Backtrace, Snafu};

/// An error in the communication with the enclave
#[derive(Debug, Snafu)]
#[non_exhaustive]
pub enum EnclaveError {
    #[snafu(display("{}", error))]
    EnclaveErr {
        error: enclave_ffi_types::EnclaveError,
        backtrace: Backtrace,
    },
    #[snafu(display("SGX error: {:?}", status))]
    SdkErr {
        status: sgx_types::sgx_status_t,
        backtrace: Backtrace,
    },
}

impl EnclaveError {
    pub fn enclave_err(error: enclave_ffi_types::EnclaveError) -> Self {
        EnclaveErr { error }.build()
    }

    pub fn sdk_err(status: sgx_types::sgx_status_t) -> Self {
        SdkErr { status }.build()
    }
}

impl From<EnclaveError> for VmError {
    fn from(error: EnclaveError) -> Self {
        VmError::EnclaveErr { source: error }
    }
}

impl From<enclave_ffi_types::EnclaveError> for VmError {
    fn from(error: enclave_ffi_types::EnclaveError) -> Self {
        match error {
            enclave_ffi_types::EnclaveError::OutOfGas => VmError::GasDepletion,
            enclave_ffi_types::EnclaveError::FailedOcall { vm_error }
                if !vm_error.ptr.is_null() =>
            // This error is boxed during ocalls.
            unsafe { *Box::<VmError>::from_raw(vm_error.ptr as *mut _) }
            other => EnclaveError::enclave_err(other).into(),
        }
    }
}
