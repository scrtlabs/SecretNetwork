mod enclave;
mod enclave_api;
mod types;
use clap::App;
use lazy_static::lazy_static;
use sgx_types::{sgx_status_t, sgx_enclave_id_t};
use std::fs::File;
use std::io::Read;
use std::io::Write;
use std::sync::Arc;
use std::net::SocketAddr;
use std::time::{SystemTime, UNIX_EPOCH};
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

fn log_request(remote_addr: SocketAddr, remote_report: &[u8], metadata: &str) -> std::io::Result<()> {

    let now = SystemTime::now().duration_since(UNIX_EPOCH).unwrap();
    let filename = format!("request_log_{}_{}.txt", now.as_secs(), now.subsec_micros());    

    let mut file = File::create(&filename)?;
    writeln!(file, "Remote Addr: {}", remote_addr)?;
    writeln!(file, "Metadata: {}", metadata)?;
    writeln!(file, "Body (hex): {}", hex::encode(&remote_report))?;

    Ok(())
}

async fn handle_http_request(eid: sgx_enclave_id_t, self_report: &Arc<Vec<u8>>, req: Request<Body>, remote_addr: SocketAddr) -> Result<Response<Body>, hyper::Error> {
    if req.method() == hyper::Method::POST {

        let metadata = if let Some(value) = req.headers().get("secret_metadata") {
            if let Ok(value_str) = value.to_str() {
                //println!("secret_metadata: {}", value_str);
                //println!("addr: {}", remote_addr);
                Some(value_str.to_owned())
            } else {
                None
            }
        } else {
            None
        };

        let whole_body = hyper::body::to_bytes(req.into_body()).await?;

        match export_rot_seed(eid, &whole_body) {
            Some(res) => {

                if let Some(metadata_value) = metadata {
                    let _res = log_request(remote_addr, &whole_body, &metadata_value);
                }

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
        .get_matches();

    let is_testnet = matches.is_present("testnet");

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

    #[allow(clippy::if_same_then_else)]
    let api_key_bytes = if is_testnet {
        include_bytes!("../../ias_keys/develop/api_key.txt")
    } else {
        include_bytes!("../../ias_keys/production/api_key.txt")
    };

    let eid = enclave.unwrap().geteid();

    if let Some(migrate_op) = matches.value_of("migrate_op") {
        let op = migrate_op.parse::<u32>().unwrap();

        let mut retval = sgx_status_t::SGX_ERROR_BUSY;
        let status = unsafe { ecall_migration_op(eid, &mut retval, op) };

        println!("Migration op reval: {}, {}", status, retval);
    } else if matches.is_present("server_seed") {
        serve_rot_seed(eid);
    } else {
        let mut retval = NodeAuthResult::Success;
        let status = unsafe {
            ecall_check_patch_level(
                eid,
                &mut retval,
                api_key_bytes.as_ptr(),
                api_key_bytes.len() as u32,
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
    }
}
