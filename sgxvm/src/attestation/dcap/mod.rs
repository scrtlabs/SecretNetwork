use sgx_tse::rsgx_create_report;
use sgx_types::*;
use std::untrusted::time::SystemTimeEx;
use std::{time::SystemTime, vec::Vec};

use crate::ocall;

pub mod utils;

/// Returns Quoting Enclave quote with collateral data
pub fn get_qe_quote(
    pub_k: &sgx_ec256_public_t,
    qe_target_info: &sgx_target_info_t,
    quote_size: u32,
) -> SgxResult<(Vec<u8>, Vec<u8>)> {
    let mut report_data: sgx_report_data_t = sgx_report_data_t::default();

    // Copy public key to report data
    let mut pub_k_gx = pub_k.gx;
    pub_k_gx.reverse();
    let mut pub_k_gy = pub_k.gy;
    pub_k_gy.reverse();
    report_data.d[..32].clone_from_slice(&pub_k_gx);
    report_data.d[32..].clone_from_slice(&pub_k_gy);

    // Prepare report
    let report = match rsgx_create_report(qe_target_info, &report_data) {
        Ok(report) => report,
        Err(err) => {
            println!(
                "[Enclave] Call to `rsgx_create_report` failed. Status code: {:?}",
                err
            );
            return Err(err);
        }
    };

    // Get quote from PCCS
    let mut retval = sgx_status_t::SGX_ERROR_UNEXPECTED;
    let mut quote_buf = vec![0u8; quote_size as usize];
    let res = unsafe {
        ocall::ocall_get_ecdsa_quote(
            &mut retval,
            &report as *const sgx_report_t,
            quote_buf.as_mut_ptr(),
            quote_size,
        )
    };

    if res != sgx_status_t::SGX_SUCCESS {
        println!(
            "[Enclave] Call to `ocall_get_ecdsa_quote` failed. Status code: {:?}",
            res
        );
        return Err(res);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        println!(
            "[Enclave] Error during `ocall_get_ecdsa_quote`. Status code: {:?}",
            retval
        );
        return Err(retval);
    }

    // Perform additional check if quote was tampered
    let p_quote3: *const sgx_quote3_t = quote_buf.as_ptr() as *const sgx_quote3_t;
    let quote3: sgx_quote3_t = unsafe { *p_quote3 };

    if quote3.report_body.mr_enclave.m != report.body.mr_enclave.m {
        println!("MRENCLAVE in quote and report are different. Quote was tampered!");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    // Obtain collateral data
    let qe_collateral = get_collateral_data(&quote_buf)?;

    Ok((quote_buf, qe_collateral))
}

fn get_collateral_data(vec_quote: &Vec<u8>) -> SgxResult<Vec<u8>> {
    let mut vec_coll: Vec<u8> = vec![0; 0x4000];
    let mut size_coll: u32 = 0;
    let mut retval = sgx_status_t::SGX_ERROR_UNEXPECTED;

    let res = unsafe {
        ocall::ocall_get_quote_ecdsa_collateral(
            &mut retval as *mut sgx_status_t,
            vec_quote.as_ptr(),
            vec_quote.len() as u32,
            vec_coll.as_mut_ptr(),
            vec_coll.len() as u32,
            &mut size_coll,
        )
    };

    if res != sgx_status_t::SGX_SUCCESS {
        println!(
            "[Enclave] Call to `ocall_get_quote_ecdsa_collateral` failed. Status code: {:?}",
            res
        );
        return Err(res);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        println!(
            "[Enclave] Error during `ocall_get_quote_ecdsa_collateral`. Status code: {:?}",
            retval
        );
        return Err(retval);
    }

    println!("Collateral size = {}", size_coll);

    let call_again = size_coll > vec_coll.len() as u32;
    vec_coll.resize(size_coll as usize, 0);

    if call_again {
        let res = unsafe {
            ocall::ocall_get_quote_ecdsa_collateral(
                &mut retval as *mut sgx_status_t,
                vec_quote.as_ptr(),
                vec_quote.len() as u32,
                vec_coll.as_mut_ptr(),
                vec_coll.len() as u32,
                &mut size_coll,
            )
        };

        if res != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave] Call to `ocall_get_quote_ecdsa_collateral` failed. Status code: {:?}",
                res
            );
            return Err(res);
        }
    
        if retval != sgx_status_t::SGX_SUCCESS {
            println!(
                "[Enclave] Error during `ocall_get_quote_ecdsa_collateral`. Status code: {:?}",
                retval
            );
            return Err(retval);
        }
    }

    Ok(vec_coll)
}

