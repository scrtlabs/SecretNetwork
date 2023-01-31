use enclave_crypto::KeyPair;

#[cfg(feature = "SGX_MODE_HW")]
use enclave_networking::http::{Headers, HttpRequest, Method};
#[cfg(feature = "SGX_MODE_HW")]
use enclave_networking::tls::TlsSession;

#[cfg(feature = "SGX_MODE_HW")]
use log::*;

#[cfg(feature = "SGX_MODE_HW")]
use itertools::Itertools;

#[cfg(feature = "SGX_MODE_HW")]
use sgx_rand::{os, Rng};

#[cfg(feature = "SGX_MODE_HW")]
use sgx_tse::{rsgx_create_report, rsgx_self_report, rsgx_verify_report};

#[cfg(feature = "SGX_MODE_HW")]
use sgx_tcrypto::rsgx_sha256_slice;

use sgx_tcrypto::SgxEccHandle;

use sgx_types::{sgx_quote_sign_type_t, sgx_status_t};

#[cfg(feature = "SGX_MODE_HW")]
use sgx_types::{
    sgx_epid_group_id_t, sgx_quote_nonce_t, sgx_report_data_t, sgx_report_t, sgx_spid_t,
    sgx_target_info_t, SgxResult,
};

#[cfg(feature = "SGX_MODE_HW")]
use std::{
    io::{Read, Write},
    ptr, str,
    string::String,
};

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
use crate::registration::cert::verify_ra_cert;

#[cfg(feature = "SGX_MODE_HW")]
use enclave_crypto::consts::SIGNING_METHOD;

#[cfg(feature = "SGX_MODE_HW")]
use enclave_crypto::consts::SigningMethod;

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
use enclave_crypto::consts::{
    CURRENT_CONSENSUS_SEED_SEALING_PATH, DEFAULT_SGX_SECRET_PATH,
    GENESIS_CONSENSUS_SEED_SEALING_PATH, NODE_ENCRYPTED_SEED_KEY_CURRENT_FILE,
    NODE_ENCRYPTED_SEED_KEY_GENESIS_FILE, NODE_EXCHANGE_KEY_FILE, REGISTRATION_KEY_SEALING_PATH,
};
#[cfg(feature = "SGX_MODE_HW")]
use enclave_networking::endpoints::Endpoint;
#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
use std::sgxfs::remove as SgxFsRemove;

#[cfg(feature = "SGX_MODE_HW")]
use super::ocalls::{ocall_get_quote, ocall_sgx_init_quote};

#[cfg(feature = "SGX_MODE_HW")]
use super::{hex, report::EndorsedAttestationReport};

#[cfg(feature = "SGX_MODE_HW")]
pub const DEV_HOSTNAME: &str = "api.trustedservices.intel.com";

#[cfg(feature = "production")]
pub const SIGRL_SUFFIX: &str = "/sgx/attestation/v4/sigrl/";
#[cfg(feature = "production")]
pub const REPORT_SUFFIX: &str = "/sgx/attestation/v4/report";

#[cfg(all(feature = "SGX_MODE_HW", not(feature = "production")))]
pub const SIGRL_SUFFIX: &str = "/sgx/dev/attestation/v4/sigrl/";
#[cfg(all(feature = "SGX_MODE_HW", not(feature = "production")))]
pub const REPORT_SUFFIX: &str = "/sgx/dev/attestation/v4/report";

/// extra_data size that will store the public key of the attesting node
#[cfg(feature = "SGX_MODE_HW")]
const REPORT_DATA_SIZE: usize = 32;

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
pub const SPID: &str = "783C75FD041E28AEA2DBCD48617577FE";
#[cfg(all(feature = "SGX_MODE_HW", not(feature = "production")))]
pub const SPID: &str = "D0A5D0AF1E244EC7EA2175BC2E32093B";

#[cfg(not(feature = "SGX_MODE_HW"))]
pub fn create_attestation_certificate(
    kp: &KeyPair,
    _sign_type: sgx_quote_sign_type_t,
    _api_key: &[u8],
    _challenge: Option<&[u8]>,
) -> Result<(Vec<u8>, Vec<u8>), sgx_status_t> {
    // init sgx ecc
    let ecc_handle = SgxEccHandle::new();
    let _result = ecc_handle.open();

    // convert keypair private to sgx ecc private
    let (prv_k, pub_k) = ecc_handle.create_key_pair().unwrap();

    // this is the ed25519 public key we want to encode
    let encoded_pubkey = base64::encode(&kp.get_pubkey());

    let (key_der, cert_der) =
        super::cert::gen_ecc_cert(encoded_pubkey, &prv_k, &pub_k, &ecc_handle)?;
    let _result = ecc_handle.close();

    Ok((key_der, cert_der))
}

