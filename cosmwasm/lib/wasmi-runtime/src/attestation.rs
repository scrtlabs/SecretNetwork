#[cfg(feature = "SGX_MODE_HW")]
use itertools::Itertools;

use log::*;
use sgx_rand::*;
use sgx_tcrypto::*;
use sgx_tse::*;
use sgx_types::*;
use std::io::{Read, Write};
use std::net::TcpStream;
use std::prelude::v1::*;

use std::ptr;
use std::str;
use std::string::String;
use std::sync::Arc;
use std::untrusted::fs;
use std::vec::Vec;

use crate::crypto::KeyPair;
use crate::consts::{API_KEY_FILE, SPID_FILE};
use crate::hex;
use crate::imports::*;

pub const DEV_HOSTNAME: &'static str = "api.trustedservices.intel.com";

#[cfg(feature = "production")]
pub const SIGRL_SUFFIX: &str = "/sgx/attestation/v4/sigrl/";
#[cfg(feature = "production")]
pub const REPORT_SUFFIX: &str = "/sgx/attestation/v4/report";

#[cfg(not(feature = "production"))]
pub const SIGRL_SUFFIX: &str = "/sgx/dev/attestation/v4/sigrl/";
#[cfg(not(feature = "production"))]
pub const REPORT_SUFFIX: &str = "/sgx/dev/attestation/v4/report";

pub const CERTEXPIRYDAYS: i64 = 90i64;

// extra_data size (limit to 64)
static REPORT_DATA_SIZE: usize = 64;

#[cfg(not(feature = "SGX_MODE_HW"))]
pub fn software_mode_quote() -> sgx_status_t {
    // info!("produce_quote entered");
    // let spid = hex::decode(spid).unwrap();
    // let mut id = [0; 16];
    // id.copy_from_slice(&spid);
    // let spid: sgx_spid_t = sgx_spid_t { id };
    // info!("before check_busy");
    // // create quote
    // let (status, (target_info, _gid)) = check_busy(|| {
    //     let mut target_info = sgx_target_info_t::default();
    //     let mut gid = sgx_epid_group_id_t::default();
    //     info!("before sgx_init_quote");
    //     let status = unsafe { sgx_init_quote(&mut target_info, &mut gid) };
    //     (status, (target_info, gid))
    // });
    // if status != sgx_status_t::SGX_SUCCESS {
    //     return Err(Error::SdkErr { inner: status }.into());
    // }
    //
    // // create report
    // let (status, (report, retval)) = check_busy(move || {
    //     let mut report = sgx_report_t::default();
    //     let mut retval = sgx_status_t::SGX_SUCCESS;
    //     info!("before ecall_get_registration_quote");
    //     let status = unsafe { ecall_get_registration_quote(eid, &mut retval, &target_info, &mut report) };
    //     (status, (report, retval))
    // });
    //
    // if status != sgx_status_t::SGX_SUCCESS || retval != sgx_status_t::SGX_SUCCESS {
    //     return Err(Error::SdkErr { inner: status }.into());
    // }
    //
    //
    // // calc quote size
    // let (status, quote_size) = check_busy(|| {
    //     let mut quote_size: u32 = 0;
    //     info!("before sgx_calc_quote_size");
    //     let status = unsafe { sgx_calc_quote_size(std::ptr::null(), 0, &mut quote_size) };
    //     (status, quote_size)
    // });
    // if status != sgx_status_t::SGX_SUCCESS || quote_size == 0 {
    //     return Err(Error::SdkErr { inner: status }.into());
    // }
    //
    // // get the actual quote
    // let (status, the_quote) = check_busy(|| {
    //     let mut the_quote = vec![0u8; quote_size as usize].into_boxed_slice();
    //     // all of this is according to this: https://software.intel.com/en-us/sgx-sdk-dev-reference-sgx-get-quote
    //     // the `p_qe_report` is null together with the nonce because we don't have an ISV enclave that needs to verify this
    //     // and we don't care about replay attacks because the signing key will stay the same and that's what's important.
    //     let status = unsafe {
    //         info!("before sgx_get_quote");
    //         sgx_get_quote(&report,
    //                       sgx_quote_sign_type_t::SGX_UNLINKABLE_SIGNATURE,
    //                       &spid,
    //                       std::ptr::null(),
    //                       std::ptr::null(),
    //                       0,
    //                       std::ptr::null_mut(),
    //                       the_quote.as_mut_ptr() as *mut sgx_quote_t,
    //                       quote_size,
    //         )
    //     };
    //     (status, the_quote)
    // });
    // if status != sgx_status_t::SGX_SUCCESS {
    //     return Err(Error::SdkErr { inner: status }.into());
    // }
    //
    // let encoded_quote = base64::encode(&the_quote);
    // info!("encoded_quote: {:?}", encoded_quote);
    // Ok(encoded_quote)

    let (status, rt, (target_info, _gid)) = {
        let mut target_info = sgx_target_info_t::default();
        let mut epid = sgx_epid_group_id_t::default();
        // let mut report_data: sgx_report_data_t = sgx_report_data_t::default();
        let mut rt: sgx_status_t = sgx_status_t::SGX_ERROR_UNEXPECTED;
        info!("before sgx_init_quote");
        let status = unsafe {
            ocall_sgx_init_quote(
                &mut rt as *mut sgx_status_t,
                &mut target_info as *mut sgx_target_info_t,
                &mut epid as *mut sgx_epid_group_id_t,
            )
        };
        (status, rt, (target_info, epid))
    };
    if status != sgx_status_t::SGX_SUCCESS {
        return status;
    }
    if rt != sgx_status_t::SGX_SUCCESS {
        return rt;
    }

    let mut report = sgx_report_t::default();

    create_report_with_data(&target_info, &mut report, &[0u8])
}