fn get_app_enclave_target_info() -> SgxResult<sgx_target_info_t> {
    let mut app_enclave_target_info = sgx_target_info_t::default();
    let res = unsafe { sgx_self_target(&mut app_enclave_target_info) };

    if res != sgx_status_t::SGX_SUCCESS {
        println!(
            "[Enclave] Error during `sgx_self_target`. Status code: {:?}",
            res
        );
        return Err(res);
    }

    Ok(app_enclave_target_info)
}

fn get_random_nonce() -> SgxResult<Vec<u8>> {
    let mut nonce = vec![0u8; 16];
    let res = unsafe { sgx_read_rand(nonce.as_mut_ptr(), nonce.len()) };
    if res != sgx_status_t::SGX_SUCCESS {
        println!("Call to `sgx_read_rand failed`. Status code: {:?}", res);
        return Err(res);
    }

    Ok(nonce)
}

fn get_supplemental_data_size() -> SgxResult<u32> {
    let mut ret_val = sgx_status_t::SGX_ERROR_UNEXPECTED;
    let mut data_size = 0u32;
    let res = unsafe { ocall::ocall_get_supplemental_data_size(&mut ret_val, &mut data_size) };

    if res != sgx_status_t::SGX_SUCCESS {
        println!(
            "[Enclave] Call to `ocall_get_supplemental_data_size` failed. Status code: {:?}",
            res
        );
        return Err(res);
    }

    if ret_val != sgx_status_t::SGX_SUCCESS {
        println!(
            "[Enclave] Failure during `ocall_get_supplemental_data_size`. Status code: {:?}",
            ret_val
        );
        return Err(ret_val);
    }

    Ok(data_size)
}

