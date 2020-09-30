use log::*;
use parity_wasm::elements;
use parity_wasm::elements::Module;
use wasmi::ModuleInstance;

use enclave_ffi_types::{Ctx, EnclaveError};

use crate::cosmwasm::types::{CanonicalAddr, Env, SigInfo};
use crate::crypto::Ed25519PublicKey;
use crate::results::{HandleSuccess, InitSuccess, QuerySuccess};
use crate::wasm::types::{IoNonce, SecretMessage};

use super::contract_validation::{
    calc_contract_hash, extract_contract_key, generate_encryption_key, validate_contract_key,
    validate_msg, verify_params, ContractKey, CONTRACT_KEY_LENGTH,
};
use super::gas::{gas_rules, WasmCosts};
use super::io::encrypt_output;
use super::{
    memory::validate_memory,
    runtime::{create_builder, ContractInstance, ContractOperation, Engine, WasmiImportResolver},
};

use crate::coalesce;
use crate::cosmwasm::encoding::Binary;

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
    sig_info: &[u8],    // info about signature verification
) -> Result<InitSuccess, EnclaveError> {
    let mut parsed_env: Env = serde_json::from_slice(env).map_err(|err| {
        warn!(
            "got an error while trying to deserialize env input bytes into json {:?}: {}",
            String::from_utf8_lossy(&env),
            err
        );
        EnclaveError::FailedToDeserialize
    })?;

    let canonical_contract_address = CanonicalAddr::from_human(&parsed_env.contract.address).map_err(|err| {
        warn!(
            "got an error while trying to deserialize parsed_env.contract.address from bech32 string to bytes {:?}: {}",
            parsed_env.contract.address, err
        );
        EnclaveError::FailedToDeserialize
    })?;
    let contract_key =
        generate_encryption_key(&parsed_env, contract, &(canonical_contract_address.0).0)?;
    trace!("Init: Contract Key: {:?}", contract_key.to_vec().as_slice());

    let parsed_sig_info: SigInfo = serde_json::from_slice(sig_info).map_err(|err| {
        warn!(
            "got an error while trying to deserialize env input bytes into json {:?}: {}",
            String::from_utf8_lossy(&sig_info),
            err
        );
        EnclaveError::FailedToDeserialize
    })?;

    let secret_msg = SecretMessage::from_slice(msg)?;
    trace!(
        "Init input before decryption: {:?}",
        String::from_utf8_lossy(&msg)
    );

    verify_params(&parsed_sig_info, &parsed_env, &secret_msg)?;

    let decrypted_msg = secret_msg.decrypt()?;

    let validated_msg = validate_msg(&decrypted_msg, contract)?;

    trace!(
        "Init input after decryption: {:?}",
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

    parsed_env.contract_code_hash = hex::encode(calc_contract_hash(contract));

    let new_env = serde_json::to_vec(&parsed_env).map_err(|err| {
        warn!(
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
        let output = encrypt_output(
            output,
            secret_msg.nonce,
            secret_msg.user_public_key,
            &canonical_contract_address,
        )?;

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
        contract_key,
    })
}

pub fn handle(
    context: Ctx,
    gas_limit: u64,
    used_gas: &mut u64,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
    sig_info: &[u8],
) -> Result<HandleSuccess, EnclaveError> {
    let mut parsed_env: Env = serde_json::from_slice(env).map_err(|err| {
        warn!(
            "got an error while trying to deserialize env input bytes into json {:?}: {}",
            env, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    trace!("handle parsed_env: {:?}", parsed_env);

    let parsed_sig_info: SigInfo = serde_json::from_slice(sig_info).map_err(|err| {
        warn!(
            "got an error while trying to deserialize env input bytes into json {:?}: {}",
            String::from_utf8_lossy(&sig_info),
            err
        );
        EnclaveError::FailedToDeserialize
    })?;

    let secret_msg = SecretMessage::from_slice(msg)?;

    // Verify env parameters against the signed tx
    verify_params(&parsed_sig_info, &parsed_env, &secret_msg)?;

    let contract_key = extract_contract_key(&parsed_env)?;

    let secret_msg = SecretMessage::from_slice(msg)?;
    let decrypted_msg = secret_msg.decrypt()?;

    let validated_msg = validate_msg(&decrypted_msg, contract)?;

    trace!(
        "Handle input afer decryption: {:?}",
        String::from_utf8_lossy(&validated_msg)
    );

    let canonical_contract_address = CanonicalAddr::from_human(&parsed_env.contract.address).map_err(|err| {
        warn!(
            "got an error while trying to deserialize parsed_env.contract.address from bech32 string to bytes {:?}: {}",
            parsed_env.contract.address, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    if !validate_contract_key(&contract_key, &(canonical_contract_address.0).0, contract) {
        warn!("got an error while trying to deserialize output bytes");
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

    parsed_env.contract_code_hash = hex::encode(calc_contract_hash(contract));

    let new_env = serde_json::to_vec(&parsed_env).map_err(|err| {
        warn!(
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
        let output = encrypt_output(
            output,
            secret_msg.nonce,
            secret_msg.user_public_key,
            &canonical_contract_address,
        )?;
        Ok(output)
    })
    .map_err(|err| {
        *used_gas = engine.gas_used();
        err
    })?;

    *used_gas = engine.gas_used();
    Ok(HandleSuccess { output })
}

pub fn query(
    context: Ctx,
    gas_limit: u64,
    used_gas: &mut u64,
    contract: &[u8],
    msg: &[u8],
) -> Result<QuerySuccess, EnclaveError> {
    if msg.len() < CONTRACT_KEY_LENGTH {
        warn!("Input query is shorter than the minimum expected. Msg is malformed");
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

        let output = encrypt_output(
            output,
            secret_msg.nonce,
            secret_msg.user_public_key,
            &CanonicalAddr(Binary(Vec::new())), // Not used for queries
        )?;
        Ok(output)
    })
    .map_err(|err| {
        *used_gas = engine.gas_used();
        err
    })?;

    *used_gas = engine.gas_used();
    Ok(QuerySuccess { output })
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
    info!("Deserializing Wasm contract");

    // Create a parity-wasm module first, so we can inject gas metering to it
    // (you need a parity-wasm module to use the pwasm-utils crate)
    let mut p_modlue: Module =
        elements::deserialize_buffer(contract).map_err(|_| EnclaveError::InvalidWasm)?;

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

    // Create new imports resolver.
    // These are the signatures of rust functions available to invoke from wasm code.
    let resolver = WasmiImportResolver {};
    let imports_builder = create_builder(&resolver);

    // Instantiate a module with our imports and assert that there is no `start` function.
    let module_instance = ModuleInstance::new(&module, &imports_builder).map_err(|err| {
        warn!("Error in instantiation: {:?}", err);
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
