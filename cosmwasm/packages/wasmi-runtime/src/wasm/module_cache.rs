use std::sync::SgxRwLock;

use lazy_static::lazy_static;
use log::*;
use lru::LruCache;
use parity_wasm::elements;
use parity_wasm::elements::Module;
use wasmi::{ModuleInstance, ModuleRef};

use enclave_ffi_types::EnclaveError;

use crate::crypto::HASH_SIZE;
use crate::wasm::types::ContractCode;

use super::gas::{gas_rules, WasmCosts};
use super::memory::validate_memory;
use super::runtime::{create_builder, WasmiImportResolver};

lazy_static! {
    static ref MODULE_CACHE: SgxRwLock<LruCache<[u8; HASH_SIZE], wasmi::Module>> =
        SgxRwLock::new(LruCache::new(0));
}

pub fn configure_module_cache(cap: usize) {
    debug!("configuring module cache: {}", cap);
    MODULE_CACHE.write().unwrap().resize(cap)
}

pub fn create_module_instance(contract_code: ContractCode) -> Result<ModuleRef, EnclaveError> {
    let code_hash = contract_code.hash();

    // Update the LRU cache as quickly as possible so it knows this was recently used
    MODULE_CACHE.write().unwrap().get(&code_hash);

    match get_module_instance(&code_hash) {
        Some(Ok(module_ref)) => return Ok(module_ref),
        None => {} // continue

        // If the stored module failed to process for some reason, remove it.
        // Shouldn't happen because we already compiled it before.
        Some(Err(_)) => {
            MODULE_CACHE.write().unwrap().pop(&code_hash);
        }
    }

    let mut cache = MODULE_CACHE.write().unwrap();
    match cache.get(&code_hash).map(create_instance) {
        Some(Ok(module_ref)) => return Ok(module_ref),
        None => {} // continue

        // If the stored module failed to process for some reason, remove it.
        // Shouldn't happen because we already compiled it before.
        Some(Err(_)) => {
            cache.pop(&code_hash);
        }
    }

    let module = compile_module(contract_code.code())?;
    let instance = create_instance(&module)?;
    cache.put(code_hash, module);
    Ok(instance)
}

// This is a separate function for scoping, so that we don't hold the read/write handle
// for too long
fn get_module_instance(code_hash: &[u8; HASH_SIZE]) -> Option<Result<ModuleRef, EnclaveError>> {
    // Note that this peek doesn't update the LRU cache
    MODULE_CACHE
        .read()
        .unwrap()
        .peek(code_hash)
        .map(create_instance)
}

// The compilation steps in this section are very expensive, and generate a
// static object that can be reused without leaking memories between contracts.
// This is why we separate the compilation step and cache its result in memory.
fn compile_module(code: &[u8]) -> Result<wasmi::Module, EnclaveError> {
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

    module
        .deny_floating_point()
        .map_err(|_err| EnclaveError::WasmModuleWithFP)?;

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
