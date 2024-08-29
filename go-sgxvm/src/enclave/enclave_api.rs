use crate::enclave::attestation::dcap_utils;
use crate::errors::Error;
use crate::types::{AllocationWithResult, GoQuerier};
use sgx_types::*;

pub struct EnclaveApi;

impl EnclaveApi {
    pub fn check_node_status(eid: sgx_enclave_id_t) -> Result<(), Error> {
        let mut ret_val = sgx_status_t::SGX_ERROR_UNEXPECTED;
        let res = unsafe { super::ecall_status(eid, &mut ret_val) };

        match (res, ret_val) {
            (sgx_status_t::SGX_SUCCESS, sgx_status_t::SGX_SUCCESS) => Ok(()),
            (_, _) => {
                let error_str = if res == sgx_status_t::SGX_SUCCESS {
                    res.as_str()
                } else {
                    ret_val.as_str()
                };
                Err(Error::enclave_error(error_str))
            }
        }
    }

    pub fn initialize_enclave(eid: sgx_enclave_id_t, reset: bool) -> Result<(), Error> {
        let mut ret_val = sgx_status_t::SGX_ERROR_UNEXPECTED;
        let res = unsafe { super::ecall_initialize_enclave(eid, &mut ret_val, reset as i32) };

        match (res, ret_val) {
            (sgx_status_t::SGX_SUCCESS, sgx_status_t::SGX_SUCCESS) => Ok(()),
            (_, _) => {
                let error_str = if res == sgx_status_t::SGX_SUCCESS {
                    res.as_str()
                } else {
                    ret_val.as_str()
                };
                Err(Error::enclave_error(error_str))
            }
        }
    }

    pub fn attest_peer(eid: sgx_enclave_id_t, fd: i32, is_dcap: bool) -> Result<(), Error> {
        match is_dcap {
            true => EnclaveApi::attest_peer_dcap(eid, fd),
            false => EnclaveApi::attest_peer_epid(eid, fd),
        }
    }

    pub fn dump_dcap_quote(eid: sgx_enclave_id_t, filepath: &str) -> Result<(), Error> {
        dcap_utils::dump_dcap_quote(eid, filepath)
    }

    pub fn verify_dcap_quote(eid: sgx_enclave_id_t, filepath: &str) -> Result<(), Error> {
        dcap_utils::verify_dcap_quote(eid, filepath)
    }

