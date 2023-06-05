use serde::{Deserialize, Serialize};

#[cfg(feature = "random")]
use cw_types_generic::{ContractFeature, CwEnv};

use cw_types_generic::{BaseAddr, BaseEnv};

use cw_types_v010::encoding::Binary;
use cw_types_v010::types::CanonicalAddr;

use enclave_cosmos_types::types::{ContractCode, HandleType, SigInfo};
use enclave_crypto::{sha_256, Ed25519PublicKey};
use enclave_ffi_types::{Ctx, EnclaveError};
use log::*;

use crate::cosmwasm_config::ContractOperation;

#[cfg(feature = "light-client-validation")]
use crate::contract_validation::verify_block_info;

use crate::contract_validation::{ReplyParams, ValidatedMessage};
use crate::external::results::{HandleSuccess, InitSuccess, MigrateSuccess, QuerySuccess};
use crate::message::{is_ibc_msg, parse_message};
use crate::types::ParsedMessage;

use crate::random::update_msg_counter;

#[cfg(feature = "random")]
use crate::random::derive_random;
#[cfg(feature = "random")]
use crate::wasm3::Engine;

use super::contract_validation::{
    generate_contract_key, validate_contract_key, validate_msg, verify_params, ContractKey,
};
use super::gas::WasmCosts;
use super::io::{
    finalize_raw_output, manipulate_callback_sig_for_plaintext, post_process_output,
    set_all_logs_to_plaintext,
};
use super::types::{IoNonce, SecretMessage};

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

fn generate_admin_signature(admin: &[u8], contract_key: &[u8]) -> [u8; enclave_crypto::HASH_SIZE] {
    let mut data_to_hash = vec![];
    data_to_hash.extend_from_slice(admin);
    data_to_hash.extend_from_slice(contract_key);
    sha_256(&data_to_hash)
}

fn generate_contract_key_proof(
    address: &[u8],
    code_hash: &[u8],
    prev_contract_key: &[u8],
    new_contract_key: &[u8],
) -> [u8; enclave_crypto::HASH_SIZE] {
    let mut data_to_hash = vec![];
    data_to_hash.extend_from_slice(address);
    data_to_hash.extend_from_slice(code_hash);
    data_to_hash.extend_from_slice(prev_contract_key);
    data_to_hash.extend_from_slice(new_contract_key);
    sha_256(&data_to_hash)
}