#[cfg(not(feature = "SGX_MODE_HW"))]
pub fn create_report_with_data(
    target_info: &sgx_target_info_t,
    out_report: &mut sgx_report_t,
    extra_data: &[u8],
) -> sgx_status_t {
    let mut report_data: sgx_report_data_t = sgx_report_data_t::default();
    // secret data to be attached with the report.
    // if extra_data.len() > REPORT_DATA_SIZE {
    //     return sgx_status_t::SGX_ERROR_INVALID_PARAMETER;
    // }
    // report_data.d[..extra_data.len()].copy_from_slice(extra_data);
    let mut report = sgx_report_t::default();
    let ret = unsafe {
        sgx_create_report(
            target_info as *const sgx_target_info_t,
            &report_data as *const sgx_report_data_t,
            &mut report as *mut sgx_report_t,
        )
    };
    let result = match ret {
        sgx_status_t::SGX_SUCCESS => Ok(report),
        _ => Err(ret),
    };
    match result {
        Ok(r) => {
            *out_report = r;
            sgx_status_t::SGX_SUCCESS
        }
        Err(err) => {
            // println!("[-] Enclave: error creating report");
            sgx_status_t::from(err)
        }
    }
}

// todo: add public/private key handling pub_k: &sgx_ec256_public_t,
#[cfg(feature = "SGX_MODE_HW")]
pub fn create_attestation_certificate(
    kp: &KeyPair,
    sign_type: sgx_quote_sign_type_t,
) -> Result<(Vec<u8>, Vec<u8>), sgx_status_t> {

    let mut priv_key_buf: [u8; 32] = [0u8; 32];

    priv_key_buf.copy_from_slice(kp.get_privkey());





    let ecc_handle = SgxEccHandle::new();
    let _result = ecc_handle.open();

    let prv_k = sgx_ec256_private_t {
        r: priv_key_buf.clone()
    };
    let pub_k = rsgx_ecc256_pub_from_priv(&prv_k).unwrap();

    // if we want to use ephemeral certificates, we can do this
    // let (prv_k, pub_k) = ecc_handle.create_key_pair().unwrap();

    let (attn_report, sig, cert) = match create_attestation_report(&pub_k, sign_type) {
        Ok(r) => r,
        Err(e) => {
            error!("Error in create_attestation_report: {:?}", e);
            return Err(e);
        }
    };

    let payload = attn_report + "|" + &sig + "|" + &cert;
    let (key_der, cert_der) = match crate::cert::gen_ecc_cert(payload, &prv_k, &pub_k, &ecc_handle)
    {
        Ok(r) => r,
        Err(e) => {
            error!("Error in gen_ecc_cert: {:?}", e);
            return Err(e);
        }
    };
    let _result = ecc_handle.close();

    Ok((key_der, cert_der))
}

