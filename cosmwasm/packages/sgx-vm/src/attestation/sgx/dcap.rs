use log::error;
use sgx_types::{
    sgx_get_quote, sgx_qe_get_quote, sgx_qe_get_quote_size, sgx_qe_get_target_info,
    sgx_quote3_error_t, sgx_report_data_t, sgx_report_t, sgx_status_t, sgx_target_info_t, uint32_t,
    SgxResult,
};

#[no_mangle]
pub extern "C" fn ocall_get_target_info(ret_target_info: *mut sgx_target_info_t) -> sgx_status_t {
    let res = unsafe { sgx_qe_get_target_info(ret_target_info) };
    if res != sgx_quote3_error_t::SGX_QL_SUCCESS {
        error!("Failed to generate dcap target info: {:?}", res);
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    sgx_status_t::SGX_SUCCESS
}

#[no_mangle]
pub extern "C" fn ocall_get_quote_dcap(
    p_qe_report: *const sgx_report_t,
    ret_quote: *mut u8,
    maxlen: u32,
    ret_quote_len: *mut u32,
) -> sgx_status_t {
    let mut quote_size: u32 = 0;
    let res = unsafe { sgx_qe_get_quote_size(&mut quote_size as *mut uint32_t) };
    if res != sgx_quote3_error_t::SGX_QL_SUCCESS {
        panic!("Failed to generate dcap target info");
    }
    let mut quote_vec: Vec<u8> = vec![0; quote_size as usize];

    println!("\nStep4: Call sgx_qe_get_quote:");

    if quote_size > maxlen {
        error!("Enclave buffer too small for quote!");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let ret = unsafe {
        sgx_qe_get_quote(
            p_qe_report,
            quote_size as uint32_t,
            quote_vec.as_mut_ptr() as *mut sgx_types::uint8_t,
        )
    };

    if ret != sgx_quote3_error_t::SGX_QL_SUCCESS {
        println!("Error in sgx_qe_get_quote. {:?}\n", ret);
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    unsafe { ret_quote.copy_from(quote_vec.as_ptr(), quote_size as usize) };
    unsafe {
        *ret_quote_len = quote_size;
    }
    sgx_status_t::SGX_SUCCESS
}

extern "C" {
    #[cfg(feature = "dcap")]
    pub fn ecall_generate_report_dcap(
        target_info: &sgx_target_info_t,
        p_report: &mut sgx_report_t,
    ) -> sgx_status_t;
}