    fn attest_peer_dcap(eid: sgx_enclave_id_t, fd: i32) -> Result<(), Error> {
        let qe_target_info = dcap_utils::get_qe_target_info()?;
        let quote_size = dcap_utils::get_quote_size()?;

        let mut retval = sgx_status_t::SGX_ERROR_UNEXPECTED;
        let res = unsafe {
            super::ecall_attest_peer_dcap(eid, &mut retval, fd, &qe_target_info, quote_size)
        };

        if res != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave Wrapper] Cannot call `ecall_attest_peer_dcap`. Reason: {:?}",
                res
            );
            return Err(Error::enclave_error(res));
        }

        if retval != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave Wrapper] `ecall_attest_peer_dcap` failed. Reason: {:?}",
                retval
            );
            return Err(Error::enclave_error(retval));
        }

        Ok(())
    }

    fn attest_peer_epid(eid: sgx_enclave_id_t, fd: i32) -> Result<(), Error> {
        let mut retval = sgx_status_t::SGX_ERROR_UNEXPECTED;
        let res = unsafe { super::ecall_attest_peer_epid(eid, &mut retval, fd) };

        if res != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave Wrapper] Cannot call `ecall_attest_peer_epid`. Reason: {:?}",
                res
            );
            return Err(Error::enclave_error(res));
        }

        if retval != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave Wrapper] `ecall_attest_peer_epid` failed. Reason: {:?}",
                retval
            );
            return Err(Error::enclave_error(retval));
        }

        Ok(())
    }

    pub fn request_remote_attestation(
        eid: sgx_enclave_id_t,
        hostname: String,
        fd: i32,
        is_dcap: bool,
    ) -> Result<(), Error> {
        match is_dcap {
            true => EnclaveApi::perform_dcap_attestation(eid, hostname, fd),
            false => EnclaveApi::perform_epid_attestation(eid, hostname, fd),
        }
    }

    pub fn perform_epid_attestation(
        eid: sgx_enclave_id_t,
        hostname: String,
        fd: i32,
    ) -> Result<(), Error> {
        if hostname.is_empty() {
            return Err(Error::unset_arg("Hostname was not set"));
        }

        let mut ret_val = sgx_status_t::SGX_ERROR_UNEXPECTED;
        let res = unsafe {
            super::ecall_request_epoch_keys_epid(
                eid,
                &mut ret_val,
                hostname.as_ptr() as *const u8,
                hostname.len(),
                fd,
            )
        };

        if res != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave] Call to `ecall_request_epoch_keys_epid` failed. Status code: {:?}",
                res
            );
            return Err(Error::enclave_error(res));
        }

        if ret_val != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave] `ecall_request_epoch_keys_epid` returned error: {:?}",
                ret_val
            );
            return Err(Error::enclave_error(ret_val));
        }

        Ok(())
    }

    pub fn perform_dcap_attestation(
        eid: sgx_enclave_id_t,
        hostname: String,
        fd: i32,
    ) -> Result<(), Error> {
        println!("[Enclave Wrapper] perform_dcap_attestation");

        // Validate provided host
        if hostname.is_empty() {
            return Err(Error::unset_arg("Hostname was not set"));
        }

        let qe_target_info = dcap_utils::get_qe_target_info()?;
        let quote_size = dcap_utils::get_quote_size()?;

        let mut retval = sgx_status_t::SGX_ERROR_UNEXPECTED;
        let res = unsafe {
            super::ecall_request_epoch_keys_dcap(
                eid,
                &mut retval,
                hostname.as_ptr() as *const u8,
                hostname.len(),
                fd,
                &qe_target_info,
                quote_size,
            )
        };

        if res != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave Wrapper] Cannot call `ecall_request_epoch_keys_dcap`. Reason: {:?}",
                res
            );
            return Err(Error::enclave_error(res));
        }

        if retval != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave Wrapper] `ecall_request_epoch_keys_dcap` failed. Reason: {:?}",
                retval
            );
            return Err(Error::enclave_error(retval));
        }

        Ok(())
    }

    pub fn is_enclave_initialized(eid: sgx_enclave_id_t) -> Result<bool, Error> {
        let mut ret_val = 0i32;
        let res = unsafe { super::ecall_is_initialized(eid, &mut ret_val) };

        match res {
            sgx_status_t::SGX_SUCCESS => Ok(ret_val != 0),
            _ => Err(Error::enclave_error(res.as_str())),
        }
    }

    pub fn add_epoch(eid: sgx_enclave_id_t, starting_block: u64) -> Result<(), Error> {
        let mut retval = sgx_status_t::SGX_ERROR_UNEXPECTED;
        let res = unsafe { super::ecall_add_epoch(eid, &mut retval, starting_block) };

        if res != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave Wrapper] Cannot call `ecall_add_epoch`. Reason: {:?}",
                res
            );
            return Err(Error::enclave_error(res));
        }

        if retval != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave Wrapper] `ecall_add_epoch` failed. Reason: {:?}",
                retval
            );
            return Err(Error::enclave_error(retval));
        }

        Ok(())
    }

    pub fn list_epochs(eid: sgx_enclave_id_t) -> Result<Vec<u8>, Error> {
        let mut ret_val = std::mem::MaybeUninit::<AllocationWithResult>::uninit();
        let res = unsafe { super::ecall_list_epochs(eid, ret_val.as_mut_ptr()) };

        if res != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave Wrapper] Cannot call `ecall_list_epochs`. Reason: {:?}",
                res
            );
            return Err(Error::enclave_error(res));
        }

        let request_result = unsafe { ret_val.assume_init() };
        // Parse execution result
        match request_result.status {
            sgx_status_t::SGX_SUCCESS => {
                let data = unsafe {
                    Vec::from_raw_parts(
                        request_result.result_ptr,
                        request_result.result_size,
                        request_result.result_size,
                    )
                };
                Ok(data)
            }
            err => {
                println!(
                    "[Enclave Wrapper] `ecall_list_epochs` failed. Reason: {:?}",
                    err
                );
                Err(Error::vm_err(err))
            }
        }
    }

    pub fn remove_latest_epoch(eid: sgx_enclave_id_t) -> Result<(), Error> {
        let mut retval = sgx_status_t::SGX_ERROR_UNEXPECTED;
        let res = unsafe { super::ecall_remove_latest_epoch(eid, &mut retval) };

        if res != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave Wrapper] Cannot call `ecall_remove_latest_epoch`. Reason: {:?}",
                res
            );
            return Err(Error::enclave_error(res));
        }

        if retval != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave Wrapper] `ecall_remove_latest_epoch` failed. Reason: {:?}",
                retval
            );
            return Err(Error::enclave_error(retval));
        }

        Ok(())
    }

    pub fn handle_evm_request(
        eid: sgx_enclave_id_t,
        request_bytes: &[u8],
        querier: GoQuerier,
    ) -> Result<Vec<u8>, Error> {
        let request_vec = Vec::from(request_bytes);
        let mut querier = querier;
        let mut ret_val = std::mem::MaybeUninit::<AllocationWithResult>::uninit();

        let evm_res = unsafe {
            super::handle_request(
                eid,
                ret_val.as_mut_ptr(),
                &mut querier as *mut GoQuerier,
                request_vec.as_ptr(),
                request_vec.len(),
            )
        };

        let handle_request_result = unsafe { ret_val.assume_init() };
        if evm_res != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave Wrapper] Call to handle_request failed. Status code: {:?}",
                evm_res
            );
            return Err(Error::enclave_error(evm_res));
        }

        // Parse execution result
        match handle_request_result.status {
            sgx_status_t::SGX_SUCCESS => {
                let data = unsafe {
                    Vec::from_raw_parts(
                        handle_request_result.result_ptr,
                        handle_request_result.result_size,
                        handle_request_result.result_size,
                    )
                };
                Ok(data)
            }
            err => {
                println!("[Enclave Wrapper] EVM call failed. Status code: {:?}", err);
                Err(Error::vm_err(err))
            }
        }
    }
}