//input: pub_k: &sgx_ec256_public_t, todo: make this the pubkey of the node
#[cfg(feature = "SGX_MODE_HW")]
#[allow(const_err)]
pub fn create_attestation_report(
    pub_k: &sgx_ec256_public_t,
    sign_type: sgx_quote_sign_type_t,
) -> Result<(String, String, String), sgx_status_t> {
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

    debug!("EPID group = {:?}", eg);

    if res != sgx_status_t::SGX_SUCCESS {
        return Err(res);
    }

    if rt != sgx_status_t::SGX_SUCCESS {
        return Err(rt);
    }

    let eg_num = as_u32_le(&eg);

    // (1.5) get sigrl
    let mut ias_sock: i32 = 0;

    let res =
        unsafe { ocall_get_ias_socket(&mut rt as *mut sgx_status_t, &mut ias_sock as *mut i32) };

    if res != sgx_status_t::SGX_SUCCESS {
        return Err(res);
    }

    if rt != sgx_status_t::SGX_SUCCESS {
        return Err(rt);
    }

    debug!("Got ias_sock successfully = {}", ias_sock);

    // Now sigrl_vec is the revocation list, a vec<u8>
    let sigrl_vec: Vec<u8> = get_sigrl_from_intel(ias_sock, eg_num);

    // (2) Generate the report
    // Fill ecc256 public key into report_data
    let mut report_data: sgx_report_data_t = sgx_report_data_t::default();

    let mut pub_k_gx = pub_k.gx.clone();
    pub_k_gx.reverse();
    let mut pub_k_gy = pub_k.gy.clone();
    pub_k_gy.reverse();
    report_data.d[..32].clone_from_slice(&pub_k_gx);
    report_data.d[32..].clone_from_slice(&pub_k_gy);

    let rep = match rsgx_create_report(&ti, &report_data) {
        Ok(r) => {
            info!("Report creation => success {:?}", r.body.mr_signer.m);
            Some(r)
        }
        Err(e) => {
            info!("Report creation => failed {:?}", e);
            None
        }
    };

    let mut quote_nonce = sgx_quote_nonce_t { rand: [0; 16] };
    let mut os_rng = os::SgxRng::new().unwrap();
    os_rng.fill_bytes(&mut quote_nonce.rand);
    debug!("Nonce generated successfully");
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
    let (p_sigrl, sigrl_len) = if sigrl_vec.len() == 0 {
        (ptr::null(), 0)
    } else {
        (sigrl_vec.as_ptr(), sigrl_vec.len() as u32)
    };
    let p_report = (&rep.unwrap()) as *const sgx_report_t;
    let quote_type = sign_type;

    let spid: sgx_spid_t = load_spid(SPID_FILE);

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
        error!("ocall_get_quote returned {}", result);
        return Err(result);
    }

    if rt != sgx_status_t::SGX_SUCCESS {
        error!("ocall_get_quote returned {}", rt);
        return Err(rt);
    }

    // Added 09-28-2018
    // Perform a check on qe_report to verify if the qe_report is valid
    match rsgx_verify_report(&qe_report) {
        Ok(()) => println!("rsgx_verify_report passed!"),
        Err(x) => {
            error!("rsgx_verify_report failed with {:?}", x);
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

    info!("QE report check passed");

    // Debug
    // for i in 0..quote_len {
    //     print!("{:02X}", unsafe {*p_quote.offset(i as isize)});
    // }
    // println!("");

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
    let lhs_hash = &qe_report.body.report_data.d[..32];

    debug!("Report rhs hash = {:02X}", rhs_hash.iter().format(""));
    debug!("Report lhs hash = {:02X}", lhs_hash.iter().format(""));

    if rhs_hash != lhs_hash {
        error!("Quote is tampered!");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    let quote_vec: Vec<u8> = return_quote_buf[..quote_len as usize].to_vec();
    let res =
        unsafe { ocall_get_ias_socket(&mut rt as *mut sgx_status_t, &mut ias_sock as *mut i32) };

    if res != sgx_status_t::SGX_SUCCESS {
        return Err(res);
    }

    if rt != sgx_status_t::SGX_SUCCESS {
        return Err(rt);
    }

    let (attn_report, sig, cert) = get_report_from_intel(ias_sock, quote_vec);
    Ok((attn_report, sig, cert))
}

#[cfg(feature = "SGX_MODE_HW")]
fn parse_response_attn_report(resp: &[u8]) -> (String, String, String) {
    debug!("parse_response_attn_report");
    let mut headers = [httparse::EMPTY_HEADER; 16];
    let mut respp = httparse::Response::new(&mut headers);
    let result = respp.parse(resp);
    debug!("parse result {:?}", result);

    let msg: &'static str;

    match respp.code {
        Some(200) => msg = "OK Operation Successful",
        Some(401) => msg = "Unauthorized Failed to authenticate or authorize request.",
        Some(404) => msg = "Not Found GID does not refer to a valid EPID group ID.",
        Some(500) => msg = "Internal error occurred",
        Some(503) => {
            msg = "Service is currently not able to process the request (due to
            a temporary overloading or maintenance). This is a
            temporary state – the same request can be repeated after
            some time. "
        }
        _ => {
            error!("DBG:{}", respp.code.unwrap());
            msg = "Unknown error occured"
        }
    }

    debug!("{}", msg);
    let mut len_num: u32 = 0;

    let mut sig = String::new();
    let mut cert = String::new();
    let mut attn_report = String::new();

    for i in 0..respp.headers.len() {
        let h = respp.headers[i];
        //println!("{} : {}", h.name, str::from_utf8(h.value).unwrap());
        match h.name {
            "Content-Length" => {
                let len_str = String::from_utf8(h.value.to_vec()).unwrap();
                len_num = len_str.parse::<u32>().unwrap();
                debug!("content length = {}", len_num);
            }
            "X-IASReport-Signature" => sig = str::from_utf8(h.value).unwrap().to_string(),
            "X-IASReport-Signing-Certificate" => {
                cert = str::from_utf8(h.value).unwrap().to_string()
            }
            _ => (),
        }
    }

    // Remove %0A from cert, and only obtain the signing cert
    cert = cert.replace("%0A", "");
    cert = hex::percent_decode(cert);

    let v: Vec<&str> = cert.split("-----").collect();
    let sig_cert = v[2].to_string();

    if len_num != 0 {
        let header_len = result.unwrap().unwrap();
        let resp_body = &resp[header_len..];
        attn_report = str::from_utf8(resp_body).unwrap().to_string();
        info!("Attestation report: {}", attn_report);
    }

    // len_num == 0
    (attn_report, sig, sig_cert)
}

#[cfg(feature = "SGX_MODE_HW")]
fn parse_response_sigrl(resp: &[u8]) -> Vec<u8> {
    println!("parse_response_sigrl");
    let mut headers = [httparse::EMPTY_HEADER; 16];
    let mut respp = httparse::Response::new(&mut headers);
    let result = respp.parse(resp);
    println!("parse result {:?}", result);
    println!("parse response{:?}", respp);

    let msg: &'static str;

    match respp.code {
        Some(200) => msg = "OK Operation Successful",
        Some(401) => msg = "Unauthorized Failed to authenticate or authorize request.",
        Some(404) => msg = "Not Found GID does not refer to a valid EPID group ID.",
        Some(500) => msg = "Internal error occurred",
        Some(503) => {
            msg = "Service is currently not able to process the request (due to
            a temporary overloading or maintenance). This is a
            temporary state – the same request can be repeated after
            some time. "
        }
        _ => msg = "Unknown error occured",
    }

    println!("{}", msg);
    let mut len_num: u32 = 0;

    for i in 0..respp.headers.len() {
        let h = respp.headers[i];
        if h.name == "content-length" {
            let len_str = String::from_utf8(h.value.to_vec()).unwrap();
            len_num = len_str.parse::<u32>().unwrap();
            println!("content length = {}", len_num);
        }
    }

    if len_num != 0 {
        let header_len = result.unwrap().unwrap();
        let resp_body = &resp[header_len..];
        println!("Base64-encoded SigRL: {:?}", resp_body);

        return base64::decode(str::from_utf8(resp_body).unwrap()).unwrap();
    }

    // len_num == 0
    Vec::new()
}

