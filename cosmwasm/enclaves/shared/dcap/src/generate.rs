use crate::ocalls;
use enclave_utils::sgx_errors::check_errors;
use secret_attestation_token::UserData;
use sgx_types::{
    sgx_report_data_t, sgx_report_t, sgx_status_t, sgx_target_info_t, uint32_t, SgxResult,
};

pub fn generate_quote(user_data: &UserData) -> SgxResult<Vec<u8>> {
    let mut target_info: sgx_target_info_t = sgx_target_info_t::default();
    let mut rt: sgx_status_t = sgx_status_t::SGX_ERROR_UNEXPECTED;

    const RET_QUOTE_BUF_LEN: u32 = 2048;
    let mut return_quote_buf: [u8; RET_QUOTE_BUF_LEN as usize] = [0; RET_QUOTE_BUF_LEN as usize];
    let quote_len: u32 = 0;

    let res = unsafe {
        ocalls::ocall_sgx_get_target_info(
            &mut rt as *mut sgx_status_t,
            &mut target_info as *mut sgx_target_info_t,
        )
    };
    check_errors(res, rt, "ocall_sgx_get_target_info")?;

    let mut report_data: sgx_report_data_t = sgx_report_data_t::default();
    report_data.d[..32].copy_from_slice(user_data);

    let report = sgx_tse::rsgx_create_report(&target_info, &report_data)?;

    let res = unsafe {
        ocalls::ocall_get_quote_dcap(
            &mut rt as *mut sgx_status_t,
            &report as *const sgx_report_t,
            RET_QUOTE_BUF_LEN,
            return_quote_buf.as_mut_ptr(),
            quote_len as *mut uint32_t,
        )
    };

    check_errors(res, rt, "ocall_get_quote_dcap")?;

    Ok(return_quote_buf.to_vec())
}
