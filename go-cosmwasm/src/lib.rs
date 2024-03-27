mod api;
mod db;
mod error;
mod gas_meter;
mod iterator;
mod logger;
mod memory;
mod querier;

pub use api::GoApi;
pub use db::{db_t, DB};
use logger::get_log_level;
pub use memory::{free_rust, Buffer};
pub use querier::GoQuerier;

use std::convert::TryInto;
use std::panic::{catch_unwind, AssertUnwindSafe};
use std::str::from_utf8;

use crate::error::{clear_error, handle_c_error, handle_c_error_default, set_error, Error};

use cosmwasm_sgx_vm::untrusted_init_bootstrap;
use cosmwasm_sgx_vm::{
    call_handle_raw, call_init_raw, call_migrate_raw, call_query_raw, call_update_admin_raw,
    features_from_csv, Checksum, CosmCache, Extern,
};
use cosmwasm_sgx_vm::{
    create_attestation_report_u, untrusted_get_encrypted_genesis_seed,
    untrusted_get_encrypted_seed, untrusted_health_check, untrusted_init_node, untrusted_key_gen,
    untrusted_migrate_sealing,
};

use ctor::ctor;
use log::*;

#[ctor]
fn init_logger() {
    let default_log_level = log::Level::Info;
    simple_logger::init_with_level(get_log_level(default_log_level)).unwrap();
}

#[repr(C)]
pub struct cache_t {}

fn to_cache(ptr: *mut cache_t) -> Option<&'static mut CosmCache<DB, GoApi, GoQuerier>> {
    if ptr.is_null() {
        None
    } else {
        let c = unsafe { &mut *(ptr as *mut CosmCache<DB, GoApi, GoQuerier>) };
        Some(c)
    }
}

#[no_mangle]
pub extern "C" fn get_health_check(err: Option<&mut Buffer>) -> Buffer {
    match untrusted_health_check() {
        Err(e) => {
            set_error(Error::enclave_err(e.to_string()), err);
            Buffer::default()
        }
        Ok(res) => {
            clear_error();
            Buffer::from_vec(format!("{:?}", res).into_bytes())
        }
    }
}

#[no_mangle]
pub extern "C" fn get_encrypted_seed(cert: Buffer, err: Option<&mut Buffer>) -> Buffer {
    trace!("Called get_encrypted_seed");
    let cert_slice = match unsafe { cert.read() } {
        None => {
            set_error(Error::empty_arg("attestation_cert"), err);
            return Buffer::default();
        }
        Some(r) => r,
    };
    trace!("Hello from right before untrusted_get_encrypted_seed");
    match untrusted_get_encrypted_seed(cert_slice) {
        Err(e) => {
            // An error happened in the SGX sdk.
            set_error(Error::enclave_err(e.to_string()), err);
            Buffer::default()
        }
        Ok(Err(e)) => {
            // An error was returned from the enclave.
            set_error(Error::enclave_err(e.to_string()), err);
            Buffer::default()
        }
        Ok(Ok(seed)) => {
            clear_error();
            Buffer::from_vec(seed.to_vec())
        }
    }
}

#[no_mangle]
pub extern "C" fn get_encrypted_genesis_seed(pk: Buffer, err: Option<&mut Buffer>) -> Buffer {
    trace!("Called get_encrypted_genesis_seed");
    let pk_slice = match unsafe { pk.read() } {
        None => {
            set_error(Error::empty_arg("public_key"), err);
            return Buffer::default();
        }
        Some(r) => r,
    };
    trace!("Hello from right before untrusted_get_encrypted_genesis_seed");
    match untrusted_get_encrypted_genesis_seed(pk_slice) {
        Err(e) => {
            // An error happened in the SGX sdk.
            set_error(Error::enclave_err(e.to_string()), err);
            Buffer::default()
        }
        Ok(seed) => {
            clear_error();
            Buffer::from_vec(seed.to_vec())
        }
    }
}

