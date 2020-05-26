/*
    Copyright 2019 Supercomputing Systems AG
    Licensed under the Apache License, Version 2.0 (the "License");
    you may not use this file except in compliance with the License.
    You may obtain a copy of the License at
        http://www.apache.org/licenses/LICENSE-2.0
    Unless required by applicable law or agreed to in writing, software
    distributed under the License is distributed on an "AS IS" BASIS,
    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    See the License for the specific language governing permissions and
    limitations under the License.
*/

use log::*;

use sgx_trts::trts::{
    rsgx_lfence, rsgx_raw_is_outside_enclave, rsgx_sfence, rsgx_slice_is_outside_enclave,
};
use sgx_types::*;

use crate::attestation::create_attestation_certificate;
use crate::crypto::KeyPair;
use crate::storage::write_to_untrusted;

pub trait UnwrapOrSgxErrorUnexpected {
    type ReturnType;
    fn sgx_error(self) -> Result<Self::ReturnType, sgx_status_t>;
    fn sgx_error_with_log(self, err_mgs: &str) -> Result<Self::ReturnType, sgx_status_t>;
}

impl<T, S> UnwrapOrSgxErrorUnexpected for Result<T, S> {
    type ReturnType = T;
    fn sgx_error(self) -> Result<Self::ReturnType, sgx_status_t> {
        match self {
            Ok(r) => Ok(r),
            Err(_) => Err(sgx_status_t::SGX_ERROR_UNEXPECTED),
        }
    }

    fn sgx_error_with_log(self, log_msg: &str) -> Result<Self::ReturnType, sgx_status_t> {
        match self {
            Ok(r) => Ok(r),
            Err(_) => {
                error!("{}", log_msg);
                Err(sgx_status_t::SGX_ERROR_UNEXPECTED)
            }
        }
    }
}
pub fn validate_mut_ptr(ptr: *mut u8, ptr_len: usize) -> SgxResult<()> {
    if rsgx_raw_is_outside_enclave(ptr, ptr_len) {
        error!("Tried to access memory outside enclave -- rsgx_slice_is_outside_enclave");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }
    rsgx_sfence();
    Ok(())
}

pub fn validate_const_ptr(ptr: *const u8, ptr_len: usize) -> SgxResult<()> {
    if ptr.is_null() || ptr_len == 0 {
        error!("Tried to access an empty pointer - encrypted_seed.is_null()");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }
    rsgx_lfence();
    Ok(())
}

pub fn attest_from_key(kp: &KeyPair, save_path: &str) -> SgxResult<()> {
    let (_, cert) = match create_attestation_certificate(
        &kp,
        sgx_quote_sign_type_t::SGX_UNLINKABLE_SIGNATURE,
    ) {
        Err(e) => {
            error!("Error in create_attestation_certificate: {:?}", e);
            return Err(e);
        }
        Ok(res) => res,
    };
    // info!("private key {:?}, cert: {:?}", private_key_der, cert);

    if let Err(status) = write_to_untrusted(cert.as_slice(), save_path) {
        return Err(status);
    }

    Ok(())
}
