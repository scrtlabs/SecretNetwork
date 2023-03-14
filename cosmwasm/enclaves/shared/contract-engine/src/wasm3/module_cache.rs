use std::sync::SgxRwLock;

use lazy_static::lazy_static;
use log::*;
use lru::LruCache;

use cw_types_generic::{ContractFeature, CosmWasmApiVersion};

use enclave_ffi_types::EnclaveError;

use enclave_cosmos_types::types::ContractCode;
use enclave_crypto::HASH_SIZE;

use super::{gas, validation};
use crate::cosmwasm_config::ContractOperation;
use crate::cosmwasm_config::{api_marker, features};
use crate::gas::WasmCosts;

pub struct VersionedCode {
    pub code: Vec<u8>,
    pub version: CosmWasmApiVersion,
    pub features: Vec<ContractFeature>,
}

impl VersionedCode {
    pub fn new(code: Vec<u8>, version: CosmWasmApiVersion, features: Vec<ContractFeature>) -> Self {
        Self {
            code,
            version,
            features,
        }
    }
}

lazy_static! {
    static ref MODULE_CACHE: SgxRwLock<LruCache<[u8; HASH_SIZE], VersionedCode>> =
        SgxRwLock::new(LruCache::new(0));
}

pub fn configure_module_cache(cap: usize) {
    debug!("configuring module cache: {}", cap);
    MODULE_CACHE.write().unwrap().resize(cap)
}

pub fn create_module_instance(
    contract_code: &ContractCode,
    gas_costs: &WasmCosts,
    operation: ContractOperation,
) -> Result<VersionedCode, EnclaveError> {
    debug!("fetching module from cache");
    let cache = MODULE_CACHE.read().unwrap();

    // If the cache is disabled, don't try to use it and just compile the module.
    if cache.cap() == 0 {
        debug!("cache is disabled, building module");
        return analyze_module(contract_code, gas_costs, operation);
    }
    debug!("cache is enabled");

    // Try to fetch a cached instance
    let mut code = None;
    let mut api_version = CosmWasmApiVersion::Invalid;
    let mut features = vec![];
    debug!("peeking in cache");
    let peek_result = cache.peek(&contract_code.hash());
    if let Some(VersionedCode {
        code: cached_code,
        version: cached_ver,
        features: cached_features,
    }) = peek_result
    {
        debug!("found instance in cache!");
        code = Some(cached_code.clone());
        api_version = *cached_ver;
        features = cached_features.clone();
    }

    drop(cache); // Release read lock

    // if we couldn't find the code in the cache, analyze it now
    if code.is_none() {
        debug!("code not found in cache! analyzing now");
        let versioned_code = analyze_module(contract_code, gas_costs, operation)?;
        code = Some(versioned_code.code);
        api_version = versioned_code.version;
        features = versioned_code.features;
    }

    // If we analyzed the code in the previous step, insert it to the LRU cache
    debug!("updating cache");
    let mut cache = MODULE_CACHE.write().unwrap();
    if let Some(code) = code.clone() {
        debug!("storing code in cache");
        cache.put(
            contract_code.hash(),
            VersionedCode::new(code, api_version, features.clone()),
        );
    } else {
        // Touch the cache to update the LRU value
        debug!("updating LRU without storing anything");
        cache.get(&contract_code.hash());
    }

    let code = code.unwrap();

    debug!("returning built instance");
    Ok(VersionedCode::new(code, api_version, features))
}

pub fn analyze_module(
    contract_code: &ContractCode,
    gas_costs: &WasmCosts,
    operation: ContractOperation,
) -> Result<VersionedCode, EnclaveError> {
    let mut module = walrus::ModuleConfig::new()
        .generate_producers_section(false)
        .parse(contract_code.code())
        .map_err(|_| EnclaveError::InvalidWasm)?;

    for import in module.imports.iter() {
        trace!("import {:?}", import)
    }
    for export in module.exports.iter() {
        trace!("export {:?}", export)
    }

    use walrus::Export;
    let mut exports = module.exports.iter();
    let marker_export =
        exports.find(|&exp| exp.name == api_marker::V0_10 || exp.name == api_marker::V1);
    let cosmwasm_api_version = match marker_export {
        Some(Export { name, .. }) if name == api_marker::V0_10 => CosmWasmApiVersion::V010,
        Some(Export { name, .. }) if name == api_marker::V1 => CosmWasmApiVersion::V1,
        _ => {
            error!("Invalid cosmwasm api version2");
            return Err(EnclaveError::InvalidWasm);
        }
    };

    // features
    let random_enabled = module
        .exports
        .iter()
        .find(|&exp| exp.name == features::RANDOM)
        .is_some();

    let features = if random_enabled {
        debug!("Found supported features: random");
        vec![ContractFeature::Random]
    } else {
        vec![]
    };
    drop(exports);

    validation::validate_memory(&mut module)?;

    if let ContractOperation::Init = operation {
        if module.has_floats() {
            debug!("contract was found to contain floating point operations");
            return Err(EnclaveError::WasmModuleWithFP);
        }
    }

    gas::add_metering(&mut module, gas_costs);

    let code = module.emit_wasm();

    Ok(VersionedCode::new(code, cosmwasm_api_version, features))
}
