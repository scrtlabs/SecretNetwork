use log::*;

use enclave_ffi_types::{Ctx, EnclaveError};

use crate::coalesce;
use crate::cosmwasm_v010_types;
use crate::cosmwasm_v010_types::encoding::Binary;
use crate::cosmwasm_v010_types::types::CanonicalAddr;
use crate::cosmwasm_v016_types;
use crate::cosmwasm_v016_types::addresses::Addr;
use crate::cosmwasm_v016_types::timestamp::Timestamp;
use crate::crypto::Ed25519PublicKey;
use crate::results::{HandleSuccess, InitSuccess, QuerySuccess};
use crate::wasm::runtime::CosmWasmApiVersion;

use super::contract_validation::{
    extract_contract_key, generate_encryption_key, validate_contract_key, validate_msg,
    verify_params, ContractKey,
};
use super::gas::WasmCosts;
use super::io::encrypt_output;
use super::module_cache::create_module_instance;
use super::runtime::{ContractInstance, ContractOperation, Engine};
use super::types::{ContractCode, IoNonce, SecretMessage, SigInfo};

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
    let contract_code = ContractCode::new(contract);

    let mut env_v010: cosmwasm_v010_types::types::Env =
        serde_json::from_slice(env).map_err(|err| {
            warn!(
                "init got an error while trying to deserialize env input bytes into json {:?}: {}",
                String::from_utf8_lossy(&env),
                err
            );
            EnclaveError::FailedToDeserialize
        })?;
    env_v010.contract_code_hash = hex::encode(contract_code.hash());

    trace!("init env_v010: {:?}", env_v010);

    let canonical_contract_address = CanonicalAddr::from_human(&env_v010.contract.address).map_err(|err| {
        warn!(
            "init got an error while trying to deserialize env_v010.contract.address from bech32 string to bytes {:?}: {}",
            env_v010.contract.address, err
        );
        EnclaveError::FailedToDeserialize
    })?;
    let contract_key = generate_encryption_key(
        &env_v010,
        contract_code.hash(),
        &(canonical_contract_address.0).0,
    )?;
    trace!("init contract key: {:?}", hex::encode(contract_key));

    let parsed_sig_info: SigInfo = serde_json::from_slice(sig_info).map_err(|err| {
        warn!(
            "init got an error while trying to deserialize env input bytes into json {:?}: {}",
            String::from_utf8_lossy(&sig_info),
            err
        );
        EnclaveError::FailedToDeserialize
    })?;

    trace!("init input before decryption: {:?}", base64::encode(&msg));
    let secret_msg = SecretMessage::from_slice(msg)?;

    verify_params(&parsed_sig_info, &env_v010, &secret_msg)?;

    let decrypted_msg = secret_msg.decrypt()?;

    let validated_msg = validate_msg(&decrypted_msg, contract_code.hash())?;

    trace!(
        "init input after decryption: {:?}",
        String::from_utf8_lossy(&validated_msg)
    );

    let mut engine = start_engine(
        context,
        gas_limit,
        contract_code,
        &contract_key,
        ContractOperation::Init,
        secret_msg.nonce,
        secret_msg.user_public_key,
    )?;

    let (contract_env_bytes, contract_msg_info_bytes) =
        env_to_env_msg_info_bytes(&engine, &mut env_v010)?;

    let env_ptr = engine.write_to_memory(&contract_env_bytes)?;
    let msg_info_ptr = engine.write_to_memory(&contract_msg_info_bytes)?;
    let msg_ptr = engine.write_to_memory(&validated_msg)?;

    // This wrapper is used to coalesce all errors in this block to one object
    // so we can `.map_err()` in one place for all of them
    let output = coalesce!(EnclaveError, {
        let vec_ptr = engine.init(env_ptr, msg_info_ptr, msg_ptr)?;
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
    let contract_code = ContractCode::new(contract);

    let mut env_v010: cosmwasm_v010_types::types::Env =
        serde_json::from_slice(env).map_err(|err| {
            warn!(
            "handle got an error while trying to deserialize env input bytes into json {:?}: {}",
            env, err
        );
            EnclaveError::FailedToDeserialize
        })?;
    env_v010.contract_code_hash = hex::encode(contract_code.hash());

    trace!("handle env_v010: {:?}", env_v010);

    let contract_key = extract_contract_key(&env_v010)?;

    if !validate_contract_key(&contract_key, &env_v010.contract.address, contract)? {
        warn!("hande got an error while trying to validate contract key");
        return Err(EnclaveError::FailedContractAuthentication);
    }

    let parsed_sig_info: SigInfo = serde_json::from_slice(sig_info).map_err(|err| {
        warn!(
            "handle got an error while trying to deserialize env input bytes into json {:?}: {}",
            String::from_utf8_lossy(&sig_info),
            err
        );
        EnclaveError::FailedToDeserialize
    })?;

    trace!("handle input before decryption: {:?}", base64::encode(&msg));
    let secret_msg = SecretMessage::from_slice(msg)?;

    // Verify env parameters against the signed tx
    verify_params(&parsed_sig_info, &env_v010, &secret_msg)?;

    let secret_msg = SecretMessage::from_slice(msg)?;
    let decrypted_msg = secret_msg.decrypt()?;

    let validated_msg = validate_msg(&decrypted_msg, contract_code.hash())?;

    trace!(
        "handle input afer decryption: {:?}",
        String::from_utf8_lossy(&validated_msg)
    );

    trace!("successfully authenticated the contract!");
    trace!("handle contract key: {:?}", hex::encode(contract_key));

    let canonical_contract_address = CanonicalAddr::from_human(&env_v010.contract.address).map_err(|err| {
        warn!(
            "handle got an error while trying to deserialize env_v010.contract.address from bech32 string to bytes {:?}: {}",
            env_v010.contract.address, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    let mut engine = start_engine(
        context,
        gas_limit,
        contract_code,
        &contract_key,
        ContractOperation::Handle,
        secret_msg.nonce,
        secret_msg.user_public_key,
    )?;

    let (contract_env_bytes, contract_msg_info_bytes) =
        env_to_env_msg_info_bytes(&engine, &mut env_v010)?;

    let env_ptr = engine.write_to_memory(&contract_env_bytes)?;
    let msg_info_ptr = engine.write_to_memory(&contract_msg_info_bytes)?;
    let msg_ptr = engine.write_to_memory(&validated_msg)?;

    // This wrapper is used to coalesce all errors in this block to one object
    // so we can `.map_err()` in one place for all of them
    let output = coalesce!(EnclaveError, {
        let vec_ptr = engine.handle(env_ptr, msg_info_ptr, msg_ptr)?;

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
    env: &[u8],
    msg: &[u8],
) -> Result<QuerySuccess, EnclaveError> {
    let contract_code = ContractCode::new(contract);

    let mut env_v010: cosmwasm_v010_types::types::Env =
        serde_json::from_slice(env).map_err(|err| {
            warn!(
                "query got an error while trying to deserialize env input bytes into json {:?}: {}",
                env, err
            );
            EnclaveError::FailedToDeserialize
        })?;
    env_v010.contract_code_hash = hex::encode(contract_code.hash());

    trace!("query env_v010: {:?}", env_v010);

    let contract_key = extract_contract_key(&env_v010)?;

    if !validate_contract_key(&contract_key, &env_v010.contract.address, contract)? {
        warn!("query got an error while trying to validate contract key");
        return Err(EnclaveError::FailedContractAuthentication);
    }

    trace!("successfully authenticated the contract!");
    trace!("query contract key: {:?}", hex::encode(contract_key));

    trace!("query input before decryption: {:?}", base64::encode(&msg));
    let secret_msg = SecretMessage::from_slice(msg)?;
    let decrypted_msg = secret_msg.decrypt()?;
    trace!(
        "query input afer decryption: {:?}",
        String::from_utf8_lossy(&decrypted_msg)
    );
    let validated_msg = validate_msg(&decrypted_msg, contract_code.hash())?;

    let mut engine = start_engine(
        context,
        gas_limit,
        contract_code,
        &contract_key,
        ContractOperation::Query,
        secret_msg.nonce,
        secret_msg.user_public_key,
    )?;

    let (contract_env_bytes, _ /* no msg_info in query */) =
        env_to_env_msg_info_bytes(&engine, &mut env_v010)?;

    let env_ptr = engine.write_to_memory(&contract_env_bytes)?;
    let msg_ptr = engine.write_to_memory(&validated_msg)?;

    // This wrapper is used to coalesce all errors in this block to one object
    // so we can `.map_err()` in one place for all of them
    let output = coalesce!(EnclaveError, {
        let vec_ptr = engine.query(env_ptr, msg_ptr)?;

        let output = engine.extract_vector(vec_ptr)?;

        let output = encrypt_output(
            output,
            secret_msg.nonce,
            secret_msg.user_public_key,
            &CanonicalAddr(Binary(Vec::new())), // Not used for queries (no logs)
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
    contract_code: ContractCode,
    contract_key: &ContractKey,
    operation: ContractOperation,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
) -> Result<Engine, EnclaveError> {
    let module = create_module_instance(contract_code)?;

    // Set the gas costs for wasm op-codes (there is an inline stack_height limit in WasmCosts)
    let wasm_costs = WasmCosts::default();

    let contract_instance = ContractInstance::new(
        context,
        module.clone(),
        gas_limit,
        wasm_costs,
        *contract_key,
        operation,
        nonce,
        user_public_key,
    )?;

    Ok(Engine::new(contract_instance, module))
}

fn env_to_env_msg_info_bytes(
    engine: &Engine,
    env_v010: &mut cosmwasm_v010_types::types::Env,
) -> Result<(Vec<u8>, Vec<u8>), EnclaveError> {
    match engine.contract_instance.cosmwasm_api_version {
        CosmWasmApiVersion::V010 => {
            // Assaf: contract_key is irrelevant inside the contract,
            // but existing v0.10 contracts might expect it to be populated :facepalm:,
            // therefore we are going to leave it populated :shrug:.
            // env_v010.contract_key = None;

            // in v0.10 the timestamp passed from Go was unix time in seconds
            // v0.16 time is unix time in nanoseconds, so now Go passes here unix time in nanoseconds
            // but v0.10 contracts still expect time to be in unix seconds,
            // so we need to convert it from nanoseconds to seconds
            env_v010.block.time = Timestamp::from_nanos(env_v010.block.time).seconds();

            let env_v010_bytes = serde_json::to_vec(env_v010).map_err(|err| {
                warn!(
                    "got an error while trying to serialize env_v010 (cosmwasm v0.10) into bytes {:?}: {}",
                    env_v010, err
                );
                EnclaveError::FailedToSerialize
            })?;

            let msg_info_v010_bytes: Vec<u8> = vec![]; // in v0.10 msg_info is inside env

            Ok((env_v010_bytes, msg_info_v010_bytes))
        }
        CosmWasmApiVersion::V016 => {
            let env_v016 = cosmwasm_v016_types::types::Env {
                block: cosmwasm_v016_types::types::BlockInfo {
                    height: env_v010.block.height,
                    time: Timestamp::from_nanos(env_v010.block.time),
                    chain_id: env_v010.block.chain_id.clone(),
                },
                contract: cosmwasm_v016_types::types::ContractInfo {
                    address: Addr(env_v010.contract.address.0.clone()),
                    code_hash: env_v010.contract_code_hash.clone(),
                },
            };

            let env_v016_bytes =  serde_json::to_vec(&env_v016).map_err(|err| {
                warn!(
                    "got an error while trying to serialize env_v016 (cosmwasm v0.16) into bytes {:?}: {}",
                    env_v016, err
                );
                EnclaveError::FailedToSerialize
            })?;

            let msg_info_v016 = cosmwasm_v016_types::types::MessageInfo {
                sender: Addr(env_v010.message.sender.0.clone()),
                funds: env_v010
                    .message
                    .sent_funds
                    .iter()
                    .map(|coin| {
                        cosmwasm_v016_types::coins::Coin::new(
                            coin.amount.u128(),
                            coin.denom.clone(),
                        )
                    })
                    .collect::<Vec<cosmwasm_v016_types::coins::Coin>>(),
            };

            let msg_info_v016_bytes =  serde_json::to_vec(&msg_info_v016).map_err(|err| {
                warn!(
                    "got an error while trying to serialize msg_info_v016 (cosmwasm v0.16) into bytes {:?}: {}",
                    msg_info_v016, err
                );
                EnclaveError::FailedToSerialize
            })?;

            Ok((env_v016_bytes, msg_info_v016_bytes))
        }
    }
}
