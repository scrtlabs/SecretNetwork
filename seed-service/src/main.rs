mod db;
use rand_core::{OsRng, RngCore};
use std::sync::RwLock;

use crate::db::{create_db, get_seed_count, get_seed_from_db, is_db_exists, to_string, write_seed};
use core::task::{Context, Poll};
use futures_util::ready;
use hyper::server::accept::Accept;
//use hyper::server::conn::http1;
use hyper::server::conn::{AddrIncoming, AddrStream};
use hyper::service::{make_service_fn, service_fn};
use hyper::{Body, Request, Response, Server};
use std::convert::Infallible;
use std::future::Future;
//use std::net::SocketAddr;
use enclave_contract_engine::report::{AdvisoryIDs, AttestationReport, SgxQuoteStatus};
use std::collections::HashMap;
use std::pin::Pin;
use std::sync::Arc;
use std::{fs, io, sync};
use tokio::io::{AsyncRead, AsyncWrite, ReadBuf};
//use tokio::net::TcpListener;
use tokio_rustls::rustls::ServerConfig;

static mut DB_RW_LOCK: Option<RwLock<u8>> = None;
static mut CR_RW_LOCK: Option<RwLock<u8>> = None;
static mut CR_STORE: Option<HashMap<Vec<u8>, String>> = None;

enum State {
    Handshaking(tokio_rustls::Accept<AddrStream>),
    Streaming(tokio_rustls::server::TlsStream<AddrStream>),
}

pub struct TlsStream {
    state: State,
}

impl TlsStream {
    fn new(stream: AddrStream, config: Arc<ServerConfig>) -> TlsStream {
        let accept = tokio_rustls::TlsAcceptor::from(config).accept(stream);
        TlsStream {
            state: State::Handshaking(accept),
        }
    }
}

const WHITELIST_FROM_FILE: &str = include_str!("../../cosmwasm/enclaves/execute/whitelist.txt");

impl AsyncRead for TlsStream {
    fn poll_read(
        self: Pin<&mut Self>,
        cx: &mut Context,
        buf: &mut ReadBuf,
    ) -> Poll<io::Result<()>> {
        let pin = self.get_mut();
        match pin.state {
            State::Handshaking(ref mut accept) => match ready!(Pin::new(accept).poll(cx)) {
                Ok(mut stream) => {
                    let result = Pin::new(&mut stream).poll_read(cx, buf);
                    pin.state = State::Streaming(stream);
                    result
                }
                Err(err) => Poll::Ready(Err(err)),
            },
            State::Streaming(ref mut stream) => Pin::new(stream).poll_read(cx, buf),
        }
    }
}

impl AsyncWrite for TlsStream {
    fn poll_write(
        self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &[u8],
    ) -> Poll<io::Result<usize>> {
        let pin = self.get_mut();
        match pin.state {
            State::Handshaking(ref mut accept) => match ready!(Pin::new(accept).poll(cx)) {
                Ok(mut stream) => {
                    let result = Pin::new(&mut stream).poll_write(cx, buf);
                    pin.state = State::Streaming(stream);
                    result
                }
                Err(err) => Poll::Ready(Err(err)),
            },
            State::Streaming(ref mut stream) => Pin::new(stream).poll_write(cx, buf),
        }
    }