#[no_mangle]
pub extern "C" fn init_bootstrap(
    spid: Buffer,
    api_key: Buffer,
    err: Option<&mut Buffer>,
) -> Buffer {
    trace!("Hello from right before init_bootstrap");

    let spid_slice = match unsafe { spid.read() } {
        None => {
            set_error(Error::empty_arg("spid"), err);
            return Buffer::default();
        }
        Some(r) => r,
    };

    let api_key_slice = match unsafe { api_key.read() } {
        None => {
            set_error(Error::empty_arg("api_key"), err);
            return Buffer::default();
        }
        Some(r) => r,
    };

    match untrusted_init_bootstrap(spid_slice, api_key_slice) {
        Err(e) => {
            set_error(Error::enclave_err(e.to_string()), err);
            Buffer::default()
        }
        Ok(r) => {
            clear_error();
            Buffer::from_vec(r.to_vec())
        }
    }
}

#[no_mangle]
pub extern "C" fn init_node(
    master_key: Buffer,
    encrypted_seed: Buffer,
    api_key: Buffer,
    err: Option<&mut Buffer>,
) -> bool {
    let pk_slice = match unsafe { master_key.read() } {
        None => {
            set_error(Error::empty_arg("master_key"), err);
            return false;
        }
        Some(r) => r,
    };
    let encrypted_seed_slice = match unsafe { encrypted_seed.read() } {
        None => {
            set_error(Error::empty_arg("encrypted_seed"), err);
            return false;
        }
        Some(r) => r,
    };
    let api_key_slice = match unsafe { api_key.read() } {
        None => {
            set_error(Error::empty_arg("api_key"), err);
            return false;
        }
        Some(r) => r,
    };

    match untrusted_init_node(pk_slice, encrypted_seed_slice, api_key_slice) {
        Ok(()) => {
            clear_error();
            true
        }
        Err(e) => {
            set_error(Error::enclave_err(e.to_string()), err);
            false
        }
    }
}

#[no_mangle]
pub extern "C" fn create_attestation_report(api_key: Buffer, err: Option<&mut Buffer>) -> bool {
    let api_key_slice = match unsafe { api_key.read() } {
        None => {
            set_error(Error::empty_arg("api_key"), err);
            return false;
        }
        Some(r) => r,
    };

    if let Err(status) = create_attestation_report_u(api_key_slice) {
        set_error(Error::enclave_err(status.to_string()), err);
        return false;
    }
    clear_error();
    true
}

fn to_extern(storage: DB, api: GoApi, querier: GoQuerier) -> Extern<DB, GoApi, GoQuerier> {
    Extern {
        storage,
        api,
        querier,
    }
}

#[no_mangle]
pub extern "C" fn init_cache(
    data_dir: Buffer,
    supported_features: Buffer,
    // TODO: remove unused cache size
    _cache_size: usize,
    err: Option<&mut Buffer>,
) -> *mut cache_t {
    let r = catch_unwind(|| do_init_cache(data_dir, supported_features))
        .unwrap_or_else(|_| Err(Error::panic()));
    match r {
        Ok(t) => {
            clear_error();
            t as *mut cache_t
        }
        Err(e) => {
            set_error(e, err);
            std::ptr::null_mut()
        }
    }
}

#[no_mangle]
pub extern "C" fn submit_block_signatures(
    header: Buffer,
    commit: Buffer,
    txs: Buffer,
    random: Buffer,
    // val_set: Buffer,
    // next_val_set: Buffer,
    err: Option<&mut Buffer>,
) -> Buffer {
    trace!("Hello from right before init_bootstrap");

    let header_slice = match unsafe { header.read() } {
        None => {
            set_error(Error::empty_arg("header"), err);
            return Buffer::default();
        }
        Some(r) => r,
    };

    let commit_slice = match unsafe { commit.read() } {
        None => {
            set_error(Error::empty_arg("api_key"), err);
            return Buffer::default();
        }
        Some(r) => r,
    };

    let txs_slice = match unsafe { txs.read() } {
        None => {
            set_error(Error::empty_arg("txs"), err);
            return Buffer::default();
        }
        Some(r) => r,
    };

    let random_slice = match unsafe { random.read() } {
        None => {
            set_error(Error::empty_arg("random"), err);
            return Buffer::default();
        }
        Some(r) => r,
    };
    // let val_set_slice = match unsafe { val_set.read() } {
    //     None => {
    //         set_error(Error::empty_arg("api_key"), err);
    //         return Buffer::default();
    //     }
    //     Some(r) => r,
    // };
    //
    // let next_val_set_slice = match unsafe { next_val_set.read() } {
    //     None => {
    //         set_error(Error::empty_arg("api_key"), err);
    //         return Buffer::default();
    //     }
    //     Some(r) => r,
    // };

    match cosmwasm_sgx_vm::untrusted_submit_block_signatures(
        header_slice,
        commit_slice,
        txs_slice,
        random_slice,
        // val_set_slice,
        // next_val_set_slice,
    ) {
        Err(e) => {
            set_error(Error::enclave_err(e.to_string()), err);
            Buffer::default()
        }
        Ok(r) => {
            clear_error();
            Buffer::from_vec(r.to_vec())
        }
    }
}

