mod enclave;
mod enclave_api;
mod types;
use clap::App;
use lazy_static::lazy_static;
use sgx_types::{sgx_status_t, sgx_enclave_id_t, sgx_quote_t, sgx_ql_ecdsa_sig_data_t, sgx_ql_auth_data_t, sgx_ql_certification_data_t};
use std::fs::File;
use std::io::Read;
use std::io::Write;
use std::sync::Arc;
use std::net::SocketAddr;
use std::time::{SystemTime, UNIX_EPOCH};
use std::mem;
use std::slice;
use std::fs;
use std::net::IpAddr;
use hyper::service::{make_service_fn, service_fn};
use hyper::server::conn::AddrStream; 
use hyper::{Body, Request, Response};
use hyper::server::Server;
use base64::{engine::general_purpose, Engine as _};
use hex;

use crate::{
    enclave_api::ecall_check_patch_level, enclave_api::ecall_migration_op, types::EnclaveDoorbell,
};

use enclave_ffi_types::NodeAuthResult;

const ENCLAVE_FILE_TESTNET: &str = "check_hw_enclave_testnet.so";
const ENCLAVE_FILE_MAINNET: &str = "check_hw_enclave.so";
const TCS_NUM: u8 = 1;

pub fn get_sgx_secret_path(file_name: &str) -> String {
    std::path::Path::new("/opt/secret/.sgx_secrets/")
        .join(file_name)
        .to_string_lossy()
        .into_owned()
}

lazy_static! {
    static ref ENCLAVE_DOORBELL: EnclaveDoorbell = {
        let is_testnet = std::env::args().any(|arg| arg == "--testnet" || arg == "-t");
        let enclave_file = if is_testnet {
            ENCLAVE_FILE_TESTNET
        } else {
            ENCLAVE_FILE_MAINNET
        };
        EnclaveDoorbell::new(enclave_file, TCS_NUM, is_testnet as i32)
    };
}

pub fn get_path_remote_report() -> String {
    get_sgx_secret_path("migration_report_remote.bin")
}

fn export_rot_seed(eid: sgx_enclave_id_t, remote_report: &[u8]) -> Option<Vec<u8>> {

    match File::create(get_path_remote_report()) {
        Ok(mut f_out) => {
            let _ = f_out.write_all(remote_report);
        },
        Err(_) => {
            return None;
        }
    };

    let mut retval = sgx_status_t::SGX_ERROR_BUSY;
    let _status = unsafe { ecall_migration_op(eid, &mut retval, 7) };
    if retval != sgx_status_t::SGX_SUCCESS {
        return None;
    }

    let res = match File::open(get_sgx_secret_path("rot_seed_encr.bin")) {
        Ok(mut f_in) => {
            let mut buffer = Vec::new();
            match f_in.read_to_end(&mut buffer) {
                Ok(_) => buffer,
                Err(_) => {
                    return None;
                }
            }

        },
        Err(_) => {
            return None;
        }
    };

    Some(res.to_vec())
}

fn log_request(remote_addr: SocketAddr, whole_body: &[u8]) -> std::io::Result<()> {

    let now = SystemTime::now().duration_since(UNIX_EPOCH).unwrap();
    let filename = format!("request_log_{}_{}_from_{}.txt", now.as_secs(), now.subsec_micros(), remote_addr);

    let mut file = File::create(&filename)?;
    file.write_all(whole_body)?;

    Ok(())
}

async fn handle_http_request(eid: sgx_enclave_id_t, self_report: &Arc<Vec<u8>>, req: Request<Body>, remote_addr: SocketAddr) -> Result<Response<Body>, hyper::Error> {
    if req.method() == hyper::Method::POST {

        let whole_body = hyper::body::to_bytes(req.into_body()).await?;

        match export_rot_seed(eid, &whole_body) {
            Some(res) => {

                let _res = log_request(remote_addr, &whole_body);

                let b64_buf1 = general_purpose::STANDARD.encode(&**self_report);
                let b64_buf2 = general_purpose::STANDARD.encode(res);

                let json = format!("{{\"buf1\":\"{}\",\"buf2\":\"{}\"}}", b64_buf1, b64_buf2);
                Ok(Response::new(Body::from(json)))
            },
            None => {
                Ok(Response::builder()
                    .status(500)
                    .body("Failed to export".into())
                    .unwrap())
            }
        }

    } else {
        Ok(Response::builder()
            .status(405)
            .body("Only POST supported".into())
            .unwrap())
    }
}

