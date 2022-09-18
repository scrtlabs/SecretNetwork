use cw_types_generic::{BaseAddr, BaseEnv};

use cw_types_v010::encoding::Binary;
use cw_types_v010::types::CanonicalAddr;

use enclave_cosmos_types::types::{ContractCode, HandleType, SigInfo};
use enclave_crypto::Ed25519PublicKey;
use enclave_ffi_types::{Ctx, EnclaveError};
use log::*;
use std::time::Instant;

use crate::contract_validation::{ReplyParams, ValidatedMessage};
use crate::external::results::{HandleSuccess, InitSuccess, QuerySuccess};
use crate::message::{is_ibc_msg, parse_message, ParsedMessage};
use crate::wasm::traits::WasmiApi;

use super::contract_validation::{
    generate_encryption_key, validate_contract_key, validate_msg, verify_params, ContractKey,
};
use super::gas::WasmCosts;
use super::io::{
    encrypt_output, finalize_raw_output, manipulate_callback_sig_for_plaintext,
    set_all_logs_to_plaintext,
};
//use super::module_cache::create_module_instance;
use super::types::{IoNonce, SecretMessage};
use super::wasm::ContractOperation;
// use super::wasm::{ContractInstance, Engine};

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
    trace!("Starting init");

    let mut start = Instant::now();
    let contract_code = ContractCode::new(contract);
    let contract_hash = contract_code.hash();
    let mut duration = start.elapsed();
    println!("Time elapsed in ContractCode::new is: {:?}", duration);

    let mut start = Instant::now();
    let base_env: BaseEnv = extract_base_env(env)?;
    let mut duration = start.elapsed();
    println!("Time elapsed in extract_base_env is: {:?}", duration);

    let mut start = Instant::now();
    let (sender, contract_address, block_height, sent_funds) = base_env.get_verification_params();
    let mut duration = start.elapsed();
    println!("Time elapsed in get_verification_paramsis: {:?}", duration);

    let canonical_contract_address = to_canonical(contract_address)?;

    let canonical_sender_address = to_canonical(sender)?;

    let contract_key = generate_encryption_key(
        &canonical_sender_address,
        &block_height,
        &contract_hash,
        &canonical_contract_address,
    )?;

    let parsed_sig_info: SigInfo = extract_sig_info(sig_info)?;

    let secret_msg = SecretMessage::from_slice(msg)?;

    let mut start = Instant::now();
    verify_params(
        &parsed_sig_info,
        &sent_funds,
        &canonical_sender_address,
        &contract_address,
        &secret_msg,
    )?;
    let mut duration = start.elapsed();
    println!("Time elapsed in verify_params: {:?}", duration);

    let mut start = Instant::now();
    let decrypted_msg = secret_msg.decrypt()?;
    let mut duration = start.elapsed();
    println!("Time elapsed in decrypt: {:?}", duration);

    let mut start = Instant::now();
    let ValidatedMessage {
        validated_msg,
        reply_params,
    } = validate_msg(&decrypted_msg, &contract_hash, None, None)?;
    let mut duration = start.elapsed();
    println!("Time elapsed in validate_msg: {:?}", duration);

    let mut start = Instant::now();
    let mut engine = start_engine(
        context,
        gas_limit,
        contract_code,
        &contract_key,
        ContractOperation::Init,
        secret_msg.nonce,
        secret_msg.user_public_key,
    )?;
    let mut duration = start.elapsed();
    println!("Time elapsed in start_engine: {:?}", duration);

    let mut versioned_env = base_env.into_versioned_env(&engine.get_api_version());

    versioned_env.set_contract_hash(&contract_hash);

    let mut start = Instant::now();
    let result = engine.init(&versioned_env, validated_msg);
    let mut duration = start.elapsed();
    println!("Time elapsed in engine.init: {:?}", duration);

    *used_gas = engine.gas_used();
    let output = result?;

    engine
        .contract_instance
        .flush_cache()
        .map_err(|_| EnclaveError::FailedFunctionCall)?;

    // TODO: copy cosmwasm's structures to enclave
    // TODO: ref: https://github.com/CosmWasm/cosmwasm/blob/b971c037a773bf6a5f5d08a88485113d9b9e8e7b/packages/std/src/init_handle.rs#L129
    // TODO: ref: https://github.com/CosmWasm/cosmwasm/blob/b971c037a773bf6a5f5d08a88485113d9b9e8e7b/packages/std/src/query.rs#L13
    let mut start = Instant::now();
    let output = encrypt_output(
        output,
        &secret_msg,
        &canonical_contract_address,
        versioned_env.get_contract_hash(),
        reply_params,
        &canonical_sender_address,
        false,
        false,
    )?;
    let mut duration = start.elapsed();
    println!("Time elapsed in encrypt_output: {:?}", duration);

    // todo: can move the key to somewhere in the output message if we want

    Ok(InitSuccess {
        output,
        contract_key,
    })
}