// store some common string for argument names
static DATA_DIR_ARG: &str = "data_dir";
static FEATURES_ARG: &str = "supported_features";
static CACHE_ARG: &str = "cache";
static WASM_ARG: &str = "wasm";
static CODE_ID_ARG: &str = "code_id";
static MSG_ARG: &str = "msg";
static PARAMS_ARG: &str = "params";
static GAS_USED_ARG: &str = "gas_used";
static SIG_INFO_ARG: &str = "sig_info";
static CURRENT_ADMIN_ARG: &str = "current_admin";
static CURRENT_ADMIN_PROOF_ARG: &str = "current_admin_proof";

fn do_init_cache(
    data_dir: Buffer,
    supported_features: Buffer,
) -> Result<*mut CosmCache<DB, GoApi, GoQuerier>, Error> {
    let dir = unsafe { data_dir.read() }.ok_or_else(|| Error::empty_arg(DATA_DIR_ARG))?;
    let dir_str = from_utf8(dir)?;
    // parse the supported features
    let features_bin =
        unsafe { supported_features.read() }.ok_or_else(|| Error::empty_arg(FEATURES_ARG))?;
    let features_str = from_utf8(features_bin)?;
    let features = features_from_csv(features_str);
    let cache = unsafe { CosmCache::new(dir_str, features) }?;
    let out = Box::new(cache);
    Ok(Box::into_raw(out))
}

/// frees a cache reference
///
/// # Safety
///
/// This must be called exactly once for any `*cache_t` returned by `init_cache`
/// and cannot be called on any other pointer.
#[no_mangle]
pub extern "C" fn release_cache(cache: *mut cache_t) {
    if !cache.is_null() {
        // this will free cache when it goes out of scope
        let _ = unsafe { Box::from_raw(cache as *mut CosmCache<DB, GoApi, GoQuerier>) };
    }
}

#[repr(C)]
pub struct EnclaveRuntimeConfig {
    pub module_cache_size: u32,
}

impl EnclaveRuntimeConfig {
    fn to_sgx_vm(&self) -> cosmwasm_sgx_vm::EnclaveRuntimeConfig {
        cosmwasm_sgx_vm::EnclaveRuntimeConfig {
            module_cache_size: self.module_cache_size,
        }
    }
}

#[no_mangle]
pub extern "C" fn configure_enclave_runtime(
    config: EnclaveRuntimeConfig,
    err: Option<&mut Buffer>,
) {
    let r = cosmwasm_sgx_vm::configure_enclave(config.to_sgx_vm())
        .map_err(|err| Error::enclave_err(err.to_string()));

    if let Err(e) = r {
        set_error(e, err);
    } else {
        clear_error();
    }
}

#[no_mangle]
pub extern "C" fn create(cache: *mut cache_t, wasm: Buffer, err: Option<&mut Buffer>) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_create(c, wasm)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::empty_arg(CACHE_ARG)),
    };
    let data = handle_c_error(r, err);
    Buffer::from_vec(data)
}

fn do_create(cache: &mut CosmCache<DB, GoApi, GoQuerier>, wasm: Buffer) -> Result<Checksum, Error> {
    let wasm = unsafe { wasm.read() }.ok_or_else(|| Error::empty_arg(WASM_ARG))?;
    let checksum = cache.save_wasm(wasm)?;
    Ok(checksum)
}

