use core::ptr;
use enclave_crypto::consts::{SigningMethod, SIGNING_METHOD};
use enclave_utils::sgx_errors::check_errors;
use log::{error, trace};
use secret_attestation_token::{AsAttestationToken, AttestationNonce, SecretAttestationToken};

use sgx_tse::rsgx_create_report;
use sgx_types::{
    sgx_epid_group_id_t, sgx_quote_nonce_t, sgx_quote_sign_type_t, sgx_report_data_t, sgx_report_t,
    sgx_spid_t, sgx_status_t, sgx_target_info_t, SgxResult,
};

use crate::ias::{get_report_from_intel, get_sigrl_from_intel, SPID};
use crate::types::EndorsedEpidAttestationReport;
use crate::{ocalls, REPORT_DATA_SIZE};

/// this creates data structure that will be used by the network to validate the node
/// this function is called during the "Generation" phase
#[allow(const_err)]
pub fn impl_generate_authentication_material_hw(
    pub_k: &[u8; 32],
    sign_type: sgx_quote_sign_type_t,
    api_key_file: &[u8],
    nonce: Option<AttestationNonce>,
) -> Result<SecretAttestationToken, sgx_status_t> {
    // Workflow:
    // (1) ocall to get the target_info structure (ti) and epid group id (eg)
    // (1.5) get sigrl
    // (2) call sgx_create_report with ti+data, produce an sgx_report_t
    // (3) ocall to sgx_get_quote to generate (*mut sgx-quote_t, uint32_t)

    // (1) get ti + eg
    let mut ti: sgx_target_info_t = sgx_target_info_t::default();
    let mut eg: sgx_epid_group_id_t = sgx_epid_group_id_t::default();
    let mut rt: sgx_status_t = sgx_status_t::SGX_ERROR_UNEXPECTED;

    let res = unsafe {
        ocalls::ocall_sgx_init_quote(
            &mut rt as *mut sgx_status_t,
            &mut ti as *mut sgx_target_info_t,
            &mut eg as *mut sgx_epid_group_id_t,
        )
    };

    trace!("EPID group = {:?}", eg);

    check_errors(res, rt, "ocall_sgx_init_quote")?;

    let eg_num = u32::from_le_bytes(eg as [u8; 4]);

    // Now sigrl_vec is the revocation list, a vec<u8>
    let sigrl_vec: Vec<u8> = get_sigrl_from_intel(eg_num, api_key_file)?;

    // (2) Generate the report
    // Fill ecc256 public key into report_data
    let mut report_data: sgx_report_data_t = sgx_report_data_t::default();

    report_data.d[..32].copy_from_slice(pub_k);

    // todo: just for backwards compat with the seed service PR
    if let Some(c) = nonce {
        report_data.d[32..36].copy_from_slice(&c);
    }

    let rep = match rsgx_create_report(&ti, &report_data) {
        Ok(r) => {
            match SIGNING_METHOD {
                SigningMethod::MRENCLAVE => {
                    trace!(
                        "Report creation => success. Using MR_SIGNER: {:?}",
                        r.body.mr_signer.m
                    );
                }
                SigningMethod::MRSIGNER => {
                    trace!(
                        "Report creation => success. Got MR_ENCLAVE {:?}",
                        r.body.mr_signer.m
                    );
                }
                SigningMethod::NONE => {
                    trace!("Report creation => success. Not using any verification");
                }
            }
            r
        }
        Err(e) => {
            error!("Report creation => failed {:?}", e);
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    };

    let mut quote_nonce = sgx_quote_nonce_t { rand: [0; 16] };

    if let Some(n) = nonce {
        quote_nonce.rand = n;
    } else {
        enclave_crypto::rand_slice(&mut quote_nonce.rand as &mut [u8; 16]).map_err(|_| {
            error!("Failed to get random nonce");
            sgx_status_t::SGX_ERROR_UNEXPECTED
        })?;
    }

    trace!("Nonce generated successfully");
    let mut qe_report = sgx_report_t::default();
    const RET_QUOTE_BUF_LEN: u32 = 2048;
    let mut return_quote_buf: [u8; RET_QUOTE_BUF_LEN as usize] = [0; RET_QUOTE_BUF_LEN as usize];
    let mut quote_len: u32 = 0;

    // (3) Generate the quote
    // Args:
    //       1. sigrl: ptr + len
    //       2. report: ptr 432bytes
    //       3. linkable: u32, unlinkable=0, linkable=1
    //       4. spid: sgx_spid_t ptr 16bytes
    //       5. sgx_quote_nonce_t ptr 16bytes
    //       6. p_sig_rl + sigrl size ( same to sigrl)
    //       7. [out]p_qe_report need further check
    //       8. [out]p_quote
    //       9. quote_size
    let (p_sigrl, sigrl_len) = if sigrl_vec.is_empty() {
        (ptr::null(), 0)
    } else {
        (sigrl_vec.as_ptr(), sigrl_vec.len() as u32)
    };
    let p_report = (&rep) as *const sgx_report_t;
    let quote_type = sign_type;

    let spid: sgx_spid_t = spid_from_hex(SPID)?;

    let p_spid = &spid as *const sgx_spid_t;
    let p_nonce = &quote_nonce as *const sgx_quote_nonce_t;
    let p_qe_report = &mut qe_report as *mut sgx_report_t;
    let p_quote = return_quote_buf.as_mut_ptr();
    let maxlen = RET_QUOTE_BUF_LEN;
    let p_quote_len = &mut quote_len as *mut u32;

    let result = unsafe {
        crate::ocalls::ocall_get_quote_epid(
            &mut rt as *mut sgx_status_t,
            p_sigrl,
            sigrl_len,
            p_report,
            quote_type,
            p_spid,
            p_nonce,
            p_qe_report,
            p_quote,
            maxlen,
            p_quote_len,
        )
    };

    check_errors(result, rt, "ocall_get_quote")?;

    attestation::sgx_report::verify_report(qe_report, ti)?;

    crate::epid_quote::check_sgx_quote_is_tampered(
        qe_report,
        quote_nonce,
        quote_len,
        &return_quote_buf,
        REPORT_DATA_SIZE,
    )?;

    let quote_vec: Vec<u8> = return_quote_buf[..quote_len as usize].to_vec();

    let (attn_report, signature, signing_cert) = get_report_from_intel(quote_vec, api_key_file)?;

    let mut token: SecretAttestationToken = EndorsedEpidAttestationReport {
        report: attn_report,
        signature,
        cert: signing_cert,
    }
    .as_attestation_token();

    token.node_key = pub_k.clone();

    Ok(token)
}

fn spid_from_hex(hex_string: &str) -> SgxResult<sgx_spid_t> {
    let result = hex::decode(hex_string).map_err(|_| sgx_status_t::SGX_ERROR_INVALID_PARAMETER)?;

    let mut spid = sgx_spid_t::default();

    spid.id.copy_from_slice(&result[0..16]);

    Ok(spid)
}