#[cfg(feature = "SGX_MODE_HW")]
pub fn make_ias_client_config() -> rustls::ClientConfig {
    let mut config = rustls::ClientConfig::new();

    config
        .root_store
        .add_server_trust_anchors(&webpki_roots::TLS_SERVER_ROOTS);

    config
}

#[cfg(feature = "SGX_MODE_HW")]
pub fn get_sigrl_from_intel(fd: c_int, gid: u32) -> Vec<u8> {
    info!("get_sigrl_from_intel fd = {:?}", fd);
    let config = make_ias_client_config();
    let ias_key = get_ias_api_key();

    let req = format!("GET {}{:08x} HTTP/1.1\r\nHOST: {}\r\nOcp-Apim-Subscription-Key: {}\r\nConnection: Close\r\n\r\n",
                      SIGRL_SUFFIX,
                      gid,
                      DEV_HOSTNAME,
                      ias_key);

    info!("get_sigrl_from_intel: {}", req);

    let dns_name = webpki::DNSNameRef::try_from_ascii_str(DEV_HOSTNAME).unwrap();
    let mut sess = rustls::ClientSession::new(&Arc::new(config), dns_name);
    let mut sock = TcpStream::new(fd).unwrap();
    let mut tls = rustls::Stream::new(&mut sess, &mut sock);

    let _result = tls.write(req.as_bytes());
    let mut plaintext = Vec::new();

    info!("write complete");

    match tls.read_to_end(&mut plaintext) {
        Ok(_) => (),
        Err(e) => {
            info!("get_sigrl_from_intel tls.read_to_end: {:?}", e);
            panic!("haha");
        }
    }
    info!("read_to_end complete");
    let resp_string = String::from_utf8(plaintext.clone()).unwrap();

    debug!("{}", resp_string);

    // resp_string

    parse_response_sigrl(&plaintext)
}