#[cfg(feature = "SGX_MODE_HW")]
pub fn create_attestation_certificate(
    kp: &KeyPair,
    sign_type: sgx_quote_sign_type_t,
    api_key: &[u8],
    challenge: Option<&[u8]>,
) -> Result<(Vec<u8>, Vec<u8>), sgx_status_t> {
    // extract private key from KeyPair
    let ecc_handle = SgxEccHandle::new();
    let _result = ecc_handle.open();

    // use ephemeral key
    let (prv_k, pub_k) = ecc_handle.create_key_pair().unwrap();

    // call create_report using the secp256k1 public key, and __not__ the P256 one
    let signed_report =
        match create_attestation_report(&kp.get_pubkey(), sign_type, api_key, challenge) {
            Ok(r) => r,
            Err(e) => {
                error!("Error creating attestation report");
                return Err(e);
            }
        };

    let payload: String = serde_json::to_string(&signed_report).map_err(|_| {
        error!("Error serializing report. May be malformed, or badly encoded");
        sgx_status_t::SGX_ERROR_UNEXPECTED
    })?;
    let (key_der, cert_der) = super::cert::gen_ecc_cert(payload, &prv_k, &pub_k, &ecc_handle)?;
    let _result = ecc_handle.close();

    #[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
    validate_report(&cert_der, None);

    Ok((key_der, cert_der))
}

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
pub fn validate_report(cert: &[u8], _override_verify: Option<SigningMethod>) {
    let _ = verify_ra_cert(cert, None).map_err(|e| {
        info!("Error validating created certificate: {:?}", e);
        let _ = SgxFsRemove(GENESIS_CONSENSUS_SEED_SEALING_PATH.as_str());
        let _ = SgxFsRemove(CURRENT_CONSENSUS_SEED_SEALING_PATH.as_str());
        let _ = SgxFsRemove(REGISTRATION_KEY_SEALING_PATH.as_str());
        let _ = SgxFsRemove(
            std::path::Path::new(DEFAULT_SGX_SECRET_PATH)
                .join(NODE_ENCRYPTED_SEED_KEY_GENESIS_FILE)
                .as_path(),
        );
        let _ = SgxFsRemove(
            std::path::Path::new(DEFAULT_SGX_SECRET_PATH)
                .join(NODE_ENCRYPTED_SEED_KEY_CURRENT_FILE)
                .as_path(),
        );
        let _ = SgxFsRemove(
            std::path::Path::new(DEFAULT_SGX_SECRET_PATH)
                .join(NODE_EXCHANGE_KEY_FILE)
                .as_path(),
        );
    });
}

#[cfg(feature = "SGX_MODE_HW")]
pub fn get_mr_enclave() -> [u8; 32] {
    rsgx_self_report().body.mr_enclave.m
}