#[no_mangle]
pub extern "C" fn get_code(cache: *mut cache_t, id: Buffer, err: Option<&mut Buffer>) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_get_code(c, id)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::empty_arg(CACHE_ARG)),
    };
    let data = handle_c_error(r, err);
    Buffer::from_vec(data)
}

fn do_get_code(cache: &mut CosmCache<DB, GoApi, GoQuerier>, id: Buffer) -> Result<Vec<u8>, Error> {
    let id: Checksum = unsafe { id.read() }
        .ok_or_else(|| Error::empty_arg(CACHE_ARG))?
        .try_into()?;
    let wasm = cache.load_wasm(&id)?;
    Ok(wasm)
}

#[no_mangle]
pub extern "C" fn instantiate(
    cache: *mut cache_t,
    contract_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
    sig_info: Buffer,
    admin: Buffer,
) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_init(
                c,
                contract_id,
                params,
                msg,
                db,
                api,
                querier,
                gas_limit,
                gas_used,
                sig_info,
                admin,
            )
        }))
        .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::empty_arg(CACHE_ARG)),
    };
    let data = handle_c_error(r, err);
    Buffer::from_vec(data)
}

#[allow(clippy::too_many_arguments)]
fn do_init(
    cache: &mut CosmCache<DB, GoApi, GoQuerier>,
    code_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    gas_used: Option<&mut u64>,
    sig_info: Buffer,
    admin: Buffer,
) -> Result<Vec<u8>, Error> {
    let gas_used = gas_used.ok_or_else(|| Error::empty_arg(GAS_USED_ARG))?;
    let code_id: Checksum = unsafe { code_id.read() }
        .ok_or_else(|| Error::empty_arg(CODE_ID_ARG))?
        .try_into()?;
    let params = unsafe { params.read() }.ok_or_else(|| Error::empty_arg(PARAMS_ARG))?;
    let msg = unsafe { msg.read() }.ok_or_else(|| Error::empty_arg(MSG_ARG))?;
    let sig_info = unsafe { sig_info.read() }.ok_or_else(|| Error::empty_arg(SIG_INFO_ARG))?;
    let admin = unsafe { admin.read() }.unwrap_or_default();

    let deps = to_extern(db, api, querier);
    let mut instance = cache.get_instance(&code_id, deps, gas_limit)?;
    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = call_init_raw(&mut instance, params, msg, sig_info, admin);
    *gas_used = instance.create_gas_report().used_internally;
    instance.recycle();
    Ok(res?)
}

#[no_mangle]
pub extern "C" fn migrate(
    cache: *mut cache_t,
    contract_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
    sig_info: Buffer,
    admin: Buffer,
    admin_proof: Buffer,
) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_migrate(
                c,
                contract_id,
                params,
                msg,
                db,
                api,
                querier,
                gas_limit,
                gas_used,
                sig_info,
                admin,
                admin_proof,
            )
        }))
        .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::empty_arg(CACHE_ARG)),
    };
    let data = handle_c_error(r, err);
    Buffer::from_vec(data)
}

#[allow(clippy::too_many_arguments)]
fn do_migrate(
    cache: &mut CosmCache<DB, GoApi, GoQuerier>,
    code_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    gas_used: Option<&mut u64>,
    sig_info: Buffer,
    admin: Buffer,
    admin_proof: Buffer,
) -> Result<Vec<u8>, Error> {
    let gas_used = gas_used.ok_or_else(|| Error::empty_arg(GAS_USED_ARG))?;
    let code_id: Checksum = unsafe { code_id.read() }
        .ok_or_else(|| Error::empty_arg(CODE_ID_ARG))?
        .try_into()?;
    let params = unsafe { params.read() }.ok_or_else(|| Error::empty_arg(PARAMS_ARG))?;
    let msg = unsafe { msg.read() }.ok_or_else(|| Error::empty_arg(MSG_ARG))?;
    let sig_info = unsafe { sig_info.read() }.ok_or_else(|| Error::empty_arg(SIG_INFO_ARG))?;
    let admin = unsafe { admin.read() }.ok_or_else(|| Error::empty_arg(CURRENT_ADMIN_ARG))?;
    let admin_proof =
        unsafe { admin_proof.read() }.ok_or_else(|| Error::empty_arg(CURRENT_ADMIN_PROOF_ARG))?;

    let deps = to_extern(db, api, querier);
    let mut instance = cache.get_instance(&code_id, deps, gas_limit)?;
    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = call_migrate_raw(&mut instance, params, msg, sig_info, admin, admin_proof);
    *gas_used = instance.create_gas_report().used_internally;
    instance.recycle();
    Ok(res?)
}

