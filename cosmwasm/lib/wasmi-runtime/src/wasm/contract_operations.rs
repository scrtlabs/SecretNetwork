use log::*;
use parity_wasm::elements;
use wasmi::{ImportsBuilder, ModuleInstance};

use enclave_ffi_types::{Ctx, EnclaveError};

use crate::cosmwasm::types::Env;
use crate::results::{HandleSuccess, InitSuccess, QuerySuccess};
use crate::wasm::contract_validation::ContractKey;

use super::contract_validation::{
    calc_contract_hash, extract_contract_key, generate_contract_id, generate_encryption_key,
    generate_sender_id, validate_contract_key, CONTRACT_KEY_LENGTH,
};
use super::errors::wasmi_error_to_enclave_error;
use super::gas::{gas_rules, WasmCosts};
use super::io::{decrypt_msg, encrypt_output};
use super::runtime::{Engine, EnigmaImportResolver, Runtime};

/*
Each contract is compiled with these functions alreadyy implemented in wasm:
fn cosmwasm_api_0_6() -> i32;  // Seems unused, but we should support it anyways
fn allocate(size: usize) -> *mut c_void;
fn deallocate(pointer: *mut c_void);
fn init(env_ptr: *mut c_void, msg_ptr: *mut c_void) -> *mut c_void
fn handle(env_ptr: *mut c_void, msg_ptr: *mut c_void) -> *mut c_void
fn query(msg_ptr: *mut c_void) -> *mut c_void

Re `init`, `handle` and `query`: We need to pass `env` & `msg`
down to the wasm implementations, but because they are buffers
we need to allocate memory regions inside the VM's instance and copy
`env` & `msg` into those memory regions inside the VM's instance.
*/