// TODO: support pse
#[cfg(feature = "SGX_MODE_HW")]
pub fn get_report_from_intel(fd: c_int, quote: Vec<u8>) -> (String, String, String) {
    info!("get_report_from_intel fd = {:?}", fd);
    let config = make_ias_client_config();
    let encoded_quote = base64::encode(&quote[..]);
    let encoded_json = format!("{{\"isvEnclaveQuote\":\"{}\"}}\r\n", encoded_quote);
    let ias_key = get_ias_api_key();

    let req = format!("POST {} HTTP/1.1\r\nHOST: {}\r\nOcp-Apim-Subscription-Key:{}\r\nContent-Length:{}\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n{}",
                      REPORT_SUFFIX,
                      DEV_HOSTNAME,
                      ias_key,
                      encoded_json.len(),
                      encoded_json);

    info!("{}", req);
    let dns_name = webpki::DNSNameRef::try_from_ascii_str(DEV_HOSTNAME).unwrap();
    let mut sess = rustls::ClientSession::new(&Arc::new(config), dns_name);
    let mut sock = TcpStream::new(fd).unwrap();
    let mut tls = rustls::Stream::new(&mut sess, &mut sock);

    let _result = tls.write(req.as_bytes());
    let mut plaintext = Vec::new();

    info!("write complete");

    tls.read_to_end(&mut plaintext).unwrap();
    info!("read_to_end complete");
    let resp_string = String::from_utf8(plaintext.clone()).unwrap();

    info!("resp_string = {}", resp_string);

    let (attn_report, sig, cert) = parse_response_attn_report(&plaintext);

    (attn_report, sig, cert)
}

#[cfg(feature = "SGX_MODE_HW")]
fn as_u32_le(array: &[u8; 4]) -> u32 {
    ((array[0] as u32) << 0)
        + ((array[1] as u32) << 8)
        + ((array[2] as u32) << 16)
        + ((array[3] as u32) << 24)
}

fn load_spid(filename: &str) -> sgx_spid_t {
    let mut spidfile = fs::File::open(filename).expect("cannot open spid file");
    let mut contents = String::new();
    spidfile
        .read_to_string(&mut contents)
        .expect("cannot read the spid file");

    hex::decode_spid(&contents)
}

#[cfg(feature = "SGX_MODE_HW")]
fn get_ias_api_key() -> String {
    let mut keyfile = fs::File::open(API_KEY_FILE).expect("cannot open ias key file");
    let mut key = String::new();
    keyfile
        .read_to_string(&mut key)
        .expect("cannot read the ias key file");
    key.trim_end().to_owned()
}