fn to_canonical(contract_address: &BaseAddr) -> Result<CanonicalAddr, EnclaveError> {
    CanonicalAddr::from_human(contract_address).map_err(|err| {
        warn!(
            "error while trying to deserialize address from bech32 string to bytes {:?}: {}",
            contract_address, err
        );
        EnclaveError::FailedToDeserialize
    })
}

#[cfg_attr(feature = "cargo-clippy", allow(clippy::too_many_arguments))]
pub fn handle(
    context: Ctx,
    gas_limit: u64,
    used_gas: &mut u64,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
    sig_info: &[u8],
    handle_type: u8,
) -> Result<HandleSuccess, EnclaveError> {
    trace!("Starting handle");

    let contract_code = ContractCode::new(contract);
    let contract_hash = contract_code.hash();

    let base_env: BaseEnv = extract_base_env(env)?;

    let (sender, contract_address, _, sent_funds) = base_env.get_verification_params();

    let canonical_contract_address = to_canonical(contract_address)?;

    let contract_key = base_env.get_contract_key()?;

    validate_contract_key(&contract_key, &canonical_contract_address, &contract_code)?;

    let parsed_sig_info: SigInfo = extract_sig_info(sig_info)?;

    // The flow of handle is now used for multiple messages (such ash Handle, Reply)
    // When the message is handle, we expect it always to be encrypted while in Reply for example it might be plaintext
    let parsed_handle_type = HandleType::try_from(handle_type)?;

    trace!("Handle type is {:?}", parsed_handle_type);

    let ParsedMessage {
        should_validate_sig_info,
        was_msg_encrypted,
        should_encrypt_output,
        secret_msg,
        decrypted_msg,
        contract_hash_for_validation,
    } = parse_message(msg, &parsed_sig_info, &parsed_handle_type)?;

    let canonical_sender_address = match to_canonical(sender) {
        Ok(can) => can,
        Err(_) => CanonicalAddr::from_vec(vec![]),
    };

    // There is no signature to verify when the input isn't signed.
    // Receiving unsigned messages is only possible in Handle. (Init tx are always signed)
    // All of these functions go through handle but the data isn't signed:
    //  Reply (that is not WASM reply)
    if should_validate_sig_info {
        // Verify env parameters against the signed tx

        verify_params(
            &parsed_sig_info,
            sent_funds,
            &canonical_sender_address,
            contract_address,
            &secret_msg,
        )?;
    }

    let mut validated_msg = decrypted_msg.clone();
    let mut reply_params: Option<ReplyParams> = None;
    if was_msg_encrypted {
        let x = validate_msg(
            &decrypted_msg,
            &contract_hash,
            contract_hash_for_validation,
            Some(parsed_handle_type.clone()),
        )?;
        validated_msg = x.validated_msg;
        reply_params = x.reply_params;
    }

    // Although the operation here is not always handle it is irrelevant in this case
    // because it only helps to decide whether to check floating points or not
    // In this case we want to do the same as in Handle both for Reply and for others so we can always pass "Handle".
    let mut engine = start_engine(
        context,
        gas_limit,
        contract_code,
        &contract_key,
        ContractOperation::Handle,
        secret_msg.nonce,
        secret_msg.user_public_key,
    )?;

    let mut versioned_env = base_env
        .clone()
        .into_versioned_env(&engine.get_api_version());

    versioned_env.set_contract_hash(&contract_hash);

    let result = engine.handle(&versioned_env, validated_msg, &parsed_handle_type);
    *used_gas = engine.gas_used();
    let mut output = result?;

    engine
        .contract_instance
        .flush_cache()
        .map_err(|_| EnclaveError::FailedFunctionCall)?;

    debug!(
        "(2) nonce just before encrypt_output: nonce = {:?} pubkey = {:?}",
        secret_msg.nonce, secret_msg.user_public_key
    );
    if should_encrypt_output {
        output = encrypt_output(
            output,
            &secret_msg,
            &canonical_contract_address,
            versioned_env.get_contract_hash(),
            reply_params,
            &canonical_sender_address,
            false,
            is_ibc_msg(parsed_handle_type),
        )?;
    } else {
        let mut raw_output =
            manipulate_callback_sig_for_plaintext(&canonical_contract_address, output)?;
        set_all_logs_to_plaintext(&mut raw_output);

        let finalized_output =
            finalize_raw_output(raw_output, false, is_ibc_msg(parsed_handle_type), false);
        trace!(
            "Wasm output for plaintext message is: {:?}",
            finalized_output
        );

        output = serde_json::to_vec(&finalized_output).map_err(|err| {
            debug!(
                "got an error while trying to serialize output json into bytes {:?}: {}",
                finalized_output, err
            );
            EnclaveError::FailedToSerialize
        })?;
    }

    Ok(HandleSuccess { output })
}