pub fn init(
    context: Ctx,    // need to pass this to read_db & write_db
    gas_limit: u64,  // gas limit for this execution
    contract: &[u8], // contract wasm bytes
    env: &[u8],      // blockchain state
    msg: &[u8],      // probably function call and args
) -> Result<InitSuccess, EnclaveError> {
    let parsed_env: Env = serde_json::from_slice(env).map_err(|err| {
        error!(
            "got an error while trying to deserialize output bytes into json {:?}: {}",
            env, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    let contract_key = generate_encryption_key(&parsed_env, contract)?;

    info!("Init: Contract Key: {:?}", contract_key.to_vec().as_slice());

    let mut engine = start_engine(context, gas_limit, contract, &contract_key)?;

    let env_ptr = engine
        .write_to_memory(env)
        .map_err(wasmi_error_to_enclave_error)?;

    let (msg, user_pubkey, nonce) = decrypt_msg(msg)?;

    let msg_ptr = engine
        .write_to_memory(&msg)
        .map_err(wasmi_error_to_enclave_error)?;

    let vec_ptr = engine
        .init(env_ptr, msg_ptr)
        .map_err(wasmi_error_to_enclave_error)?;

    let output = engine
        .extract_vector(vec_ptr)
        .map_err(wasmi_error_to_enclave_error)?;

    let output = encrypt_output(&output, &user_pubkey, &nonce)?;

    // third time's the charm
    // let output = append_contract_key(&output, encryption_key)?;

    Ok(InitSuccess {
        output,
        used_gas: engine.gas_used(),
        signature: contract_key, // TODO this is needed anymore as output is already authenticated
    })
}

pub fn handle(
    context: Ctx,
    gas_limit: u64,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
) -> Result<HandleSuccess, EnclaveError> {
    let parsed_env: Env = serde_json::from_slice(env).map_err(|err| {
        error!(
            "got an error while trying to deserialize output bytes into json {:?}: {}",
            env, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    debug!("handle parsed_envs: {:?}", parsed_env);

    let contract_key = extract_contract_key(&parsed_env)?;

    if !validate_contract_key(&contract_key, contract) {
        error!("got an error while trying to deserialize output bytes");
        return Err(EnclaveError::FailedContractAuthentication);
    }

    debug!("Successfully authenticated the contract!");

    info!(
        "Handle: Contract Key: {:?}",
        contract_key.to_vec().as_slice()
    );

    let mut engine = start_engine(context, gas_limit, contract, &contract_key)?;

    let env_ptr = engine
        .write_to_memory(env)
        .map_err(wasmi_error_to_enclave_error)?;

    let (msg, user_pubkey, nonce) = decrypt_msg(msg)?;

    let msg_ptr = engine
        .write_to_memory(&msg)
        .map_err(wasmi_error_to_enclave_error)?;

    let vec_ptr = engine
        .handle(env_ptr, msg_ptr)
        .map_err(wasmi_error_to_enclave_error)?;

    let output = engine
        .extract_vector(vec_ptr)
        .map_err(wasmi_error_to_enclave_error)?;

    let output = encrypt_output(&output, &user_pubkey, &nonce)?;

    Ok(HandleSuccess {
        output,
        used_gas: engine.gas_used(),
        signature: [0u8; 64], // TODO this is needed anymore as output is already authenticated
    })
}

pub fn query(
    context: Ctx,
    gas_limit: u64,
    contract: &[u8],
    msg: &[u8],
) -> Result<QuerySuccess, EnclaveError> {
    if msg.len() < CONTRACT_KEY_LENGTH {
        error!("Input query is shorter than the minimum expected. Msg is malformed");
        return Err(EnclaveError::FailedFunctionCall);
    }

    let (key, msg) = msg.split_at(CONTRACT_KEY_LENGTH);

    let mut contract_key = [0; CONTRACT_KEY_LENGTH];
    contract_key.copy_from_slice(key);

    info!(
        "Query: Contract Key: {:?}",
        contract_key.to_vec().as_slice()
    );

    let mut engine = start_engine(context, gas_limit, contract, &contract_key)?;

    let (msg, user_pubkey, nonce) = decrypt_msg(msg)?;

    let msg_ptr = engine
        .write_to_memory(&msg)
        .map_err(wasmi_error_to_enclave_error)?;

    let vec_ptr = engine
        .query(msg_ptr)
        .map_err(wasmi_error_to_enclave_error)?;

    let output = engine
        .extract_vector(vec_ptr)
        .map_err(wasmi_error_to_enclave_error)?;

    let output = encrypt_output(&output, &user_pubkey, &nonce)?;

    Ok(QuerySuccess {
        output,
        used_gas: engine.gas_used(),
        signature: [0; 64], // TODO this is needed anymore as output is already authenticated
    })
}

fn start_engine(
    context: Ctx,
    gas_limit: u64,
    contract: &[u8],
    contract_key: &ContractKey,
) -> Result<Engine, EnclaveError> {
    // Create a parity-wasm module first, so we can inject gas metering to it
    // (you need a parity-wasm module to use the pwasm-utils crate)
    let p_modlue = elements::deserialize_buffer(contract).map_err(|_| EnclaveError::InvalidWasm)?;

    // Set the gas costs for wasm op-codes (there is an inline stack_height limit in WasmCosts)
    let wasm_costs = WasmCosts::default();

    // Inject gas metering to pwasm module
    let contract_module = pwasm_utils::inject_gas_counter(p_modlue, &gas_rules(&wasm_costs))
        .map_err(|_| EnclaveError::FailedGasMeteringInjection)?;

    // Create a wasmi module from the parity module
    let module = wasmi::Module::from_parity_wasm_module(contract_module)
        .map_err(|_err| EnclaveError::InvalidWasm)?;

    module
        .deny_floating_point()
        .map_err(|_err| EnclaveError::WasmModuleWithFP)?;

    // Create new imports resolver.
    // These are the signatures of rust functions available to invoke from wasm code.
    let imports = EnigmaImportResolver {};
    let module_imports = ImportsBuilder::new().with_resolver("env", &imports);

    // Instantiate a module with our imports and assert that there is no `start` function.
    let instance =
        ModuleInstance::new(&module, &module_imports).map_err(|_err| EnclaveError::InvalidWasm)?;
    if instance.has_start() {
        return Err(EnclaveError::WasmModuleWithStart);
    }
    let instance = instance.not_started_instance().clone();

    let runtime = Runtime::new(
        context,
        instance
            .export_by_name("memory")
            .expect("Module expected to have 'memory' export")
            .as_memory()
            .cloned()
            .expect("'memory' export should be of memory type"),
        gas_limit,
        contract_key.clone(),
    );

    Ok(Engine::new(runtime, instance))
}