#[no_mangle]
pub extern "C" fn update_admin(
    cache: *mut cache_t,
    contract_id: Buffer,
    params: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    err: Option<&mut Buffer>,
    sig_info: Buffer,
    current_admin: Buffer,
    current_admin_proof: Buffer,
    new_admin: Buffer,
) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_update_admin(
                c,
                contract_id,
                params,
                db,
                api,
                querier,
                gas_limit,
                sig_info,
                current_admin,
                current_admin_proof,
                new_admin,
            )
        }))
        .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::empty_arg(CACHE_ARG)),
    };
    let data = handle_c_error(r, err);
    Buffer::from_vec(data)
}

#[allow(clippy::too_many_arguments)]
fn do_update_admin(
    cache: &mut CosmCache<DB, GoApi, GoQuerier>,
    code_id: Buffer,
    params: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    sig_info: Buffer,
    current_admin: Buffer,
    current_admin_proof: Buffer,
    new_admin: Buffer,
) -> Result<Vec<u8>, Error> {
    let code_id: Checksum = unsafe { code_id.read() }
        .ok_or_else(|| Error::empty_arg(CODE_ID_ARG))?
        .try_into()?;
    let params = unsafe { params.read() }.ok_or_else(|| Error::empty_arg(PARAMS_ARG))?;
    let sig_info = unsafe { sig_info.read() }.ok_or_else(|| Error::empty_arg(SIG_INFO_ARG))?;
    let current_admin =
        unsafe { current_admin.read() }.ok_or_else(|| Error::empty_arg(CURRENT_ADMIN_ARG))?;
    let current_admin_proof = unsafe { current_admin_proof.read() }
        .ok_or_else(|| Error::empty_arg(CURRENT_ADMIN_PROOF_ARG))?;
    let new_admin = unsafe { new_admin.read() }.unwrap_or(&[]);

    let deps = to_extern(db, api, querier);
    let mut instance = cache.get_instance(&code_id, deps, gas_limit)?;
    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = call_update_admin_raw(
        &mut instance,
        params,
        sig_info,
        current_admin,
        current_admin_proof,
        new_admin,
    );
    instance.recycle();
    Ok(res?)
}

#[no_mangle]
pub extern "C" fn handle(
    cache: *mut cache_t,
    code_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
    sig_info: Buffer,
    handle_type: u8,
) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_handle(
                c,
                code_id,
                params,
                msg,
                db,
                api,
                querier,
                gas_limit,
                gas_used,
                sig_info,
                handle_type,
            )
        }))
        .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::empty_arg(CACHE_ARG)),
    };
    let data = handle_c_error(r, err);
    Buffer::from_vec(data)
}

#[allow(clippy::too_many_arguments)]
fn do_handle(
    cache: &mut CosmCache<DB, GoApi, GoQuerier>,
    code_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    gas_used: Option<&mut u64>,
    sig_info: Buffer,
    handle_type: u8,
) -> Result<Vec<u8>, Error> {
    let gas_used = gas_used.ok_or_else(|| Error::empty_arg(GAS_USED_ARG))?;
    let code_id: Checksum = unsafe { code_id.read() }
        .ok_or_else(|| Error::empty_arg(CODE_ID_ARG))?
        .try_into()?;
    let params = unsafe { params.read() }.ok_or_else(|| Error::empty_arg(PARAMS_ARG))?;
    let msg = unsafe { msg.read() }.ok_or_else(|| Error::empty_arg(MSG_ARG))?;
    let sig_info = unsafe { sig_info.read() }.ok_or_else(|| Error::empty_arg(SIG_INFO_ARG))?;

    let deps = to_extern(db, api, querier);
    let mut instance = cache.get_instance(&code_id, deps, gas_limit)?;
    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = call_handle_raw(&mut instance, params, msg, sig_info, handle_type);
    *gas_used = instance.create_gas_report().used_internally;
    instance.recycle();
    Ok(res?)
}