#[cfg_attr(feature = "cargo-clippy", allow(clippy::too_many_arguments))]
pub fn init(
    context: Ctx,       // need to pass this to read_db & write_db
    gas_limit: u64,     // gas limit for this execution
    used_gas: &mut u64, // out-parameter for gas used in execution
    contract: &[u8],    // contract wasm bytes
    env: &[u8],         // blockchain state
    msg: &[u8],         // probably function call and args
    sig_info: &[u8],    // info about signature verification
    _admin: &[u8],      // admin's canonical address or null if no admin
) -> Result<InitSuccess, EnclaveError> {
    trace!("Starting init");

    //let start = Instant::now();
    let contract_code = ContractCode::new(contract);
    let contract_hash = contract_code.hash();
    // let duration = start.elapsed();
    // trace!("Time elapsed in ContractCode::new is: {:?}", duration);
    debug!(
        "******************** init RUNNING WITH CODE: {:?}",
        contract_hash
    );

    //let start = Instant::now();
    let base_env: BaseEnv = extract_base_env(env)?;

    #[cfg(feature = "light-client-validation")]
    {
        verify_block_info(&base_env)?;
    }

    // let duration = start.elapsed();
    // trace!("Time elapsed in extract_base_env is: {:?}", duration);
    let query_depth = extract_query_depth(env)?;

    //let start = Instant::now();
    let (sender, contract_address, block_height, sent_funds) = base_env.get_verification_params();
    // let duration = start.elapsed();
    // trace!("Time elapsed in get_verification_paramsis: {:?}", duration);

    let canonical_contract_address = to_canonical(contract_address)?;
    let canonical_sender_address = to_canonical(sender)?;

    let contract_key = generate_contract_key(
        &canonical_sender_address,
        &block_height,
        &contract_hash,
        &canonical_contract_address,
    )?;

    let admin_sig = generate_admin_signature(&canonical_sender_address.0 .0, &contract_key);

    let parsed_sig_info: SigInfo = extract_sig_info(sig_info)?;

    let secret_msg = SecretMessage::from_slice(msg)?;

    //let start = Instant::now();
    verify_params(
        &parsed_sig_info,
        sent_funds,
        &canonical_sender_address,
        contract_address,
        &secret_msg,
        #[cfg(feature = "light-client-validation")]
        msg,
        true,
        true,
        HandleType::HANDLE_TYPE_EXECUTE, // same behavior as execute
    )?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in verify_params: {:?}", duration);

    //let start = Instant::now();
    let decrypted_msg = secret_msg.decrypt()?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in decrypt: {:?}", duration);

    //let start = Instant::now();
    let ValidatedMessage {
        validated_msg,
        reply_params,
    } = validate_msg(&decrypted_msg, &contract_hash, None, None)?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in validate_msg: {:?}", duration);

    //let start = Instant::now();
    let mut engine = start_engine(
        context,
        gas_limit,
        &contract_code,
        &contract_key,
        ContractOperation::Init,
        query_depth,
        secret_msg.nonce,
        secret_msg.user_public_key,
        base_env.0.block.time,
    )?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in start_engine: {:?}", duration);

    let mut versioned_env = base_env.into_versioned_env(&engine.get_api_version());

    versioned_env.set_contract_hash(&contract_hash);

    #[cfg(feature = "random")]
    set_random_in_env(block_height, &contract_key, &mut engine, &mut versioned_env);

    update_msg_counter(block_height);
    //let start = Instant::now();
    let result = engine.init(&versioned_env, validated_msg);
    // let duration = start.elapsed();
    // trace!("Time elapsed in engine.init: {:?}", duration);

    *used_gas = engine.gas_used();

    let output = result?;

    engine
        .flush_cache()
        .map_err(|_| EnclaveError::FailedFunctionCall)?;

    // TODO: copy cosmwasm's structures to enclave
    // TODO: ref: https://github.com/CosmWasm/cosmwasm/blob/b971c037a773bf6a5f5d08a88485113d9b9e8e7b/packages/std/src/init_handle.rs#L129
    // TODO: ref: https://github.com/CosmWasm/cosmwasm/blob/b971c037a773bf6a5f5d08a88485113d9b9e8e7b/packages/std/src/query.rs#L13
    //let start = Instant::now();

    let output = post_process_output(
        output,
        &secret_msg,
        &canonical_contract_address,
        versioned_env.get_contract_hash(),
        reply_params,
        &canonical_sender_address,
        false,
        false,
    )?;

    // let duration = start.elapsed();
    // trace!("Time elapsed in encrypt_output: {:?}", duration);

    // todo: can move the key to somewhere in the output message if we want

    Ok(InitSuccess {
        output,
        contract_key,
        admin_proof: admin_sig,
    })
}

