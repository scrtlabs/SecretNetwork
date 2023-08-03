use std::collections::HashMap;

use serde::{Deserialize, Serialize};

#[cfg(feature = "random")]
use cw_types_generic::{ContractFeature, CwEnv};

use cw_types_generic::{BaseAddr, BaseEnv};

use cw_types_v010::encoding::Binary;
use cw_types_v010::types::{CanonicalAddr, HumanAddr};

use enclave_cosmos_types::types::{ContractCode, HandleType, SigInfo, VerifyParamsType};
use enclave_crypto::Ed25519PublicKey;
use enclave_ffi_types::{Ctx, EnclaveError};
use log::*;

use crate::cosmwasm_config::ContractOperation;

#[cfg(feature = "light-client-validation")]
use crate::contract_validation::verify_block_info;

use crate::contract_validation::{
    generate_admin_proof, generate_contract_key_proof, ReplyParams, ValidatedMessage,
};
use crate::external::results::{
    HandleSuccess, InitSuccess, MigrateSuccess, QuerySuccess, UpdateAdminSuccess,
};
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

#[cfg_attr(feature = "cargo-clippy", allow(clippy::too_many_arguments))]
pub fn init(
    context: Ctx,       // need to pass this to read_db & write_db
    gas_limit: u64,     // gas limit for this execution
    used_gas: &mut u64, // out-parameter for gas used in execution
    contract: &[u8],    // contract wasm bytes
    env: &[u8],         // blockchain state
    msg: &[u8],         // probably function call and args
    sig_info: &[u8],    // info about signature verification
    admin: &[u8],       // admin's canonical address or null if no admin
) -> Result<InitSuccess, EnclaveError> {
    trace!("Starting init");

    //let start = Instant::now();
    let contract_code = ContractCode::new(contract);
    let contract_hash = contract_code.hash();
    // let duration = start.elapsed();
    // trace!("Time elapsed in ContractCode::new is: {:?}", duration);
    debug!(
        "******************** init RUNNING WITH CODE: {:x?}",
        contract_hash
    );

    //let start = Instant::now();
    let base_env: BaseEnv = extract_base_env(env)?;

    #[cfg(feature = "light-client-validation")]
    verify_block_info(&base_env)?;

    // let duration = start.elapsed();
    // trace!("Time elapsed in extract_base_env is: {:?}", duration);
    let query_depth = extract_query_depth(env)?;

    //let start = Instant::now();
    let (sender, contract_address, block_height, sent_funds) = base_env.get_verification_params();
    // let duration = start.elapsed();
    // trace!("Time elapsed in get_verification_paramsis: {:?}", duration);

    let canonical_contract_address = to_canonical(contract_address)?;
    let canonical_sender_address = to_canonical(sender)?;
    let canonical_admin_address = CanonicalAddr::from_vec(admin.to_vec());

    // contract_key is a unique key for each contract
    // it's used in state encryption to prevent the same
    // encryption keys from being used for different contracts
    let og_contract_key = generate_contract_key(
        &canonical_sender_address,
        &block_height,
        &contract_hash,
        &canonical_contract_address,
        None,
    )?;

    let parsed_sig_info: SigInfo = extract_sig_info(sig_info)?;

    let secret_msg = SecretMessage::from_slice(msg)?;

    //let start = Instant::now();
    verify_params(
        &parsed_sig_info,
        sent_funds,
        &canonical_sender_address,
        contract_address,
        &secret_msg,
        true,
        true,
        VerifyParamsType::Init,
        Some(&canonical_admin_address),
        None,
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
    } = validate_msg(
        &canonical_contract_address,
        &decrypted_msg,
        &contract_hash,
        None,
        None,
    )?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in validate_msg: {:?}", duration);

    //let start = Instant::now();
    let mut engine = start_engine(
        context,
        gas_limit,
        &contract_code,
        &og_contract_key,
        ContractOperation::Init,
        query_depth,
        secret_msg.nonce,
        secret_msg.user_public_key,
        base_env.0.block.time,
    )?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in start_engine: {:?}", duration);

    let mut versioned_env = base_env
        .clone()
        .into_versioned_env(&engine.get_api_version());

    versioned_env.set_contract_hash(&contract_hash);

    #[cfg(feature = "random")]
    set_random_in_env(
        block_height,
        &og_contract_key,
        &mut engine,
        &mut versioned_env,
    );

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

    let admin_proof = generate_admin_proof(&canonical_admin_address.0 .0, &og_contract_key);

    Ok(InitSuccess {
        output,
        contract_key: og_contract_key,
        admin_proof,
    })
}