#[no_mangle]
pub extern "C" fn query(
    cache: *mut cache_t,
    code_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    gas_used: Option<&mut u64>,
    err: Option<&mut Buffer>,
) -> Buffer {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || {
            do_query(
                c, code_id, params, msg, db, api, querier, gas_limit, gas_used,
            )
        }))
        .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::empty_arg(CACHE_ARG)),
    };
    let data = handle_c_error(r, err);
    Buffer::from_vec(data)
}

fn do_query(
    cache: &mut CosmCache<DB, GoApi, GoQuerier>,
    code_id: Buffer,
    params: Buffer,
    msg: Buffer,
    db: DB,
    api: GoApi,
    querier: GoQuerier,
    gas_limit: u64,
    gas_used: Option<&mut u64>,
) -> Result<Vec<u8>, Error> {
    let gas_used = gas_used.ok_or_else(|| Error::empty_arg(GAS_USED_ARG))?;
    let code_id: Checksum = unsafe { code_id.read() }
        .ok_or_else(|| Error::empty_arg(CODE_ID_ARG))?
        .try_into()?;
    let params = unsafe { params.read() }.ok_or_else(|| Error::empty_arg(PARAMS_ARG))?;
    let msg = unsafe { msg.read() }.ok_or_else(|| Error::empty_arg(MSG_ARG))?;

    let deps = to_extern(db, api, querier);
    let mut instance = cache.get_instance(&code_id, deps, gas_limit)?;
    // We only check this result after reporting gas usage and returning the instance into the cache.
    let res = call_query_raw(&mut instance, params, msg);
    *gas_used = instance.create_gas_report().used_internally;
    instance.recycle();
    Ok(res?)
}

/// The result type of the FFI function analyze_code.
///
/// Please note that the unmanaged vector in `required_features`
/// has to be destroyed exactly once. When calling `analyze_code`
/// from Go this is done via `C.destroy_unmanaged_vector`.
#[repr(C)]
#[derive(Clone, Default, Debug, PartialEq)]
pub struct AnalysisReport {
    pub has_ibc_entry_points: bool,
    /// An UTF-8 encoded comma separated list of required features.
    /// This is never None/nil.
    pub required_features: Buffer,
}

#[no_mangle]
pub extern "C" fn analyze_code(
    cache: *mut cache_t,
    checksum: Buffer,
    error_msg: Option<&mut Buffer>,
) -> AnalysisReport {
    let r = match to_cache(cache) {
        Some(c) => catch_unwind(AssertUnwindSafe(move || do_analyze_code(c, checksum)))
            .unwrap_or_else(|_| Err(Error::panic())),
        None => Err(Error::empty_arg(CACHE_ARG)),
    };

    handle_c_error_default(r, error_msg)
}

fn do_analyze_code(
    cache: &mut CosmCache<DB, GoApi, GoQuerier>,
    checksum: Buffer,
) -> Result<AnalysisReport, Error> {
    let checksum: Checksum = unsafe { checksum.read() }
        .ok_or_else(|| Error::empty_arg(CODE_ID_ARG))?
        .try_into()?;
    let report = cache.analyze(&checksum)?;
    let mut features_vec: Vec<u8> = vec![];
    for feature in &report.required_features {
        if features_vec.len() > 0 {
            features_vec.append(&mut (",".as_bytes().to_vec()))
        }

        features_vec.append(&mut feature.as_bytes().to_vec())
    }

    Ok(AnalysisReport {
        has_ibc_entry_points: report.has_ibc_entry_points,
        required_features: Buffer::from_vec(features_vec),
    })
}

#[no_mangle]
pub extern "C" fn key_gen(err: Option<&mut Buffer>) -> Buffer {
    match untrusted_key_gen() {
        Err(e) => {
            set_error(Error::enclave_err(e.to_string()), err);
            Buffer::default()
        }
        Ok(r) => {
            clear_error();
            Buffer::from_vec(r.to_vec())
        }
    }
}

#[no_mangle]
pub extern "C" fn migrate_sealing() -> bool {
    if let Err(e) = untrusted_migrate_sealing() {
        error!("migrate_sealing error: {}", e);
        return false;
    }

    true
}
