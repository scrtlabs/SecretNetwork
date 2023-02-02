use log::error;
use sgx_types::{sgx_status_t, SgxResult};

pub fn check_errors(ret1: sgx_status_t, ret2: sgx_status_t, error: &str) -> SgxResult<()> {
    if ret1 != sgx_status_t::SGX_SUCCESS {
        error!("{:?} error from function {:?}", error, ret1);
        return Err(ret1);
    }
    if ret2 != sgx_status_t::SGX_SUCCESS {
        error!("{:?} error from retval {:?}", error, ret2);
        return Err(ret2);
    }

    Ok(())
}