#[cfg(feature = "random")]
fn update_random_with_msg_counter(
    block_height: u64,
    contract_key: &[u8; 64],
    versioned_env: &mut CwEnv,
) {
    let old_random = versioned_env.get_random();
    debug!("Old random: {:x?}", old_random);

    // rand is None if env is v0.10
    if let Some(rand) = old_random {
        versioned_env.set_random(Some(derive_random(&rand, contract_key, block_height)));
    }

    debug!("New random: {:x?}", versioned_env.get_random());
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

lazy_static::lazy_static! {
    /// Current hardcoded contract admins
    static ref HARDCODED_CONTRACT_ADMINS: HashMap<&'static str, &'static str> = HashMap::from([
        (
            "secret1exampleContractAddress1",
            "secret1ExampleAdminAddress1",
        ),
        (
            "secret1exampleContractAddress2",
            "secret1ExampleAdminAddress2",
        ),
    ]);

    /// The entire history of contracts that were deployed before v1.10 and have been migrated using the hardcoded admin feature.
    /// These contracts might have other contracts that call them with a wrong code_hash, because those other contracts have it stored from before the migration.
    static ref ALLOWED_CONTRACT_CODE_HASH: HashMap<&'static str, &'static str> = HashMap::from([
    (
        "secret1exampleContractAddress1",
        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    ),
    (
        "secret1exampleContractAddress2",
        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    ),
]);
}

/// Current hardcoded contract admins
fn is_hardcoded_contract_admin(
    contract: &CanonicalAddr,
    admin: &CanonicalAddr,
    admin_proof: &[u8],
) -> bool {
    if admin_proof != [0; enclave_crypto::HASH_SIZE] {
        return false;
    }

    let contract = HumanAddr::from_canonical(contract);
    if contract.is_err() {
        trace!(
            "is_hardcoded_contract_admin: failed to convert contract to human address: {:?}",
            contract.err().unwrap()
        );
        return false;
    }
    let contract = contract.unwrap();

    let admin = HumanAddr::from_canonical(admin);
    if admin.is_err() {
        trace!(
            "is_hardcoded_contract_admin: failed to convert admin to human address: {:?}",
            admin.err().unwrap()
        );
        return false;
    }
    let admin = admin.unwrap();

    HARDCODED_CONTRACT_ADMINS.get(contract.as_str()) == Some(&admin.as_str())
}

/// The entire history of contracts that were deployed before v1.10 and have been migrated using the hardcoded admin feature.
/// These contracts might have other contracts that call them with a wrong code_hash, because those other contracts have it stored from before the migration.
pub fn is_code_hash_allowed(contract_address: &CanonicalAddr, code_hash: &str) -> bool {
    let contract_address = HumanAddr::from_canonical(contract_address);
    if contract_address.is_err() {
        trace!(
            "is_code_hash_allowed: failed to convert contract to human address: {:?}",
            contract_address.err().unwrap()
        );
        return false;
    }
    let contract = contract_address.unwrap();

    ALLOWED_CONTRACT_CODE_HASH.get(contract.as_str()) == Some(&code_hash)
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
    admin: &[u8],
    admin_proof: &[u8],
) -> Result<MigrateSuccess, EnclaveError> {
    debug!("Starting migrate");

    //let start = Instant::now();
    let contract_code = ContractCode::new(contract);
    let contract_hash = contract_code.hash();
    // let duration = start.elapsed();
    // trace!("Time elapsed in ContractCode::new is: {:?}", duration);
    debug!(
        "******************** migrate RUNNING WITH CODE: {:x?}",
        contract_hash
    );

    //let start = Instant::now();
    let base_env: BaseEnv = extract_base_env(env)?;

    #[cfg(feature = "light-client-validation")]
    verify_block_info(&base_env)?;

    // let duration = start.elapsed();
    // trace!("Time elapsed in extract_base_env is: {:?}", duration);
    let query_depth = extract_query_depth(env)?;

    //let start = Instant::now();
    let (sender, contract_address, block_height, sent_funds) = base_env.get_verification_params();
    // let duration = start.elapsed();
    // trace!("Time elapsed in get_verification_paramsis: {:?}", duration);

    let canonical_contract_address = to_canonical(contract_address)?;
    let canonical_sender_address = to_canonical(sender)?;
    let canonical_admin_address = CanonicalAddr::from_vec(admin.to_vec());

    let og_contract_key = base_env.get_og_contract_key()?;

    if is_hardcoded_contract_admin(
        &canonical_contract_address,
        &canonical_admin_address,
        admin_proof,
    ) {
        debug!("Found hardcoded admin for migrate");
    } else {
        let sender_admin_proof =
            generate_admin_proof(&canonical_sender_address.0 .0, &og_contract_key);

        if admin_proof != sender_admin_proof {
            error!("Failed to validate sender as current admin for migrate");
            return Err(EnclaveError::ValidationFailure);
        }
        debug!("Validated migrate proof successfully");
    }

    let parsed_sig_info: SigInfo = extract_sig_info(sig_info)?;

    let secret_msg = SecretMessage::from_slice(msg)?;

    //let start = Instant::now();
    verify_params(
        &parsed_sig_info,
        sent_funds,
        &canonical_sender_address,
        contract_address,
        &secret_msg,
        true,
        true,
        VerifyParamsType::Migrate,
        Some(&canonical_admin_address),
        None,
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
    } = validate_msg(
        &canonical_contract_address,
        &decrypted_msg,
        &contract_hash,
        None,
        None,
    )?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in validate_msg: {:?}", duration);

    //let start = Instant::now();
    let mut engine = start_engine(
        context,
        gas_limit,
        &contract_code,
        &og_contract_key,
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

    let new_contract_key = generate_contract_key(
        &canonical_sender_address,
        &block_height,
        &contract_hash,
        &canonical_contract_address,
        Some(&og_contract_key),
    )?;

    #[cfg(feature = "random")]
    set_random_in_env(
        block_height,
        &new_contract_key,
        &mut engine,
        &mut versioned_env,
    );

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

    let new_contract_key_proof = generate_contract_key_proof(
        &canonical_contract_address.0 .0,
        &contract_code.hash(),
        &og_contract_key,
        &new_contract_key,
    );

    debug!(
        "Migrate success: {:x?}, {:x?}",
        new_contract_key, new_contract_key_proof
    );

    Ok(MigrateSuccess {
        output,
        new_contract_key,
        new_contract_key_proof,
    })
}

pub fn update_admin(
    env: &[u8],
    sig_info: &[u8],
    current_admin: &[u8],
    current_admin_proof: &[u8],
    new_admin: &[u8],
) -> Result<UpdateAdminSuccess, EnclaveError> {
    debug!("Starting update_admin");

    let base_env: BaseEnv = extract_base_env(env)?;

    #[cfg(feature = "light-client-validation")]
    verify_block_info(&base_env)?;

    let (sender, contract_address, _block_height, sent_funds) = base_env.get_verification_params();

    let canonical_sender_address = to_canonical(sender)?;
    let canonical_current_admin_address = CanonicalAddr::from_vec(current_admin.to_vec());
    let canonical_new_admin_address = CanonicalAddr::from_vec(new_admin.to_vec());

    let canonical_contract_address = to_canonical(contract_address)?;

    if is_hardcoded_contract_admin(
        &canonical_contract_address,
        &canonical_current_admin_address,
        current_admin_proof,
    ) {
        debug!(
            "Found hardcoded admin for update_admin. Cannot update admin for hardcoded contracts."
        );
        return Err(EnclaveError::ValidationFailure);
    }

    let og_contract_key = base_env.get_og_contract_key()?;

    let sender_admin_proof = generate_admin_proof(&canonical_sender_address.0 .0, &og_contract_key);

    if sender_admin_proof != current_admin_proof {
        error!("Failed to validate sender as current admin for update_admin");
        return Err(EnclaveError::ValidationFailure);
    }
    debug!("Validated update_admin proof successfully");

    let parsed_sig_info: SigInfo = extract_sig_info(sig_info)?;

    verify_params(
        &parsed_sig_info,
        sent_funds,
        &canonical_sender_address,
        contract_address,
        &SecretMessage {
            nonce: [0; 32],
            user_public_key: [0; 32],
            msg: vec![], // must be empty vec for callback_sig verification
        },
        true,
        true,
        VerifyParamsType::UpdateAdmin,
        Some(&canonical_current_admin_address),
        Some(&canonical_new_admin_address),
    )?;

    let new_admin_proof = generate_admin_proof(&canonical_new_admin_address.0 .0, &og_contract_key);

    debug!("update_admin success: {:?}", new_admin_proof);

    Ok(UpdateAdminSuccess { new_admin_proof })
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
        "******************** HANDLE RUNNING WITH CODE: {:x?}",
        contract_hash
    );

    let base_env: BaseEnv = extract_base_env(env)?;

    #[cfg(feature = "light-client-validation")]
    verify_block_info(&base_env)?;

    let query_depth = extract_query_depth(env)?;

    let (sender, contract_address, block_height, sent_funds) = base_env.get_verification_params();

    let canonical_contract_address = to_canonical(contract_address)?;

    validate_contract_key(&base_env, &canonical_contract_address, &contract_code)?;

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
        should_verify_sig_info,
        should_verify_input,
        VerifyParamsType::HandleType(parsed_handle_type),
        None,
        None,
    )?;

    let mut validated_msg = decrypted_msg.clone();
    let mut reply_params: Option<Vec<ReplyParams>> = None;
    if was_msg_encrypted {
        let x = validate_msg(
            &canonical_contract_address,
            &decrypted_msg,
            &contract_hash,
            data_for_validation,
            Some(parsed_handle_type),
        )?;
        validated_msg = x.validated_msg;
        reply_params = x.reply_params;
    }

    let og_contract_key = base_env.get_og_contract_key()?;

    // Although the operation here is not always handle it is irrelevant in this case
    // because it only helps to decide whether to check floating points or not
    // In this case we want to do the same as in Handle both for Reply and for others so we can always pass "Handle".
    let mut engine = start_engine(
        context,
        gas_limit,
        &contract_code,
        &og_contract_key,
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
    {
        let contract_key_for_random = base_env.get_latest_contract_key()?;
        set_random_in_env(
            block_height,
            &contract_key_for_random,
            &mut engine,
            &mut versioned_env,
        );
    }

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
        "(2) nonce just before encrypt_output: nonce = {:x?} pubkey = {:x?}",
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

    validate_contract_key(&base_env, &canonical_contract_address, &contract_code)?;

    let secret_msg = SecretMessage::from_slice(msg)?;
    let decrypted_msg = secret_msg.decrypt()?;

    let ValidatedMessage { validated_msg, .. } = validate_msg(
        &canonical_contract_address,
        &decrypted_msg,
        &contract_hash,
        None,
        None,
    )?;

    let og_contract_key = base_env.get_og_contract_key()?;

    let mut engine = start_engine(
        context,
        gas_limit,
        &contract_code,
        &og_contract_key,
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
    og_contract_key: &ContractKey,
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
        *og_contract_key,
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
            trace!("env.query_depth: {:?}", env);
            env.query_depth
        })
}
