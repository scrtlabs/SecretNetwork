use log::*;
use parity_wasm::elements;
use parity_wasm::elements::Module;
use wasmi::ModuleInstance;

use enclave_ffi_types::{Ctx, EnclaveError};

use crate::coalesce;
use crate::cosmwasm::types::Env;
use crate::crypto::Ed25519PublicKey;
use crate::results::{HandleSuccess, InitSuccess, QuerySuccess};
use crate::wasm::contract_validation::ContractKey;
use crate::wasm::types::{IoNonce, SecretMessage};

use super::contract_validation::{
    calc_contract_hash, extract_contract_key, generate_encryption_key, validate_contract_key,
    validate_msg, CONTRACT_KEY_LENGTH,
};
use super::gas::{gas_rules, WasmCosts};
use super::io::encrypt_output;
use super::{
    memory::validate_memory,
    runtime::{create_builder, ContractInstance, ContractOperation, Engine, WasmiImportResolver},
};

/*
Each contract is compiled with these functions already implemented in wasm:
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
    context: Ctx,       // need to pass this to read_db & write_db
    gas_limit: u64,     // gas limit for this execution
    used_gas: &mut u64, // out-parameter for gas used in execution
    contract: &[u8],    // contract wasm bytes
    env: &[u8],         // blockchain state
    msg: &[u8],         // probably function call and args
) -> Result<InitSuccess, EnclaveError> {
    let mut parsed_env: Env = serde_json::from_slice(env).map_err(|err| {
        error!(
            "got an error while trying to deserialize env input bytes into json {:?}: {}",
            env, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    let contract_address = &parsed_env.contract.address;
    let contract_key = generate_encryption_key(&parsed_env, contract, contract_address.as_slice())?;

    trace!("Init: Contract Key: {:?}", contract_key.to_vec().as_slice());

    let secret_msg = SecretMessage::from_slice(msg)?;
    trace!(
        "Init input before decryption: {:?}",
        String::from_utf8_lossy(&msg)
    );

    let decrypted_msg = secret_msg.decrypt()?;

    let validated_msg = validate_msg(&decrypted_msg, contract)?;

    trace!(
        "Init input afer decryption: {:?}",
        String::from_utf8_lossy(&validated_msg)
    );

    let mut engine = start_engine(
        context,
        gas_limit,
        contract,
        &contract_key,
        ContractOperation::Init,
        secret_msg.nonce,
        secret_msg.user_public_key,
    )?;

    parsed_env.contract_code_hash = Some(hex::encode(calc_contract_hash(contract)));

    let new_env = serde_json::to_vec(&parsed_env).map_err(|err| {
        error!(
            "got an error while trying to serialize parsed_env into bytes {:?}: {}",
            parsed_env, err
        );
        EnclaveError::FailedToSerialize
    })?;

    let env_ptr = engine.write_to_memory(&new_env)?;
    let msg_ptr = engine.write_to_memory(&validated_msg)?;

    // This wrapper is used to coalesce all errors in this block to one object
    // so we can `.map_err()` in one place for all of them
    let output = coalesce!(EnclaveError, {
        let vec_ptr = engine.init(env_ptr, msg_ptr)?;
        let output = engine.extract_vector(vec_ptr)?;
        // TODO: copy cosmwasm's structures to enclave
        // TODO: ref: https://github.com/CosmWasm/cosmwasm/blob/b971c037a773bf6a5f5d08a88485113d9b9e8e7b/packages/std/src/init_handle.rs#L129
        // TODO: ref: https://github.com/CosmWasm/cosmwasm/blob/b971c037a773bf6a5f5d08a88485113d9b9e8e7b/packages/std/src/query.rs#L13
        let output = encrypt_output(output, secret_msg.nonce, secret_msg.user_public_key)?;
        Ok(output)
    })
    .map_err(|err| {
        *used_gas = engine.gas_used();
        err
    })?;

    *used_gas = engine.gas_used();
    // todo: can move the key to somewhere in the output message if we want

    Ok(InitSuccess {
        output,
        signature: contract_key,
    })
}

pub fn handle(
    context: Ctx,
    gas_limit: u64,
    used_gas: &mut u64,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
) -> Result<HandleSuccess, EnclaveError> {
    let mut parsed_env: Env = serde_json::from_slice(env).map_err(|err| {
        error!(
            "got an error while trying to deserialize env input bytes into json {:?}: {}",
            env, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    trace!("handle parsed_envs: {:?}", parsed_env);

    let contract_address = &parsed_env.contract.address;
    let contract_key = extract_contract_key(&parsed_env)?;

    trace!(
        "Handle input before decryption: {:?}",
        String::from_utf8_lossy(&msg)
    );

    let secret_msg = SecretMessage::from_slice(msg)?;
    let decrypted_msg = secret_msg.decrypt()?;

    let validated_msg = validate_msg(&decrypted_msg, contract)?;

    trace!(
        "Handle input afer decryption: {:?}",
        String::from_utf8_lossy(&validated_msg)
    );

    if !validate_contract_key(&contract_key, contract_address.as_slice(), contract) {
        error!("got an error while trying to deserialize output bytes");
        return Err(EnclaveError::FailedContractAuthentication);
    }

    trace!("Successfully authenticated the contract!");

    trace!(
        "Handle: Contract Key: {:?}",
        contract_key.to_vec().as_slice()
    );

    let mut engine = start_engine(
        context,
        gas_limit,
        contract,
        &contract_key,
        ContractOperation::Handle,
        secret_msg.nonce,
        secret_msg.user_public_key,
    )?;

    parsed_env.contract_code_hash = Some(hex::encode(calc_contract_hash(contract)));

    let new_env = serde_json::to_vec(&parsed_env).map_err(|err| {
        error!(
            "got an error while trying to serialize parsed_env into bytes {:?}: {}",
            parsed_env, err
        );
        EnclaveError::FailedToSerialize
    })?;

    let env_ptr = engine.write_to_memory(&new_env)?;
    let msg_ptr = engine.write_to_memory(&validated_msg)?;

    // This wrapper is used to coalesce all errors in this block to one object
    // so we can `.map_err()` in one place for all of them
    let output = coalesce!(EnclaveError, {
        let vec_ptr = engine.handle(env_ptr, msg_ptr)?;

        let output = engine.extract_vector(vec_ptr)?;

        debug!(
            "(2) nonce just before encrypt_output: nonce = {:?} pubkey = {:?}",
            secret_msg.nonce, secret_msg.user_public_key
        );
        let output = encrypt_output(output, secret_msg.nonce, secret_msg.user_public_key)?;
        Ok(output)
    })
    .map_err(|err| {
        *used_gas = engine.gas_used();
        err
    })?;

    *used_gas = engine.gas_used();
    Ok(HandleSuccess {
        output,
        signature: [0u8; 64], // TODO this is not needed anymore as output is already authenticated
    })
}

pub fn query(
    context: Ctx,
    gas_limit: u64,
    used_gas: &mut u64,
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

    trace!(
        "Query: Contract Key: {:?}",
        contract_key.to_vec().as_slice()
    );

    let secret_msg = SecretMessage::from_slice(msg)?;
    trace!(
        "Query input before decryption: {:?}",
        String::from_utf8_lossy(&msg)
    );
    let decrypted_msg = secret_msg.decrypt()?;
    trace!(
        "Query input afer decryption: {:?}",
        String::from_utf8_lossy(&decrypted_msg)
    );
    let validated_msg = validate_msg(&decrypted_msg, contract)?;

    let mut engine = start_engine(
        context,
        gas_limit,
        contract,
        &contract_key,
        ContractOperation::Query,
        secret_msg.nonce,
        secret_msg.user_public_key,
    )?;

    let msg_ptr = engine.write_to_memory(&validated_msg)?;

    // This wrapper is used to coalesce all errors in this block to one object
    // so we can `.map_err()` in one place for all of them
    let output = coalesce!(EnclaveError, {
        let vec_ptr = engine.query(msg_ptr)?;

        let output = engine.extract_vector(vec_ptr)?;

        let output = encrypt_output(output, secret_msg.nonce, secret_msg.user_public_key)?;
        Ok(output)
    })
    .map_err(|err| {
        *used_gas = engine.gas_used();
        err
    })?;

    *used_gas = engine.gas_used();
    Ok(QuerySuccess {
        output,
        signature: [0; 64], // TODO this is not needed anymore as output is already authenticated
    })
}

fn start_engine(
    context: Ctx,
    gas_limit: u64,
    contract: &[u8],
    contract_key: &ContractKey,
    operation: ContractOperation,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
) -> Result<Engine, EnclaveError> {
    trace!("Deserializing Wasm contract");

    // Create a parity-wasm module first, so we can inject gas metering to it
    // (you need a parity-wasm module to use the pwasm-utils crate)
    let mut p_modlue: Module =
        elements::deserialize_buffer(contract).map_err(|_| EnclaveError::InvalidWasm)?;

    trace!("Deserialized Wasm contract");

    trace!("Validating WASM memory demands");

    validate_memory(&mut p_modlue)?;

    trace!("Validated WASM memory demands");

    // Set the gas costs for wasm op-codes (there is an inline stack_height limit in WasmCosts)
    let wasm_costs = WasmCosts::default();

    // Inject gas metering to pwasm module
    let contract_module = pwasm_utils::inject_gas_counter(p_modlue, &gas_rules(&wasm_costs))
        .map_err(|_| EnclaveError::FailedGasMeteringInjection)?;

    trace!("Trying to create Wasmi module from parity...");

    // Create a wasmi module from the parity module
    let module = wasmi::Module::from_parity_wasm_module(contract_module)
        .map_err(|_err| EnclaveError::InvalidWasm)?;

    trace!("Created Wasmi module from parity. Now checking for floating points...");

    module
        .deny_floating_point()
        .map_err(|_err| EnclaveError::WasmModuleWithFP)?;

    // Create new imports resolver.
    // These are the signatures of rust functions available to invoke from wasm code.
    let resolver = WasmiImportResolver {};
    let imports_builder = create_builder(&resolver);

    // Instantiate a module with our imports and assert that there is no `start` function.
    let module_instance = ModuleInstance::new(&module, &imports_builder).map_err(|err| {
        error!("Error in instantiation: {:?}", err);
        EnclaveError::InvalidWasm
    })?;
    if module_instance.has_start() {
        return Err(EnclaveError::WasmModuleWithStart);
    }
    let module = module_instance.not_started_instance().clone();

    let contract_instance = ContractInstance::new(
        context,
        module.clone(),
        gas_limit,
        wasm_costs,
        *contract_key,
        operation,
        nonce,
        user_public_key,
    );

    Ok(Engine::new(contract_instance, module))
}