#[tokio::main]
async fn serve_rot_seed(eid: sgx_enclave_id_t) {

    let mut retval = sgx_status_t::SGX_ERROR_BUSY;
    let _status = unsafe { ecall_migration_op(eid, &mut retval, 1) };
    if retval == sgx_status_t::SGX_SUCCESS {

        let self_report = {
            let mut f_in = File::open(get_path_remote_report()).unwrap();

            let mut buffer = Vec::new();
            f_in.read_to_end(&mut buffer).unwrap();
            buffer
        };

        let self_report = Arc::new(self_report); // <--- wrap in Arc

        let make_svc = {
            let self_report = Arc::clone(&self_report); // clone it into the outer closure

            make_service_fn(move |conn: &AddrStream| {
                let self_report = Arc::clone(&self_report); // clone again into inner closure
                let remote_addr = conn.remote_addr();

                async move {
                    Ok::<_, hyper::Error>(service_fn(move |req| {
                        let self_report = Arc::clone(&self_report); // clone per request
                        async move {
                            handle_http_request(eid, &self_report, req, remote_addr).await
                        }
                    }))
                }
            })
        };        

        let addr = ([0, 0, 0, 0], 3000).into();

        println!("Listening on http://{}", addr);
        Server::bind(&addr).serve(make_svc).await.unwrap();

    } else {
        println!("Couldn't create self migration report: {}", retval);

    }
}

fn extract_asn1_value(cert: &[u8], oid: &[u8]) -> Option<Vec<u8>> {
    let mut offset = match cert.windows(oid.len()).position(|window| window == oid) {
        Some(size) => size,
        None => {
            return None;
        }
    };

    offset += 12; // 11 + TAG (0x04)

    if offset + 2 >= cert.len() {
        return None;
    }

    // Obtain Netscape Comment length
    let mut len = cert[offset] as usize;
    if len > 0x80 {
        len = (cert[offset + 1] as usize) * 0x100 + (cert[offset + 2] as usize);
        offset += 2;
    }

    // Obtain Netscape Comment
    offset += 1;

    if offset + len >= cert.len() {
        return None;
    }

    let payload = cert[offset..offset + len].to_vec();

    Some(payload)
}

fn extract_cpu_cert_from_cert(cert_data: &[u8]) -> Option<Vec<u8>> {
    //println!("******** cert_data: {}", orig_hex::encode(cert_data));

    let pem_text = match std::str::from_utf8(cert_data) {
        Ok(x) => x,
        Err(_) => {
            return None;
        }
    };

    //println!("******** pem: {}", pem_text);

    // Find the first PEM block
    let begin_marker = "-----BEGIN CERTIFICATE-----";
    let end_marker = "-----END CERTIFICATE-----";
    let start = match pem_text.find(begin_marker) {
        Some(x) => x + begin_marker.len(),
        None => {
            println!("no begin");
            return None;
        }
    };

    let end = match pem_text.find(end_marker) {
        Some(x) => x,
        None => {
            println!("no end");
            return None;
        }
    };
    let b64 = &pem_text[start..end];

    // Remove whitespace and line breaks
    let b64_clean: String = b64.chars().filter(|c| !c.is_whitespace()).collect();

    // Decode Base64 into DER
    let der_bytes = match base64::decode(&b64_clean) {
        Ok(x) => x,
        Err(_) => {
            return None;
        }
    };

    //println!("Leaf certificate: {}", orig_hex::encode(&der_bytes));

    let ppid_oid = &[
        0x06, 0x09, 0x2A, 0x86, 0x48, 0x86, 0xF8, 0x4D, 0x01, 0x0D, 0x01,
    ];

    let res = match extract_asn1_value(&der_bytes, ppid_oid) {
        Some(x) => x,
        None => {
            return None;
        }
    };

    Some(res)
}