fn get_timestamp() -> SgxResult<i64> {
    let timestamp = match SystemTime::now().duration_since(SystemTime::UNIX_EPOCH) {
        Ok(res) => res,
        Err(err) => {
            println!("[Enclave] Cannot get current timestamp. Reason: {:?}", err);
            return SgxResult::Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    };

    let timestamp_secs: i64 = match timestamp.as_secs().try_into() {
        Ok(timestamp_secs) => timestamp_secs,
        Err(err) => {
            println!(
                "[Enclave] Cannot convert current timestamp to i64. Reason: {:?}",
                err
            );
            return SgxResult::Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    };

    Ok(timestamp_secs)
}

pub fn verify_dcap_quote(quote: Vec<u8>, collateral: Vec<u8>) -> SgxResult<Vec<u8>> {
    // Prepare data for enclave
    let mut qve_report_info = sgx_ql_qe_report_info_t::default();

    // Construct buffer for supplemental data
    let supplemental_data_size = get_supplemental_data_size()?;
    let mut supplemental_data = vec![0u8; supplemental_data_size as usize];

    // Generate target_info for enclave
    let app_enclave_target_info = get_app_enclave_target_info()?;

    // Generate random nonce to ensure that quote was not tampered
    let nonce = get_random_nonce()?;

    // Prepare current timestamp
    let timestamp = get_timestamp()?;

    // Prepare report info
    qve_report_info.nonce.rand.copy_from_slice(&nonce);
    qve_report_info.app_enclave_target_info = app_enclave_target_info;

    // Send OCALL to QvE
    let mut retval = sgx_status_t::SGX_ERROR_UNEXPECTED;
    let mut quote_verification_result = sgx_ql_qv_result_t::SGX_QL_QV_RESULT_UNSPECIFIED;
    let mut collateral_expiration_status = 1u32;

    let res = unsafe {
        ocall::ocall_get_qve_report(
            &mut retval as *mut sgx_status_t,
            quote.as_ptr(),
            quote.len() as u32,
            timestamp,
            &mut collateral_expiration_status as *mut u32,
            &mut quote_verification_result as *mut sgx_ql_qv_result_t,
            &mut qve_report_info as *mut sgx_ql_qe_report_info_t,
            supplemental_data.as_mut_ptr(),
            supplemental_data_size,
            collateral.as_ptr(),
            collateral.len() as u32,
        )
    };

    if res != sgx_status_t::SGX_SUCCESS {
        println!(
            "[Enclave] Call to `ocall_get_qve_report` failed. Status code: {:?}",
            res
        );
        return Err(res);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        println!(
            "[Enclave] Failure during `ocall_get_qve_report`. Reason: {:?}",
            retval
        );
        return Err(retval);
    }

    // Verify returned QvE report
    if qve_report_info.nonce.rand.to_vec() != nonce {
        println!("[Enclave] Nonces of input and returned quote are different");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    let qve_isvsvn_threshold: sgx_isv_svn_t = 7;
    let res = unsafe {
        sgx_tvl_verify_qve_report_and_identity(
            quote.as_ptr(),
            quote.len() as u32,
            &qve_report_info as *const sgx_ql_qe_report_info_t,
            timestamp,
            collateral_expiration_status,
            quote_verification_result,
            supplemental_data.as_ptr(),
            supplemental_data_size,
            qve_isvsvn_threshold,
        )
    };

    if res != sgx_quote3_error_t::SGX_QL_SUCCESS {
        println!(
            "[Enclave] Call to `sgx_tvl_verify_qve_report_and_identity` failed. Status code: {:?}",
            res
        );
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    // Check quote verification result
    check_quote_verification_result(quote_verification_result, collateral_expiration_status)?;

    // Inspect quote
    let p_quote3: *const sgx_quote3_t = quote.as_ptr() as *const sgx_quote3_t;
    let quote3: sgx_quote3_t = unsafe { *p_quote3 };

    // Check MRSIGNER
    if quote3.report_body.mr_signer.m != crate::attestation::consts::MRSIGNER {
        println!("[Enclave] Invalid MRSIGNER in received quote");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    // Check ISV SVN
    if quote3.report_body.isv_svn < crate::attestation::consts::MIN_REQUIRED_SVN {
        println!("[Enclave] Quote received from outdated enclave");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    // Check for debug mode
    #[cfg(feature = "production")]
    {
        let is_debug_mode = quote3.report_body.attributes.flags & SGX_FLAGS_DEBUG;
        if (is_debug_mode) != 0 {
            println!("[Enclave] Peer enclave was built in debug mode");
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    }

    // Return report public key
    Ok(quote3.report_body.report_data.d.to_vec())
}

#[cfg(not(feature = "mainnet"))]
fn check_quote_verification_result(
    quote_verification_result: sgx_ql_qv_result_t,
    collateral_expiration_status: u32,
) -> SgxResult<()> {
    match quote_verification_result {
        sgx_ql_qv_result_t::SGX_QL_QV_RESULT_OK => {
            if 0u32 == collateral_expiration_status {
                println!("[Enclave] Quote was verified successfully");
                Ok(())
            } else {
                println!("[Enclave] Quote was verified, but collateral is out of date");
                Err(sgx_status_t::SGX_ERROR_UNEXPECTED)
            }
        }
        sgx_ql_qv_result_t::SGX_QL_QV_RESULT_CONFIG_NEEDED
        | sgx_ql_qv_result_t::SGX_QL_QV_RESULT_OUT_OF_DATE
        | sgx_ql_qv_result_t::SGX_QL_QV_RESULT_OUT_OF_DATE_CONFIG_NEEDED => {
            println!("[Enclave] Quote was verified, but additional system configuration is required. Reason: {:?}", quote_verification_result);
            Ok(())
        }
        sgx_ql_qv_result_t::SGX_QL_QV_RESULT_SW_HARDENING_NEEDED
        | sgx_ql_qv_result_t::SGX_QL_QV_RESULT_CONFIG_AND_SW_HARDENING_NEEDED => {
            println!(
                "[Enclave] Quote verification finished with non-terminal result: {:?}",
                quote_verification_result
            );
            Ok(())
        }
        _ => {
            println!(
                "[Enclave] Quote was not verified successfully. Reason: {:?}",
                quote_verification_result
            );
            Err(sgx_status_t::SGX_ERROR_UNEXPECTED)
        }
    }
}

#[cfg(feature = "mainnet")]
fn check_quote_verification_result(
    quote_verification_result: sgx_ql_qv_result_t,
    collateral_expiration_status: u32,
) -> SgxResult<()> {
    // Check verification result
    match quote_verification_result {
        sgx_ql_qv_result_t::SGX_QL_QV_RESULT_OK
        | sgx_ql_qv_result_t::SGX_QL_QV_RESULT_SW_HARDENING_NEEDED => {
            if 0u32 == collateral_expiration_status {
                println!("[Enclave] Quote was verified successfully");
                Ok(())
            } else {
                println!("[Enclave] Quote was verified, but collateral is out of date");
                Err(sgx_status_t::SGX_ERROR_UNEXPECTED)
            }
        }
        _ => {
            println!(
                "[Enclave] Quote was not verified successfully. Reason: {:?}",
                quote_verification_result
            );
            Err(sgx_status_t::SGX_ERROR_UNEXPECTED)
        }
    }
}
