use enclave_networking::http::{Headers, HttpRequest, Method};
use enclave_networking::tls::TlsSession;
use enclave_networking::endpoints::Endpoint;

use log::{error, info, trace, warn};
use sgx_types::{sgx_status_t, SgxResult};
use std::{
    io::{BufReader, Read, Write},
    str,
    string::String,
};

pub const INTEL_SERVICES_HOSTNAME: &str = "api.trustedservices.intel.com";
pub const IAS_REPORT_CA: &[u8] = include_bytes!("../Intel_SGX_Attestation_RootCA.pem");

#[cfg(feature = "production")]
pub const SIGRL_SUFFIX: &str = "/sgx/attestation/v4/sigrl/";
#[cfg(all(feature = "SGX_MODE_HW", not(feature = "production")))]
pub const SIGRL_SUFFIX: &str = "/sgx/dev/attestation/v4/sigrl/";

#[cfg(feature = "production")]
pub const REPORT_SUFFIX: &str = "/sgx/attestation/v4/report";
#[cfg(all(feature = "SGX_MODE_HW", not(feature = "production")))]
pub const REPORT_SUFFIX: &str = "/sgx/dev/attestation/v4/report";

#[cfg(all(feature = "SGX_MODE_HW", feature = "production"))]
pub const SPID: &str = "783C75FD041E28AEA2DBCD48617577FE";
#[cfg(all(feature = "SGX_MODE_HW", not(feature = "production")))]
pub const SPID: &str = "D0A5D0AF1E244EC7EA2175BC2E32093B";

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
                let len_str = String::from_utf8(h.value.to_vec()).unwrap();
                len_num = len_str.parse::<u32>().unwrap();
                trace!("content length = {}", len_num);
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
    cert = percent_decode(cert);

    let v: Vec<&str> = cert.split("-----").collect();

    if v.len() < 3 {
        error!("Error decoding response from IAS server");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }
    let sig_cert = v[2].to_string();

    if len_num != 0 {
        let header_len = result.unwrap().unwrap();
        let resp_body = &resp[header_len..];
        attn_report = str::from_utf8(resp_body).unwrap().to_string();
        info!("Attestation report: {}", attn_report);
    }

    let sig_bytes = base64::decode(&sig).unwrap();
    let sig_cert_bytes = base64::decode(&sig_cert).unwrap();
    // len_num == 0
    Ok((attn_report, sig_bytes, sig_cert_bytes))
}

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

pub fn get_sigrl_from_intel(gid: u32, api_key_file: &[u8]) -> SgxResult<Vec<u8>> {
    let ias_key = String::from_utf8_lossy(api_key_file).trim_end().to_owned();

    let req = HttpRequest::new(
        Method::GET,
        INTEL_SERVICES_HOSTNAME,
        &format!("{}{:08x}", SIGRL_SUFFIX, gid),
        Some(Headers(vec![(
            "Ocp-Apim-Subscription-Key".to_string(),
            ias_key,
        )])),
        None,
        None,
    );

    trace!("get_sigrl_from_intel: {}", req);

    let mut tls = TlsSession::new(Some(Endpoint::IntelAttestationService), INTEL_SERVICES_HOSTNAME, None)
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

pub fn get_report_from_intel(
    quote: Vec<u8>,
    api_key_file: &[u8],
) -> SgxResult<(String, Vec<u8>, Vec<u8>)> {
    let encoded_quote = base64::encode(&quote[..]);
    let encoded_json = format!("{{\"isvEnclaveQuote\":\"{}\"}}\r\n", encoded_quote);
    let ias_key = String::from_utf8_lossy(api_key_file).trim_end().to_owned();

    let request = HttpRequest::new(
        Method::POST,
        INTEL_SERVICES_HOSTNAME,
        REPORT_SUFFIX,
        Some(Headers(vec![(
            "Ocp-Apim-Subscription-Key".to_string(),
            ias_key,
        )])),
        None,
        Some(encoded_json),
    );

    let mut tls = TlsSession::new(Some(Endpoint::IntelAttestationService), INTEL_SERVICES_HOSTNAME, None)
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

pub fn get_ias_auth_config() -> (Vec<u8>, rustls::RootCertStore) {
    // Verify if the signing cert is issued by Intel CA
    let mut ias_ca_stripped = IAS_REPORT_CA.to_vec();
    ias_ca_stripped.retain(|&x| x != 0x0d && x != 0x0a);
    let head_len = "-----BEGIN CERTIFICATE-----".len();
    let tail_len = "-----END CERTIFICATE-----".len();
    let full_len = ias_ca_stripped.len();
    let ias_ca_core: &[u8] = &ias_ca_stripped[head_len..full_len - tail_len];
    let ias_cert_dec = base64::decode_config(ias_ca_core, base64::STANDARD).unwrap();

    let mut ca_reader = BufReader::new(IAS_REPORT_CA);

    let mut root_store = rustls::RootCertStore::empty();
    root_store
        .add_pem_file(&mut ca_reader)
        .expect("Failed to add CA");

    (ias_cert_dec, root_store)
}

pub fn percent_decode(orig: String) -> String {
    let v: Vec<&str> = orig.split('%').collect();
    let mut ret = String::new();
    ret.push_str(v[0]);
    if v.len() > 1 {
        for s in v[1..].iter() {
            ret.push(u8::from_str_radix(&s[0..2], 16).unwrap() as char);
            ret.push_str(&s[2..]);
        }
    }
    ret
}