    fn poll_flush(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<io::Result<()>> {
        match self.state {
            State::Handshaking(_) => Poll::Ready(Ok(())),
            State::Streaming(ref mut stream) => Pin::new(stream).poll_flush(cx),
        }
    }

    fn poll_shutdown(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<io::Result<()>> {
        match self.state {
            State::Handshaking(_) => Poll::Ready(Ok(())),
            State::Streaming(ref mut stream) => Pin::new(stream).poll_shutdown(cx),
        }
    }
}

pub struct TlsAcceptor {
    config: Arc<ServerConfig>,
    incoming: AddrIncoming,
}

impl TlsAcceptor {
    pub fn new(config: Arc<ServerConfig>, incoming: AddrIncoming) -> TlsAcceptor {
        TlsAcceptor { config, incoming }
    }
}

impl Accept for TlsAcceptor {
    type Conn = TlsStream;
    type Error = io::Error;

    fn poll_accept(
        self: Pin<&mut Self>,
        cx: &mut Context<'_>,
    ) -> Poll<Option<Result<Self::Conn, Self::Error>>> {
        let pin = self.get_mut();
        match ready!(Pin::new(&mut pin.incoming).poll_accept(cx)) {
            Some(Ok(sock)) => Poll::Ready(Some(Ok(TlsStream::new(sock, pin.config.clone())))),
            Some(Err(e)) => Poll::Ready(Some(Err(e))),
            None => Poll::Ready(None),
        }
    }
}

async fn get_seed(idx: u64) -> io::Result<[u8; 32]> {
    if idx == 0 {
        return Err(error("Unknown seed requested".to_string()));
    }

    if idx == 1 {
        return Err(error(
            "Genesis seed is not stored in the service".to_string(),
        ));
    }

    println!("Requested seed {}", idx);
    let seed_count = get_seed_count()?;

    // Someone is requesting a seed that shouldn't be present
    if idx > seed_count {
        return Err(error("Failed to fetch the requested seed".to_string()));
    }

    if idx + 3 >= seed_count {
        generate_seeds(10).await?;
    }

    unsafe {
        if let Some(rw_lock) = &DB_RW_LOCK {
            let _ = rw_lock
                .read()
                .map_err(|e| error(format!("Failed to aquire read lock {}", e)))?;
        } else {
            return Err(error("Failed to aquire lock".to_string()));
        }

        get_seed_from_db(idx)
    }
}

async fn generate_seeds(count: u8) -> io::Result<()> {
    unsafe {
        if let Some(rw_lock) = &DB_RW_LOCK {
            let _ = rw_lock
                .write()
                .map_err(|e| error(format!("Failed to aquire write lock {}", e)))?;
        } else {
            return Err(error("Failed to aquire lock".to_string()));
        }

        for _ in 1..count + 1 {
            let mut seed = [0u8; 32];
            OsRng.fill_bytes(&mut seed);
            write_seed(seed)?;
        }
    }

    Ok(())
}

async fn get_body_as_string(req: Request<Body>) -> Result<String, String> {
    let body_bytes = hyper::body::to_bytes(req.into_body()).await;
    match body_bytes {
        Err(err) => Err(format!("Failed to read request body: {}", err).to_string()),
        Ok(body) => match String::from_utf8(body.to_vec()) {
            Err(err) => Err(format!("Failed to parse body as string: {}", err).to_string()),
            Ok(str) => Ok(str),
        },
    }
}

fn parse_attestation_report(report: String) -> Result<AttestationReport, String> {
    let decoded_cert = base64::decode(report)
        .map_err(|e| format!("Failed to decode base64: {:?}", e).to_string())?;

    AttestationReport::from_cert(&decoded_cert)
        .map_err(|e| format!("Failed to decode report: {:?}", e).to_string())
}

pub fn verify_quote_status(
    quote_status: &SgxQuoteStatus,
    advisories: &AdvisoryIDs,
) -> Result<(), String> {
    Ok(())
    // match quote_status {
    //     SgxQuoteStatus::OK => Ok(()),
    //     SgxQuoteStatus::SwHardeningNeeded => Ok(()),
    //     SgxQuoteStatus::ConfigurationAndSwHardeningNeeded => {
    //         let vulnerable = advisories.vulnerable();
    //         if vulnerable.is_empty() {
    //             Ok(())
    //         } else {
    //             Err(format!("Platform is updated but requires further BIOS configuration. The following vulnerabilities must be mitigated: {:?}", vulnerable))
    //         }
    //     }
    //     _ => Err(format!(
    //         "Invalid attestation quote status - cannot verify remote node: {:?} Adv {:?}",
    //         quote_status, advisories
    //     )),
    // }
}

fn validate_attestation_report(cert: String) -> Result<AttestationReport, String> {
    // Validate Intel's cert
    let report = match parse_attestation_report(cert) {
        Err(err_str) => Err(format!("Failed to validate Intel's cert: {}", err_str).to_string()),
        Ok(report) => Ok(report),
    }?;

    // Validate challenge
    unsafe {
        if let Some(rw_lock) = &CR_RW_LOCK {
            let _ = rw_lock
                .read()
                .map_err(|e| format!("Failed to aquire read lock {}", e))?;

            match CR_STORE
                .as_ref()
                .unwrap()
                .get(&get_pub_key_from_report(&report))
            {
                None => {
                    return Err("Got response when no challenge sent".to_string());
                }
                Some(challenge) => {
                    if challenge != &base64::encode(get_response_from_report(&report).as_slice()) {
                        return Err("Failed to validate response".to_string());
                    }
                }
            }
        } else {
            return Err("Failed to aquire lock".to_string());
        }
    }

    if !check_epid_gid_is_whitelisted(&report.sgx_quote_body.gid) {
        return Err(format!(
            "Platform verification error: quote status {:?}",
            &report.sgx_quote_body.gid
        ));
    }

    verify_quote_status(&report.sgx_quote_status, &report.advisory_ids)?;

    Ok(report)
}

fn get_response_from_report(report: &AttestationReport) -> Vec<u8> {
    report.sgx_quote_body.isv_enclave_report.report_data[32..36].to_vec()
}

fn get_pub_key_from_report(report: &AttestationReport) -> Vec<u8> {
    report.sgx_quote_body.isv_enclave_report.report_data[0..32].to_vec()
}

fn get_challenge_for_report(cert: String) -> Result<String, String> {
    let mut random_challenge = [0u8; 4];
    OsRng.fill_bytes(&mut random_challenge);

    let serialized_challenge = base64::encode(&random_challenge);
    let report = parse_attestation_report(cert)?;
    println!(
        "Challenged {:?} with {:?}",
        get_pub_key_from_report(&report),
        random_challenge.clone()
    );

    unsafe {
        if let Some(rw_lock) = &CR_RW_LOCK {
            let _ = rw_lock
                .write()
                .map_err(|e| format!("Failed to aquire write lock {}", e))?;
            CR_STORE
                .as_mut()
                .unwrap()
                .insert(vec![], serialized_challenge.clone());
        } else {
            return Err("Failed to aquire lock".to_string());
        }
    }

    Ok(serialized_challenge)
}

async fn handle(req: Request<Body>) -> Result<Response<Body>, Infallible> {
    if req.method().as_str() != "GET" {
        return Ok(Response::builder().status(406).body(Body::empty()).unwrap());
    }

    let path = req.uri().path();
    let response = if path == "/" {
        Response::new(Body::from("Hello World"))
    } else if let Some(idx) = path.strip_prefix("/seed/") {
        let parsed_idx = idx.parse::<u64>();
        if let Some(err) = parsed_idx.clone().err() {
            return Ok(Response::builder()
                .status(403)
                .body(Body::from(format!(
                    "Failed to parse index as number: {}",
                    err
                )))
                .unwrap());
        }

        match get_body_as_string(req).await {
            Err(err_str) => Response::builder()
                .status(403)
                .body(Body::from(err_str))
                .unwrap(),
            Ok(_) => match get_seed(parsed_idx.unwrap()).await {
                Err(e) => {
                    println!("Failed to get a seed: {}", e);
                    Response::builder()
                        .status(403)
                        .body(Body::from(format!("Failed to fetch seed: {}", e)))
                        .unwrap()
                }
                Ok(seed) => {
                    Response::new(Body::from(base64::encode(&seed)))
                }
            },
        }
    } else if path == "/authenticate" {
        match get_body_as_string(req).await {
            Err(err_str) => Response::builder()
                .status(403)
                .body(Body::from(err_str))
                .unwrap(),
            Ok(body) => match get_challenge_for_report(body) {
                Err(err_str) => Response::builder()
                    .status(403)
                    .body(Body::from(err_str))
                    .unwrap(),
                Ok(challenge) => Response::new(Body::from(challenge)),
            },
        }
    } else {
        Response::builder().status(404).body(Body::empty()).unwrap()
    };

    Ok(response)
}

fn error(err: String) -> io::Error {
    io::Error::new(io::ErrorKind::Other, err)
}

fn check_epid_gid_is_whitelisted(epid_gid: &u32) -> bool {
    let decoded = base64::decode(WHITELIST_FROM_FILE).unwrap(); //will never fail since data is constant

    decoded.as_chunks::<4>().0.iter().any(|&arr| {
        if epid_gid == &u32::from_be_bytes(arr) {
            return true;
        }
        false
    })
}

// Load public certificate from file.
fn load_certs(filename: &str) -> io::Result<Vec<rustls::Certificate>> {
    // Open certificate file.
    let certfile = fs::File::open(filename)
        .map_err(|e| error(format!("failed to open {}: {}", filename, e)))?;
    let mut reader = io::BufReader::new(certfile);

    // Load and return certificate.
    let certs = rustls_pemfile::certs(&mut reader)
        .map_err(|_| error("failed to load certificate".into()))?;
    Ok(certs.into_iter().map(rustls::Certificate).collect())
}

// Load private key from file.
fn load_private_key(filename: &str) -> io::Result<rustls::PrivateKey> {
    // Open keyfile.
    let keyfile = fs::File::open(filename)
        .map_err(|e| error(format!("failed to open {}: {}", filename, e)))?;
    let mut reader = io::BufReader::new(keyfile);

    // Load and return a single private key.
    let keys = rustls_pemfile::rsa_private_keys(&mut reader)
        .map_err(|_| error("failed to load private key".into()))?;
    if keys.len() != 1 {
        return Err(error(
            format!("expected a single private key {}", keys.len()).into(),
        ));
    }

    Ok(rustls::PrivateKey(keys[0].clone()))
}

fn main() {
    if let Err(e) = run_server() {
        eprintln!("FAILED: {}", e);
        std::process::exit(1);
    }
}

#[tokio::main(worker_threads = 4)]
async fn run_server() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let port = "4487";
    let addr = format!("0.0.0.0:{}", port).parse()?;
    unsafe {
        DB_RW_LOCK = Some(RwLock::new(0));
        CR_RW_LOCK = Some(RwLock::new(1));
        CR_STORE = Some(HashMap::new());
    }

    if !is_db_exists() {
        create_db()?;
        generate_seeds(100)
            .await
            .map_err(|e| error(format!("Failed to generate seeds: {}", e)))?;
    }

    let tls_cfg = {
        // Load public certificate.
        let certs = load_certs("/server.crt")?;
        // Load private key.
        let key = load_private_key("/server.key")?;
        // Do not use client certificate authentication.
        let mut cfg = rustls::ServerConfig::builder()
            .with_safe_defaults()
            .with_no_client_auth()
            .with_single_cert(certs, key)
            .map_err(|e| error(format!("{}", e)))?;
        // Configure ALPN to accept HTTP/2, HTTP/1.1 in that order.
        cfg.alpn_protocols = vec![b"h2".to_vec(), b"http/1.1".to_vec()];
        sync::Arc::new(cfg)
    };

    // Create a TCP listener via tokio.
    let incoming = AddrIncoming::bind(&addr)?;

    let service =
        make_service_fn(move |_| async { Ok::<_, io::Error>(service_fn(move |req| handle(req))) });
    let server = Server::builder(TlsAcceptor::new(tls_cfg, incoming)).serve(service);

    // Run the future, keep going until an error occurs.
    println!("Starting to serve on https://{}.", addr);
    server.await.map_err(|e| {
        error(format!(
            "Failed to instantiate a server with addr: {}. error: {}",
            addr, e
        ))
    })?;
    Ok(())
}