unsafe fn extract_cpu_cert_from_quote(vec_quote: &[u8]) -> Option<Vec<u8>> {
    let my_p_quote = vec_quote.as_ptr() as *const sgx_quote_t;

    let sig_len = (*my_p_quote).signature_len as usize;
    let whole_len = sig_len.wrapping_add(mem::size_of::<sgx_quote_t>());
    if (whole_len > sig_len)
        && (whole_len <= vec_quote.len())
        && (sig_len >= mem::size_of::<sgx_ql_ecdsa_sig_data_t>())
    {
        let p_ecdsa_sig = (*my_p_quote).signature.as_ptr() as *const sgx_ql_ecdsa_sig_data_t;

        let auth_size_brutto = sig_len - mem::size_of::<sgx_ql_ecdsa_sig_data_t>();
        if auth_size_brutto >= mem::size_of::<sgx_ql_auth_data_t>() {
            let auth_size_max = auth_size_brutto - mem::size_of::<sgx_ql_auth_data_t>();

            let auth_data_wrapper =
                (*p_ecdsa_sig).auth_certification_data.as_ptr() as *const sgx_ql_auth_data_t;

            let auth_hdr_size = (*auth_data_wrapper).size as usize;
            if auth_hdr_size <= auth_size_max {
                let auth_size = auth_size_max - auth_hdr_size;

                if auth_size > mem::size_of::<sgx_ql_certification_data_t>() {
                    let cert_data = (*auth_data_wrapper)
                        .auth_data
                        .as_ptr()
                        .offset(auth_hdr_size as isize)
                        as *const sgx_ql_certification_data_t;

                    let cert_size_max = auth_size - mem::size_of::<sgx_ql_certification_data_t>();
                    let cert_size = (*cert_data).size as usize;
                    if (cert_size <= cert_size_max) && ((*cert_data).cert_key_type == 5) {
                        let cert_data = slice::from_raw_parts(
                            (*cert_data).certification_data.as_ptr(),
                            cert_size,
                        );

                        return extract_cpu_cert_from_cert(cert_data);
                    }
                }
            }
        }
    }

    None
}




fn print_request_details(file_path: &std::path::Path, ip: IpAddr) {

    let mut f_in = File::open(file_path).unwrap();

    let mut buf = [0u8; 4];
    f_in.read_exact(&mut buf).unwrap();
    let size_epid = u32::from_le_bytes(buf);

    f_in.read_exact(&mut buf).unwrap();
    let size_dcap_q = u32::from_le_bytes(buf);

    f_in.read_exact(&mut buf).unwrap();
    let size_dcap_c = u32::from_le_bytes(buf);

    let mut buf = Vec::new();
    buf.resize(size_epid as usize, 0);
    f_in.read_exact(buf.as_mut_slice()).unwrap();

    buf.resize(size_dcap_q as usize, 0);
    f_in.read_exact(buf.as_mut_slice()).unwrap();

    let ppid = unsafe { extract_cpu_cert_from_quote(&buf.as_slice()) };

    buf.resize(size_dcap_c as usize, 0);
    f_in.read_exact(buf.as_mut_slice()).unwrap();

    buf.resize(0, 0);
    f_in.read_to_end(&mut buf).unwrap();

    println!("IP: {}", ip);

    let data = base64::decode(&buf).unwrap();
    let s = String::from_utf8(data).unwrap();
    println!("Metadata: {}", &s);

    if let Some(ppid_val) = ppid {
        println!("ppid: {}", hex::encode(&ppid_val));
    }
}