fn extract_sig_info(sig_info: &[u8]) -> Result<SigInfo, EnclaveError> {
    serde_json::from_slice(sig_info).map_err(|err| {
        warn!(
            "handle got an error while trying to deserialize sig info input bytes into json {:?}: {}",
            String::from_utf8_lossy(&sig_info),
            err
        );
        EnclaveError::FailedToDeserialize
    })
}

pub fn query(
    context: Ctx,
    gas_limit: u64,
    used_gas: &mut u64,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
) -> Result<QuerySuccess, EnclaveError> {
    trace!("Entered query");

    let contract_code = ContractCode::new(contract);
    let contract_hash = contract_code.hash();

    let base_env: BaseEnv = extract_base_env(env)?;

    let (_, contract_address, _, _) = base_env.get_verification_params();

    let canonical_contract_address = to_canonical(contract_address)?;

    let contract_key = base_env.get_contract_key()?;

    validate_contract_key(&contract_key, &canonical_contract_address, &contract_code)?;

    let secret_msg = SecretMessage::from_slice(msg)?;
    let decrypted_msg = secret_msg.decrypt()?;

    let ValidatedMessage { validated_msg, .. } =
        validate_msg(&decrypted_msg, &contract_hash, None, None)?;

    let mut engine = start_engine(
        context,
        gas_limit,
        contract_code,
        &contract_key,
        ContractOperation::Query,
        secret_msg.nonce,
        secret_msg.user_public_key,
    )?;

    let mut versioned_env = base_env
        .clone()
        .into_versioned_env(&engine.get_api_version());

    versioned_env.set_contract_hash(&contract_hash);

    let result = engine.query(&versioned_env, validated_msg);
    *used_gas = engine.gas_used();
    let output = result?;

    let output = encrypt_output(
        output,
        &secret_msg,
        &CanonicalAddr(Binary(Vec::new())), // Not used for queries (can't init a new contract from a query)
        &"".to_string(), // Not used for queries (can't call a sub-message from a query),
        None,            // Not used for queries (Query response is not replied to the caller),
        &CanonicalAddr(Binary(Vec::new())), // Not used for queries (used only for replies)
        true,
        false,
    )?;

    Ok(QuerySuccess { output })
}

fn start_engine(
    context: Ctx,
    gas_limit: u64,
    contract_code: ContractCode,
    contract_key: &ContractKey,
    operation: ContractOperation,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
) -> Result<crate::wasm3::Engine, EnclaveError> {
    crate::wasm3::Engine::new(
        context,
        gas_limit,
        WasmCosts::default(),
        contract_code,
        *contract_key,
        operation,
        nonce,
        user_public_key,
    )
}

// TODO remove once the new engine works well
// fn _start_engine(
//     context: Ctx,
//     gas_limit: u64,
//     contract_code: ContractCode,
//     contract_key: &ContractKey,
//     operation: ContractOperation,
//     nonce: IoNonce,
//     user_public_key: Ed25519PublicKey,
// ) -> Result<Engine, EnclaveError> {
//     let module = create_module_instance(contract_code, operation)?;
//
//     // Set the gas costs for wasm op-codes (there is an inline stack_height limit in WasmCosts)
//     let wasm_costs = WasmCosts::default();
//
//     let contract_instance = ContractInstance::new(
//         context,
//         module.clone(),
//         gas_limit,
//         wasm_costs,
//         *contract_key,
//         operation,
//         nonce,
//         user_public_key,
//     )?;
//
//     Ok(Engine::new(contract_instance, module))
// }

fn extract_base_env(env: &[u8]) -> Result<BaseEnv, EnclaveError> {
    serde_json::from_slice(env)
        .map_err(|err| {
            warn!(
                "error while deserializing env into json {:?}: {}",
                String::from_utf8_lossy(&env),
                err
            );
            EnclaveError::FailedToDeserialize
        })
        .map(|base_env| {
            trace!("base env: {:?}", base_env);
            base_env
        })
}
