use std::sync::SgxRwLock;

use lazy_static::lazy_static;
use log::*;
use lru::LruCache;

use parity_wasm::elements;
use parity_wasm::elements::Module;
use wasmi::{ModuleInstance, ModuleRef};

use enclave_ffi_types::EnclaveError;

use enclave_cosmos_types::types::ContractCode;
use enclave_crypto::HASH_SIZE;

use super::gas::{gas_rules, WasmCosts};
use super::memory::validate_memory;
use super::wasm::{create_builder, ContractOperation, WasmiImportResolver};

lazy_static! {
    static ref MODULE_CACHE: SgxRwLock<LruCache<[u8; HASH_SIZE], wasmi::Module>> =
        SgxRwLock::new(LruCache::new(0));
}

pub fn configure_module_cache(cap: usize) {
    debug!("configuring module cache: {}", cap);
    MODULE_CACHE.write().unwrap().resize(cap)
}

pub fn create_module_instance(
    contract_code: ContractCode,
    operation: ContractOperation,
) -> Result<ModuleRef, EnclaveError> {
    debug!("fetching module from cache");
    let cache = MODULE_CACHE.read().unwrap();

    // If the cache is disabled, don't try to use it and just compile the module.
    if cache.cap() == 0 {
        debug!("cache is disabled, building module");
        drop(cache);
        let module = compile_module(contract_code.code(), operation)?;
        let instance = create_instance(&module)?;
        debug!("returning built instance");
        return Ok(instance);
    }
    debug!("cache is enabled");

    // Try to fetch a cached instance
    let mut instance = None;
    debug!("peeking in cache");
    let instance_result = cache.peek(&contract_code.hash()).map(create_instance);
    // If the stored module failed to create an instance for some reason, we try to create it again.
    // It shouldn't happen because we already compiled it before.
    if let Some(Ok(cached_instance)) = instance_result {
        debug!("found instance in cache!");
        instance = Some(cached_instance)
    }

    drop(cache); // Release read lock

    // If we couldn't find the instance in the cache, create it
    let mut module = None;
    if instance.is_none() {
        debug!("instance not found in cache. building a new one");
        let new_module = compile_module(contract_code.code(), operation)?;
        let new_instance = create_instance(&new_module)?;
        module = Some(new_module);
        instance = Some(new_instance);
    }
    let instance = instance.unwrap(); // We definitely have a value here now

    // If we created a new module in the previous step, insert it to the LRU cache
    debug!("updating cache");
    let mut cache = MODULE_CACHE.write().unwrap();
    if let Some(module) = module {
        debug!("storing module in cache");
        cache.put(contract_code.hash(), module);
    } else {
        // Touch the cache to update the LRU value
        debug!("updating LRU without storing anything");
        cache.get(&contract_code.hash());
    }

    debug!("returning built instance");
    Ok(instance)
}

// The compilation steps in this section are very expensive, and generate a
// static object that can be reused without leaking memories between contracts.
// This is why we separate the compilation step and cache its result in memory.
fn compile_module(
    code: &[u8],
    operation: ContractOperation,
) -> Result<wasmi::Module, EnclaveError> {
    info!("Deserializing Wasm contract");

    // Create a parity-wasm module first, so we can inject gas metering to it
    // (you need a parity-wasm module to use the pwasm-utils crate)
    let mut p_modlue: Module =
        elements::deserialize_buffer(code).map_err(|_| EnclaveError::InvalidWasm)?;

    info!("Deserialized Wasm contract");

    info!("Validating WASM memory demands");

    validate_memory(&mut p_modlue)?;

    info!("Validated WASM memory demands");

    // Set the gas costs for wasm op-codes (there is an inline stack_height limit in WasmCosts)
    let wasm_costs = WasmCosts::default();

    // Inject gas metering to pwasm module
    let contract_module = pwasm_utils::inject_gas_counter(p_modlue, &gas_rules(&wasm_costs))
        .map_err(|_| EnclaveError::FailedGasMeteringInjection)?;

    info!("Trying to create Wasmi module from parity...");

    // Create a wasmi module from the parity module
    let module = wasmi::Module::from_parity_wasm_module(contract_module)
        .map_err(|_err| EnclaveError::InvalidWasm)?;

    info!("Created Wasmi module from parity. Now checking for floating points...");

    // Skip the floating point check in queries and handles.
    // We know that the contract must be valid at this point,
    // otherwise the contrat storage keys will be invalid, and this
    // operation is extremely expensive (21-27ms in testing)
    if let ContractOperation::Init = operation {
        module
            .deny_floating_point()
            .map_err(|_err| EnclaveError::WasmModuleWithFP)?;
    }

    Ok(module)
}

fn create_instance(module: &wasmi::Module) -> Result<ModuleRef, EnclaveError> {
    // Create new imports resolver.
    // These are the signatures of rust functions available to invoke from wasm code.
    let resolver = WasmiImportResolver {};
    let imports_builder = create_builder(&resolver);

    // Instantiate a module with our imports and assert that there is no `start` function.
    let module_instance = ModuleInstance::new(module, &imports_builder).map_err(|err| {
        warn!("Error in instantiation: {:?}", err);
        EnclaveError::InvalidWasm
    })?;
    if module_instance.has_start() {
        return Err(EnclaveError::WasmModuleWithStart);
    }
    Ok(module_instance.not_started_instance().clone())
}