#[cfg(feature = "random")]
fn update_random_with_msg_counter(
    block_height: u64,
    contract_key: &[u8; 64],
    versioned_env: &mut CwEnv,
) {
    let old_random = versioned_env.get_random();
    debug!("Old random: {:?}", old_random);

    // rand is None if env is v0.10
    if let Some(rand) = old_random {
        versioned_env.set_random(Some(derive_random(&rand, contract_key, block_height)));
    }

    debug!("New random: {:?}", versioned_env.get_random());
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
pub fn migrate(
    context: Ctx,
    gas_limit: u64,
    used_gas: &mut u64,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
    sig_info: &[u8],
    _admin: &[u8],
    admin_proof: &[u8],
) -> Result<MigrateSuccess, EnclaveError> {
    debug!("Starting migrate");

    //let start = Instant::now();
    let contract_code = ContractCode::new(contract);
    let contract_hash = contract_code.hash();
    // let duration = start.elapsed();
    // trace!("Time elapsed in ContractCode::new is: {:?}", duration);
    debug!(
        "******************** migrate RUNNING WITH CODE: {:?}",
        contract_hash
    );

    //let start = Instant::now();
    let base_env: BaseEnv = extract_base_env(env)?;

    #[cfg(feature = "light-client-validation")]
    {
        verify_block_info(&base_env)?;
    }

    // let duration = start.elapsed();
    // trace!("Time elapsed in extract_base_env is: {:?}", duration);
    let query_depth = extract_query_depth(env)?;

    //let start = Instant::now();
    let (sender, contract_address, block_height, sent_funds) = base_env.get_verification_params();
    // let duration = start.elapsed();
    // trace!("Time elapsed in get_verification_paramsis: {:?}", duration);

    let canonical_contract_address = to_canonical(contract_address)?;
    let canonical_sender_address = to_canonical(sender)?;

    let contract_key = generate_contract_key(
        &canonical_sender_address,
        &block_height,
        &contract_hash,
        &canonical_contract_address,
    )?;

    let og_contract_key = base_env.get_contract_key()?;

    let admin_sig = generate_admin_signature(&canonical_sender_address.0 .0, &og_contract_key);

    if admin_sig != admin_proof {
        error!("Failed to validate admin signature for migrate");
        return Err(EnclaveError::ValidationFailure);
    }
    debug!("validated migration proof successfully");

    let contract_key_proof = generate_contract_key_proof(
        &canonical_sender_address.0 .0,
        &contract_code.hash(),
        &og_contract_key,
        &contract_key,
    );

    let parsed_sig_info: SigInfo = extract_sig_info(sig_info)?;

    let secret_msg = SecretMessage::from_slice(msg)?;

    //let start = Instant::now();
    verify_params(
        &parsed_sig_info,
        sent_funds,
        &canonical_sender_address,
        contract_address,
        &secret_msg,
        #[cfg(feature = "light-client-validation")]
        msg,
        true,
        true,
        HandleType::HANDLE_TYPE_EXECUTE, // same behavior as execute
    )?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in verify_params: {:?}", duration);

    //let start = Instant::now();
    let decrypted_msg = secret_msg.decrypt()?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in decrypt: {:?}", duration);

    //let start = Instant::now();
    let ValidatedMessage {
        validated_msg,
        reply_params,
    } = validate_msg(&decrypted_msg, &contract_hash, None, None)?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in validate_msg: {:?}", duration);

    //let start = Instant::now();
    let mut engine = start_engine(
        context,
        gas_limit,
        &contract_code,
        &contract_key,
        ContractOperation::Migrate,
        query_depth,
        secret_msg.nonce,
        secret_msg.user_public_key,
        base_env.0.block.time,
    )?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in start_engine: {:?}", duration);

    let mut versioned_env = base_env.into_versioned_env(&engine.get_api_version());

    versioned_env.set_contract_hash(&contract_hash);

    #[cfg(feature = "random")]
    set_random_in_env(block_height, &contract_key, &mut engine, &mut versioned_env);

    update_msg_counter(block_height);
    let result = engine.migrate(&versioned_env, validated_msg);

    *used_gas = engine.gas_used();

    let output = result?;

    engine
        .flush_cache()
        .map_err(|_| EnclaveError::FailedFunctionCall)?;

    let output = post_process_output(
        output,
        &secret_msg,
        &canonical_contract_address,
        versioned_env.get_contract_hash(),
        reply_params,
        &canonical_sender_address,
        false,
        false,
    )?;

    // let duration = start.elapsed();
    // trace!("Time elapsed in encrypt_output: {:?}", duration);

    // todo: can move the key to somewhere in the output message if we want

    debug!(
        "Migrate success: {:?}, {:?}",
        contract_key, contract_key_proof
    );

    Ok(MigrateSuccess {
        output,
        new_contract_key: contract_key,
        proof: contract_key_proof,
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

    debug!(
        "******************** HANDLE RUNNING WITH CODE: {:?}",
        contract_hash
    );

    let base_env: BaseEnv = extract_base_env(env)?;

    #[cfg(feature = "light-client-validation")]
    {
        verify_block_info(&base_env)?;
    }

    let query_depth = extract_query_depth(env)?;

    let (sender, contract_address, block_height, sent_funds) = base_env.get_verification_params();

    let canonical_contract_address = to_canonical(contract_address)?;

    let mut contract_key = base_env.get_contract_key()?;

    validate_contract_key(&contract_key, &canonical_contract_address, &contract_code)?;

    if base_env.was_migrated() {
        println!("Contract was migrated, setting keys to original one");
        let og_key = base_env.get_original_contract_key().unwrap(); // was_migrated checks that this won't fail
        contract_key = og_key.get_key();

        // validate proof
    }

    let parsed_sig_info: SigInfo = extract_sig_info(sig_info)?;

    // The flow of handle is now used for multiple messages (such ash Handle, Reply, IBC)
    // When the message is handle, we expect it always to be encrypted while in Reply & IBC it might be plaintext
    let parsed_handle_type = HandleType::try_from(handle_type)?;

    trace!("Handle type is {:?}", parsed_handle_type);

    let ParsedMessage {
        should_verify_sig_info,
        should_verify_input,
        was_msg_encrypted,
        should_encrypt_output,
        secret_msg,
        decrypted_msg,
        data_for_validation,
    } = parse_message(msg, &parsed_handle_type)?;

    let canonical_sender_address = match to_canonical(sender) {
        Ok(can) => can,
        Err(_) => CanonicalAddr::from_vec(vec![]),
    };

    // There is no signature to verify when the input isn't signed.
    // Receiving an unsigned messages is only possible in Handle (Init tx are always signed).
    // All of these scenarios go through here but the data isn't signed:
    // - Plaintext replies (resulting from an IBC call)
    // - IBC WASM Hooks
    // - (In the future:) ICA
    verify_params(
        &parsed_sig_info,
        sent_funds,
        &canonical_sender_address,
        contract_address,
        &secret_msg,
        #[cfg(feature = "light-client-validation")]
        msg,
        should_verify_sig_info,
        should_verify_input,
        parsed_handle_type,
    )?;

    let mut validated_msg = decrypted_msg.clone();
    let mut reply_params: Option<Vec<ReplyParams>> = None;
    if was_msg_encrypted {
        let x = validate_msg(
            &decrypted_msg,
            &contract_hash,
            data_for_validation,
            Some(parsed_handle_type),
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
        &contract_code,
        &contract_key,
        ContractOperation::Handle,
        query_depth,
        secret_msg.nonce,
        secret_msg.user_public_key,
        base_env.0.block.time,
    )?;

    let mut versioned_env = base_env
        .clone()
        .into_versioned_env(&engine.get_api_version());

    // We want to allow executing contracts with plaintext input via IBC,
    // even though the sender of an IBC packet cannot be verified.
    // But we don't want malicious actors using this enclave setting to fake any sender they want.
    // Therefore we'll use a null sender if it cannot be verified.
    match parsed_handle_type {
        // Execute: msg.sender was already verified
        HandleType::HANDLE_TYPE_EXECUTE => {}
        // Reply & IBC stuff: no msg.sender, set it to null just in case
        // WASM Hooks: cannot verify sender, set it to null
        HandleType::HANDLE_TYPE_REPLY
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_OPEN
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_CONNECT
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_CLOSE
        | HandleType::HANDLE_TYPE_IBC_PACKET_RECEIVE
        | HandleType::HANDLE_TYPE_IBC_PACKET_ACK
        | HandleType::HANDLE_TYPE_IBC_PACKET_TIMEOUT
        | HandleType::HANDLE_TYPE_IBC_WASM_HOOKS_INCOMING_TRANSFER
        | HandleType::HANDLE_TYPE_IBC_WASM_HOOKS_OUTGOING_TRANSFER_ACK
        | HandleType::HANDLE_TYPE_IBC_WASM_HOOKS_OUTGOING_TRANSFER_TIMEOUT => {
            versioned_env.set_msg_sender("")
        }
    }

    #[cfg(feature = "random")]
    set_random_in_env(block_height, &contract_key, &mut engine, &mut versioned_env);

    versioned_env.set_contract_hash(&contract_hash);

    update_msg_counter(block_height);

    let result = engine.handle(&versioned_env, validated_msg, &parsed_handle_type);

    *used_gas = engine.gas_used();

    let mut output = result?;

    // This gets refunded because it will get charged later by the sdk
    let refund_cache_gas = engine
        .flush_cache()
        .map_err(|_| EnclaveError::FailedFunctionCall)?;
    *used_gas = used_gas.saturating_sub(refund_cache_gas);

    debug!(
        "(2) nonce just before encrypt_output: nonce = {:?} pubkey = {:?}",
        secret_msg.nonce, secret_msg.user_public_key
    );
    if should_encrypt_output {
        output = post_process_output(
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

        output = finalize_raw_output(raw_output, false, is_ibc_msg(parsed_handle_type), false)?;
    }

    Ok(HandleSuccess { output })
}

#[cfg(feature = "random")]
fn set_random_in_env(
    block_height: u64,
    contract_key: &[u8; 64],
    engine: &mut Engine,
    versioned_env: &mut CwEnv,
) {
    {
        if engine
            .supported_features()
            .contains(&ContractFeature::Random)
        {
            debug!("random is enabled by contract");
            update_random_with_msg_counter(block_height, contract_key, versioned_env);
        } else {
            versioned_env.set_random(None);
        }
    }
}

fn extract_sig_info(sig_info: &[u8]) -> Result<SigInfo, EnclaveError> {
    serde_json::from_slice(sig_info).map_err(|err| {
        warn!(
            "handle got an error while trying to deserialize sig info input bytes into json {:?}: {}",
            String::from_utf8_lossy(sig_info),
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
    let query_depth = extract_query_depth(env)?;

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
        &contract_code,
        &contract_key,
        ContractOperation::Query,
        query_depth,
        secret_msg.nonce,
        secret_msg.user_public_key,
        base_env.0.block.time,
    )?;

    let mut versioned_env = base_env
        .clone()
        .into_versioned_env(&engine.get_api_version());

    versioned_env.set_contract_hash(&contract_hash);

    let result = engine.query(&versioned_env, validated_msg);
    *used_gas = engine.gas_used();
    let output = result?;

    let output = post_process_output(
        output,
        &secret_msg,
        &CanonicalAddr(Binary(Vec::new())), // Not used for queries (can't init a new contract from a query)
        "",   // Not used for queries (can't call a sub-message from a query),
        None, // Not used for queries (Query response is not replied to the caller),
        &CanonicalAddr(Binary(Vec::new())), // Not used for queries (used only for replies)
        true,
        false,
    )?;

    Ok(QuerySuccess { output })
}

#[allow(clippy::too_many_arguments)]
fn start_engine(
    context: Ctx,
    gas_limit: u64,
    contract_code: &ContractCode,
    contract_key: &ContractKey,
    operation: ContractOperation,
    query_depth: u32,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
    timestamp: u64,
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
        query_depth,
        timestamp,
    )
}

fn extract_base_env(env: &[u8]) -> Result<BaseEnv, EnclaveError> {
    serde_json::from_slice(env)
        .map_err(|err| {
            warn!(
                "error while deserializing env from json {:?}: {}",
                String::from_utf8_lossy(env),
                err
            );
            EnclaveError::FailedToDeserialize
        })
        .map(|base_env| {
            trace!("base env: {:?}", base_env);
            base_env
        })
}

#[derive(Debug, Serialize, Deserialize)]
struct EnvWithQD {
    query_depth: u32,
}

/// Extract the query_depth from the env parameter.
///
/// This is done in a separate method and type definition in order
/// to simplify the code and avoid further coupling of the query depth
/// parameter and the CW Env type.
fn extract_query_depth(env: &[u8]) -> Result<u32, EnclaveError> {
    serde_json::from_slice::<EnvWithQD>(env)
        .map_err(|err| {
            warn!(
                "error while deserializing env into json {:?}: {}",
                String::from_utf8_lossy(env),
                err
            );
            EnclaveError::FailedToDeserialize
        })
        .map(|env| {
            trace!("base env: {:?}", env);
            env.query_depth
        })
}