//input: pub_k: &sgx_ec256_public_t, todo: make this the pubkey of the node
#[cfg(feature = "SGX_MODE_HW")]
#[allow(const_err)]
pub fn create_attestation_report(
    pub_k: &[u8; 32],
    sign_type: sgx_quote_sign_type_t,
    api_key_file: &[u8],
    challenge: Option<&[u8]>,
) -> Result<EndorsedAttestationReport, sgx_status_t> {
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
        ocall_sgx_init_quote(
            &mut rt as *mut sgx_status_t,
            &mut ti as *mut sgx_target_info_t,
            &mut eg as *mut sgx_epid_group_id_t,
        )
    };

    trace!("EPID group = {:?}", eg);

    if res != sgx_status_t::SGX_SUCCESS {
        return Err(res);
    }

    if rt != sgx_status_t::SGX_SUCCESS {
        return Err(rt);
    }

    let eg_num = as_u32_le(eg);

    // Get revocation list from intel
    let sigrl_vec: Vec<u8> = get_sigrl_from_intel(eg_num, api_key_file)?;

    // (2) Generate the report
    // Fill ecc256 public key into report_data
    let mut report_data: sgx_report_data_t = sgx_report_data_t::default();

    report_data.d[..32].copy_from_slice(pub_k);
    if let Some(c) = challenge {
        report_data.d[32..36].copy_from_slice(c);
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
    let mut os_rng = os::SgxRng::new().unwrap();
    os_rng.fill_bytes(&mut quote_nonce.rand);
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

    //&String::from_utf8_lossy(spid_file)
    let spid: sgx_spid_t = hex::decode_spid(SPID);

    let p_spid = &spid as *const sgx_spid_t;
    let p_nonce = &quote_nonce as *const sgx_quote_nonce_t;
    let p_qe_report = &mut qe_report as *mut sgx_report_t;
    let p_quote = return_quote_buf.as_mut_ptr();
    let maxlen = RET_QUOTE_BUF_LEN;
    let p_quote_len = &mut quote_len as *mut u32;

    let result = unsafe {
        ocall_get_quote(
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

    if result != sgx_status_t::SGX_SUCCESS {
        warn!("ocall_get_quote returned {}", result);
        return Err(result);
    }

    if rt != sgx_status_t::SGX_SUCCESS {
        warn!("ocall_get_quote returned {}", rt);
        return Err(rt);
    }

    // Added 09-28-2018
    // Perform a check on qe_report to verify if the qe_report is valid
    match rsgx_verify_report(&qe_report) {
        Ok(()) => trace!("rsgx_verify_report passed!"),
        Err(x) => {
            warn!("rsgx_verify_report failed with {:?}", x);
            return Err(x);
        }
    }

    // Check if the qe_report is produced on the same platform
    if ti.mr_enclave.m != qe_report.body.mr_enclave.m
        || ti.attributes.flags != qe_report.body.attributes.flags
        || ti.attributes.xfrm != qe_report.body.attributes.xfrm
    {
        error!("qe_report does not match current target_info!");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    trace!("QE report check passed");

    // Debug
    // for i in 0..quote_len {
    //     print!("{:02X}", unsafe {*p_quote.offset(i as isize)});
    // }

    // Check qe_report to defend against replay attack
    // The purpose of p_qe_report is for the ISV enclave to confirm the QUOTE
    // it received is not modified by the untrusted SW stack, and not a replay.
    // The implementation in QE is to generate a REPORT targeting the ISV
    // enclave (target info from p_report) , with the lower 32Bytes in
    // report.data = SHA256(p_nonce||p_quote). The ISV enclave can verify the
    // p_qe_report and report.data to confirm the QUOTE has not be modified and
    // is not a replay. It is optional.

    let mut rhs_vec: Vec<u8> = quote_nonce.rand.to_vec();
    rhs_vec.extend(&return_quote_buf[..quote_len as usize]);
    let rhs_hash = rsgx_sha256_slice(&rhs_vec[..]).unwrap();
    let lhs_hash = &qe_report.body.report_data.d[..REPORT_DATA_SIZE];

    trace!("Report rhs hash = {:02X}", rhs_hash.iter().format(""));
    trace!("Report lhs hash = {:02X}", lhs_hash.iter().format(""));

    if rhs_hash != lhs_hash {
        error!("Quote is tampered!");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    let quote_vec: Vec<u8> = return_quote_buf[..quote_len as usize].to_vec();

    let (attn_report, signature, signing_cert) = get_report_from_intel(quote_vec, api_key_file)?;
    Ok(EndorsedAttestationReport {
        report: attn_report.into_bytes(),
        signature,
        signing_cert,
    })
}

#[cfg(feature = "SGX_MODE_HW")]
fn parse_response_attn_report(resp: &[u8]) -> SgxResult<(String, Vec<u8>, Vec<u8>)> {
    trace!("parse_response_attn_report");
    let mut headers = [httparse::EMPTY_HEADER; 16];
    let mut respp = httparse::Response::new(&mut headers);
    let result = respp.parse(resp);
    trace!("parse result {:?}", result);

    match respp.code {
        Some(200) => info!("Response okay"),
        Some(401) => {
            error!("Unauthorized Failed to authenticate or authorize request.");
            return Err(sgx_status_t::SGX_ERROR_INVALID_ENCLAVE);
        }
        Some(404) => {
            error!("Not Found GID does not refer to a valid EPID group ID.");
            return Err(sgx_status_t::SGX_ERROR_INVALID_ENCLAVE);
        }
        Some(500) => {
            error!("Internal error occurred in IAS server");
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
        Some(503) => {
            error!(
                "Service is currently not able to process the request (due to
            a temporary overloading or maintenance). This is a
            temporary state – the same request can be repeated after
            some time. "
            );
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
        _ => {
            error!(
                "response from IAS server :{} - unknown error or response code",
                respp.code.unwrap()
            );
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    }

    let mut len_num: u32 = 0;

    let mut sig = String::new();
    let mut cert = String::new();
    let mut attn_report = String::new();

    for i in 0..respp.headers.len() {
        let h = respp.headers[i];
        //println!("{} : {}", h.name, str::from_utf8(h.value).unwrap());
        match h.name {
            "Content-Length" => {
                let len_str = String::from_utf8(h.value.to_vec()).map_err(|e| {
                    error!("Error decoding content length as utf-8: {:?}", e);
                    sgx_status_t::SGX_ERROR_NETWORK_FAILURE
                })?;

                len_num = len_str.parse::<u32>().unwrap();
                trace!("content length = {}", len_num);
            }
            "X-IASReport-Signature" => {
                sig = str::from_utf8(h.value)
                    .map_err(|e| {
                        error!(
                            "Error decoding X-IASReport-Signature header as utf-8: {:?}",
                            e
                        );
                        sgx_status_t::SGX_ERROR_NETWORK_FAILURE
                    })?
                    .to_string()
            }
            "X-IASReport-Signing-Certificate" => {
                cert = str::from_utf8(h.value)
                    .map_err(|e| {
                        error!(
                            "Error decoding X-IASReport-Signing-Certificate header as utf-8: {:?}",
                            e
                        );
                        sgx_status_t::SGX_ERROR_NETWORK_FAILURE
                    })?
                    .to_string()
            }
            _ => (),
        }
    }

    // Remove %0A from cert, and only obtain the signing cert
    cert = cert.replace("%0A", "");
    cert = hex::percent_decode(cert);

    let v: Vec<&str> = cert.split("-----").collect();

    if v.len() < 3 {
        error!("Error decoding response from IAS server");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }
    let sig_cert = v[2].to_string();

    if len_num != 0 {
        let header_len = result.unwrap().unwrap();
        let resp_body = &resp[header_len..];
        attn_report = str::from_utf8(resp_body)
            .map_err(|e| {
                error!("Error decoding content length as utf-8: {:?}", e);
                sgx_status_t::SGX_ERROR_NETWORK_FAILURE
            })?
            .to_string();
        info!("Attestation report: {}", attn_report);
    }

    let sig_bytes = base64::decode(&sig).map_err(|e| {
        error!("Error decoding sig_bytes ase base64: {:?}", e);
        sgx_status_t::SGX_ERROR_NETWORK_FAILURE
    })?;
    let sig_cert_bytes = base64::decode(&sig_cert).map_err(|e| {
        error!("Error decoding sig_cert base64: {:?}", e);
        sgx_status_t::SGX_ERROR_NETWORK_FAILURE
    })?;
    // len_num == 0
    Ok((attn_report, sig_bytes, sig_cert_bytes))
}

#[cfg(feature = "SGX_MODE_HW")]
fn parse_response_sigrl(resp: &[u8]) -> SgxResult<Vec<u8>> {
    trace!("parse_response_sigrl");
    let mut headers = [httparse::EMPTY_HEADER; 16];
    let mut respp = httparse::Response::new(&mut headers);
    let result = respp.parse(resp);
    trace!("parse result {:?}", result);
    trace!("parse response{:?}", respp);

    let msg: &'static str = match respp.code {
        Some(200) => "OK Operation Successful",
        Some(401) => "Unauthorized Failed to authenticate or authorize request.",
        Some(404) => "Not Found GID does not refer to a valid EPID group ID.",
        Some(500) => "Internal error occurred",
        Some(503) => {
            "Service is currently not able to process the request (due to
            a temporary overloading or maintenance). This is a
            temporary state – the same request can be repeated after
            some time. "
        }
        _ => "Unknown error occurred",
    };

    info!("{}", msg);
    let mut len_num: u32 = 0;

    for i in 0..respp.headers.len() {
        let h = respp.headers[i];
        if h.name == "content-length" {
            let len_str = String::from_utf8(h.value.to_vec()).unwrap();
            len_num = len_str.parse::<u32>().unwrap();
            trace!("content length = {}", len_num);
        }
    }

    if len_num != 0 {
        let header_len = result.unwrap().unwrap();
        let resp_body = &resp[header_len..];
        trace!("Base64-encoded SigRL: {:?}", resp_body);

        let body_utf_8 = str::from_utf8(resp_body).map_err(|e| {
            error!("Error decoding response as utf-8: {:?}", e);
            sgx_status_t::SGX_ERROR_NETWORK_FAILURE
        })?;

        return Ok(base64::decode(body_utf_8).map_err(|e| {
            error!("Error decoding response as base64: {:?}", e);
            sgx_status_t::SGX_ERROR_NETWORK_FAILURE
        }))?;
    }

    // len_num == 0
    Ok(Vec::new())
}

#[cfg(feature = "SGX_MODE_HW")]
pub fn get_sigrl_from_intel(gid: u32, api_key_file: &[u8]) -> SgxResult<Vec<u8>> {
    let ias_key = String::from_utf8_lossy(api_key_file).trim_end().to_owned();

    let req = HttpRequest::new(
        Method::GET,
        DEV_HOSTNAME,
        &format!("{}{:08x}", SIGRL_SUFFIX, gid),
        Some(Headers(vec![(
            "Ocp-Apim-Subscription-Key".to_string(),
            ias_key,
        )])),
        None,
        None,
    );

    trace!("get_sigrl_from_intel: {}", req);

    let mut tls = TlsSession::new(Some(Endpoint::IntelAttestationService), DEV_HOSTNAME, None)
        .map_err(|_| sgx_status_t::SGX_ERROR_NETWORK_FAILURE)?;
    let mut stream = tls.new_stream();
    let _result = stream.write(req.to_string().as_bytes());
    let mut plaintext = Vec::new();

    info!("write complete");
    match stream.read_to_end(&mut plaintext) {
        Ok(_) => {
            info!("read_to_end complete")
        }
        Err(e) => {
            warn!("get_sigrl_from_intel tls.read_to_end: {:?}", e);
            panic!("Communication error with IAS");
        }
    }

    info!("read_to_end complete");
    let resp_string = String::from_utf8(plaintext.clone()).map_err(|e| {
        error!("Error decoding response as utf-8: {}", e);
        sgx_status_t::SGX_ERROR_NETWORK_FAILURE
    })?;

    trace!("{}", resp_string);

    parse_response_sigrl(&plaintext)
}

// TODO: support pse
#[cfg(feature = "SGX_MODE_HW")]
pub fn get_report_from_intel(
    quote: Vec<u8>,
    api_key_file: &[u8],
) -> SgxResult<(String, Vec<u8>, Vec<u8>)> {
    let encoded_quote = base64::encode(&quote[..]);
    let encoded_json = format!("{{\"isvEnclaveQuote\":\"{}\"}}\r\n", encoded_quote);
    let ias_key = String::from_utf8_lossy(api_key_file).trim_end().to_owned();

    let request = HttpRequest::new(
        Method::POST,
        DEV_HOSTNAME,
        REPORT_SUFFIX,
        Some(Headers(vec![(
            "Ocp-Apim-Subscription-Key".to_string(),
            ias_key,
        )])),
        None,
        Some(encoded_json),
    );

    let mut tls = TlsSession::new(Some(Endpoint::IntelAttestationService), DEV_HOSTNAME, None)
        .map_err(|_| sgx_status_t::SGX_ERROR_NETWORK_FAILURE)?;
    let mut stream = tls.new_stream();

    let _result = stream.write(request.to_string().as_bytes());
    let mut plaintext = Vec::new();

    info!("write complete");

    match stream.read_to_end(&mut plaintext) {
        Ok(_) => {
            info!("read_to_end complete")
        }
        Err(e) => {
            warn!("get_report_from_intel stream.read_to_end: {:?}", e);
            panic!("Communication error with IAS");
        }
    }

    let resp_string = String::from_utf8(plaintext.clone()).map_err(|e| {
        error!("Error decoding response as utf-8: {}", e);
        sgx_status_t::SGX_ERROR_NETWORK_FAILURE
    })?;

    trace!("resp_string = {}", resp_string);

    parse_response_attn_report(&plaintext)
}

#[cfg(feature = "SGX_MODE_HW")]
fn as_u32_le(array: [u8; 4]) -> u32 {
    (array[0] as u32)
        + ((array[1] as u32) << 8)
        + ((array[2] as u32) << 16)
        + ((array[3] as u32) << 24)
}