fn print_request_details2(file_path: &std::path::Path) -> bool {
    if let Some(name) = file_path.file_name().and_then(|n| n.to_str()) {
        if name.starts_with("request_log_") {
            // Example: request_log_1757752506_456332_from_23.81.165.91:40186.txt
            if let Some(from_idx) = name.find("_from_") {
                let after = &name[from_idx + "_from_".len()..];
                if let Some(colon_idx) = after.find(':') {
                    let ip_str = &after[..colon_idx];
                    if let Ok(ip) = ip_str.parse::<IpAddr>() {
                        print_request_details(file_path, ip);
                        return true;
                    }
                }
            }
        }
    }
    false
}

fn print_request_details_dir(directory_path: &str) {
    for entry in fs::read_dir(directory_path).unwrap() {
        let entry = entry.unwrap();
        let path = entry.path();

        if path.is_file() {
            if !print_request_details2(&path) {
                println!("Skipped: {}", path.display());
            }
        }
    }
}

fn main() {
    let matches = App::new("Check HW")
        .version("1.0")
        .arg(
            clap::Arg::with_name("testnet")
                .short("t")
                .long("testnet")
                .help("Run in testnet mode"),
        )
        .arg(
            clap::Arg::with_name("migrate_op")
                .long("migrate_op")
                .value_name("NUMBER") // Describes the expected value
                .help("Specify the migrate operation mode")
                .takes_value(true), // Indicates this flag takes a value
        )
        .arg(
            clap::Arg::with_name("server_seed")
                .long("server_seed")
                .help("Serve the generated seed"),
        )
        .arg(
            clap::Arg::with_name("parse_req")
                .long("parse_req")
                .value_name("path")
                .help("path to the request file")
                .takes_value(true), // Indicates this flag takes a value
        )
        .get_matches();

    //let is_testnet = matches.is_present("testnet");

    println!("Creating enclave instance..");

    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY);

    if let Err(e) = enclave_access_token {
        println!(
            "Failed to get enclave access token: {:?} (is enclave currently running or busy?)",
            e
        );
        return;
    }

    let enclave = enclave_access_token.unwrap().enclave;

    if let Err(e) = enclave {
        println!("Failed to start enclave: {:?}", e);
        return;
    }

    let eid = enclave.unwrap().geteid();

    if let Some(migrate_op) = matches.value_of("migrate_op") {
        let op = migrate_op.parse::<u32>().unwrap();

        let mut retval = sgx_status_t::SGX_ERROR_BUSY;
        let status = unsafe { ecall_migration_op(eid, &mut retval, op) };

        println!("Migration op reval: {}, {}", status, retval);
    } else if matches.is_present("server_seed") {
        serve_rot_seed(eid);
    } else if let Some(req_path) = matches.value_of("parse_req") {
        let dir = req_path.parse::<String>().unwrap();
        print_request_details_dir(&dir);
    } else {
        let mut retval = NodeAuthResult::Success;

        let mut ppid_buf = vec![0u8; 1024]; // initial buffer capacity
        let mut ppid_required_size: u32 = 0;

        let status = unsafe {
            ecall_check_patch_level(
                eid,
                &mut retval,
                ppid_buf.as_mut_ptr(),
                ppid_buf.len() as u32,
                &mut ppid_required_size,
            )
        };

        if status != sgx_status_t::SGX_SUCCESS {
            println!(
                "Failed to run hardware verification test (is the correct enclave in the correct path?)"
            );
            return;
        }

        if retval != NodeAuthResult::Success {
            println!("Failed to verify platform. Please see errors above for more info on what needs to be fixed before you can run a mainnet node. \n\
            If you require assistance or more information, please contact us on Discord or Telegram. In addition, you may use the documentation available at \
            https://docs.scrt.network
            ");
        } else {
            println!(
                "Platform verification successful! You are able to run a mainnet Secret node"
            )
        }

        if (ppid_required_size > 0) && (ppid_required_size as usize <= ppid_buf.len()) {
            ppid_buf.truncate(ppid_required_size as usize);

            println!("Your PPID: {}", hex::encode(&ppid_buf));
        }
    }
}
