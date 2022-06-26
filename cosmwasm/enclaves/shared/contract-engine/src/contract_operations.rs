use log::*;

use enclave_ffi_types::{Ctx, EnclaveError};

use crate::external::results::{HandleSuccess, InitSuccess, QuerySuccess};
use crate::wasm::CosmWasmApiVersion;
use cosmos_proto::tx::signing::SignMode;
use cosmwasm_v010_types::types::CanonicalAddr;
use cosmwasm_v016_types::addresses::Addr;
use cosmwasm_v016_types::results::{DecryptedReply, Event, Reply, SubMsgResponse, SubMsgResult};
use cosmwasm_v016_types::timestamp::Timestamp;
use enclave_cosmos_types::types::{ContractCode, HandleType, SigInfo};
use enclave_cosmwasm_types as cosmwasm_v010_types;
use enclave_cosmwasm_types::encoding::Binary;
use enclave_cosmwasm_v016_types as cosmwasm_v016_types;

use enclave_crypto::{Ed25519PublicKey, HASH_SIZE};
use enclave_utils::coalesce;

use super::contract_validation::{
    extract_contract_key, generate_encryption_key, validate_contract_key, validate_msg,
    verify_params, ContractKey,
};
use super::gas::WasmCosts;
use super::io::encrypt_output;
use super::module_cache::create_module_instance;
use super::types::{IoNonce, SecretMessage};
use super::wasm::{ContractInstance, ContractOperation, Engine};

const HEX_ENCODED_HASH_SIZE: usize = HASH_SIZE * 2;

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

    let (validated_msg, reply_recipient_contract_hash) =
        validate_msg(&decrypted_msg, contract_code.hash())?;

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
            &env_v010.contract_code_hash,
            reply_recipient_contract_hash,
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

pub struct TaggedBool {
    b: bool,
}

impl From<bool> for TaggedBool {
    fn from(b: bool) -> Self {
        TaggedBool { b }
    }
}

impl Into<bool> for TaggedBool {
    fn into(self) -> bool {
        self.b
    }
}

type ShouldValidateSigInfo = TaggedBool;
type WasMessageEncrypted = TaggedBool;

pub fn reduct_custom_events(reply: &mut Reply) {
    reply.result = match &reply.result {
        SubMsgResult::Ok(r) => {
            let mut events: Vec<Event> = Default::default();
            let filtered_types = vec![
                "execute".to_string(),
                "instantiate".to_string(),
                "wasm".to_string(),
            ];
            let filtered_attributes = vec!["contract_address".to_string(), "code_id".to_string()];
            for ev in r.events.iter() {
                if filtered_types.contains(&ev.ty) {
                    let mut had_match = false;
                    for attr in &ev.attributes {
                        if filtered_attributes.contains(&attr.key) {
                            had_match = true;
                            break;
                        }
                    }

                    if had_match {
                        continue;
                    }
                }

                events.push(ev.clone());
            }

            SubMsgResult::Ok(SubMsgResponse {
                events,
                data: r.data.clone(),
            })
        }
        SubMsgResult::Err(_) => reply.result.clone(),
    };
}

// Parse the message that was passed to handle (Based on the assumption that it might be a reply or IBC as well)
pub fn parse_message(
    message: &[u8],
    sig_info: &SigInfo,
    handle_type: &HandleType,
) -> Result<
    (
        ShouldValidateSigInfo,
        WasMessageEncrypted,
        SecretMessage,
        Vec<u8>,
    ),
    EnclaveError,
> {
    let orig_secret_msg = SecretMessage::from_slice(message)?;

    return match handle_type {
        HandleType::HANDLE_TYPE_EXECUTE => {
            trace!(
                "handle input before decryption: {:?}",
                base64::encode(&message)
            );
            let decrypted_msg = orig_secret_msg.decrypt()?;
            Ok((
                ShouldValidateSigInfo::from(true),
                WasMessageEncrypted::from(true),
                orig_secret_msg,
                decrypted_msg,
            ))
        }

        HandleType::HANDLE_TYPE_REPLY => {
            if sig_info.sign_mode == SignMode::SIGN_MODE_UNSPECIFIED {
                trace!("reply input is not encrypted");
                let decrypted_msg = orig_secret_msg.msg.clone();
                let mut reply: Reply = serde_json::from_slice(&decrypted_msg)
                    .map_err(|err| {
                        warn!(
                "reply got an error while trying to deserialize decrypted reply bytes into json {:?}: {}",
                String::from_utf8_lossy(&decrypted_msg),
                err
            );
                        EnclaveError::FailedToDeserialize
                    })?;

                let mut msg_id: [u8; 8] = [0; 8];
                msg_id.copy_from_slice(&reply.id.as_slice()[0..8]);
                let decrypted_reply = DecryptedReply {
                    id: u64::from_be_bytes(msg_id),
                    result: reply.result.clone(),
                };

                reduct_custom_events(&mut reply);
                let serialized_encrypted_reply : Vec<u8> = serde_json::to_vec(&reply).map_err(|err| {
                    warn!(
                        "got an error while trying to serialize encrypted reply into bytes {:?}: {}",
                        reply, err
                    );
                    EnclaveError::FailedToSerialize
                })?;

                let reply_secret_msg = SecretMessage {
                    nonce: orig_secret_msg.nonce,
                    user_public_key: orig_secret_msg.user_public_key,
                    msg: serialized_encrypted_reply,
                };

                trace!("LIORRRR id2 {}", decrypted_reply.id);

                let serialized_reply: Vec<u8> = serde_json::to_vec(&decrypted_reply).map_err(|err| {
                    warn!(
                        "got an error while trying to serialize decrypted reply into bytes {:?}: {}",
                        decrypted_reply, err
                    );
                    EnclaveError::FailedToSerialize
                })?;

                return Ok((
                    ShouldValidateSigInfo::from(false),
                    WasMessageEncrypted::from(false),
                    reply_secret_msg,
                    serialized_reply,
                ));
            }

            // Here we are sure the reply is OK because only OK is encrypted
            trace!(
                "reply input before decryption: {:?}",
                base64::encode(&message)
            );
            let mut parsed_encrypted_reply: Reply = serde_json::from_slice(
                &orig_secret_msg.msg.as_slice()[HEX_ENCODED_HASH_SIZE..].to_vec(),
            )
            .map_err(|err| {
                warn!(
            "reply got an error while trying to deserialize msg input bytes into json {:?}: {}",
            String::from_utf8_lossy(&orig_secret_msg.msg),
            err
            );
                EnclaveError::FailedToDeserialize
            })?;

            trace!("LIORRRRR reply is {:?}", parsed_encrypted_reply);
            match parsed_encrypted_reply.result.clone() {
                SubMsgResult::Ok(response) => {
                    let decrypted_msg_data = match response.data {
                        Some(data) => {
                            let tmp_secret_msg_data = SecretMessage {
                                nonce: orig_secret_msg.nonce,
                                user_public_key: orig_secret_msg.user_public_key,
                                msg: data.as_slice().to_vec(),
                            };

                            Some(Binary(tmp_secret_msg_data.decrypt()?))
                        }
                        None => None,
                    };

                    trace!("LIORR HEREEE");

                    let tmp_secret_msg_id = SecretMessage {
                        nonce: orig_secret_msg.nonce,
                        user_public_key: orig_secret_msg.user_public_key,
                        msg: parsed_encrypted_reply.id.as_slice().to_vec(),
                    };

                    let tmp_decrypted_msg_id = tmp_secret_msg_id.decrypt()?;

                    trace!(
                        "LIORRRRR msg {:?} part {:?}",
                        orig_secret_msg.msg,
                        orig_secret_msg.msg.as_slice()[0..HEX_ENCODED_HASH_SIZE].to_vec()
                    );

                    let tmp_secret_msg_contract_hash = SecretMessage {
                        nonce: orig_secret_msg.nonce,
                        user_public_key: orig_secret_msg.user_public_key,
                        msg: orig_secret_msg.msg.as_slice()[0..HEX_ENCODED_HASH_SIZE].to_vec(),
                    };

                    let mut tmp_decrypted_msg_contract_hash =
                        tmp_secret_msg_contract_hash.decrypt()?;

                    trace!("LIORRR I WISH TO BE HERE");
                    // Now we need to create synthetic SecretMessage to fit the API in "handle"
                    let result = SubMsgResult::Ok(SubMsgResponse {
                        events: response.events,
                        data: decrypted_msg_data,
                    });

                    let mut msg_id: [u8; 8] = [0; 8];
                    msg_id.copy_from_slice(&tmp_decrypted_msg_id[0..8]);
                    let decrypted_reply = DecryptedReply {
                        id: u64::from_be_bytes(msg_id),
                        result,
                    };

                    trace!("LIORRRR id1 {}", decrypted_reply.id);

                    let decrypted_reply_as_vec =
                        serde_json::to_vec(&decrypted_reply).map_err(|err| {
                            warn!(
                                "got an error while trying to serialize reply into bytes {:?}: {}",
                                decrypted_reply, err
                            );
                            EnclaveError::FailedToSerialize
                        })?;

                    reduct_custom_events(&mut parsed_encrypted_reply);
                    let serialized_encrypted_reply : Vec<u8> = serde_json::to_vec(&parsed_encrypted_reply).map_err(|err| {
                    warn!(
                        "got an error while trying to serialize encrypted reply into bytes {:?}: {}",
                        parsed_encrypted_reply, err
                    );
                    EnclaveError::FailedToSerialize
                })?;

                    tmp_decrypted_msg_contract_hash.extend(serialized_encrypted_reply);

                    let reply_secret_msg = SecretMessage {
                        nonce: orig_secret_msg.nonce,
                        user_public_key: orig_secret_msg.user_public_key,
                        msg: tmp_decrypted_msg_contract_hash,
                    };

                    Ok((
                        ShouldValidateSigInfo::from(true),
                        WasMessageEncrypted::from(true),
                        reply_secret_msg,
                        decrypted_reply_as_vec,
                    ))
                }
                SubMsgResult::Err(_) => {
                    warn!("got an error while trying to deserialize reply, error should not be encrypted");
                    Err(EnclaveError::FailedToDeserialize)
                }
            }
        }
    };
}

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

    let canonical_contract_address = CanonicalAddr::from_human(&env_v010.contract.address).map_err(|err| {
        warn!(
            "got an error while trying to deserialize env_v010.contract.address from bech32 string to bytes {:?}: {}",
            env_v010.contract.address, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    let contract_key = extract_contract_key(&env_v010)?;

    if !validate_contract_key(&contract_key, &canonical_contract_address, &contract_code) {
        warn!("got an error while trying to deserialize output bytes");
        return Err(EnclaveError::FailedContractAuthentication);
    }

    let parsed_sig_info: SigInfo = serde_json::from_slice(sig_info).map_err(|err| {
        warn!(
            "handle got an error while trying to deserialize sig info input bytes into json {:?}: {}",
            String::from_utf8_lossy(&sig_info),
            err
        );
        EnclaveError::FailedToDeserialize
    })?;

    // The flow of handle is now used for multiple messages (such ash Handle, Reply)
    // When the message is handle, we expect it always to be encrypted while in Reply for example it might be plaintext
    let parsed_handle_type = HandleType::try_from(handle_type)?;

    let (should_validate_sig_info, was_msg_encrypted, secret_msg, decrypted_msg) =
        parse_message(msg, &parsed_sig_info, &parsed_handle_type)?;

    // There is no signature to verify when the input isn't signed.
    // Receiving unsigned messages is only possible in Handle. (Init tx are always signed)
    // All of these functions go through handle but the data isn't signed:
    //  Reply (that is not WASM reply)
    if should_validate_sig_info.into() {
        // Verify env parameters against the signed tx
        verify_params(&parsed_sig_info, &env_v010, &secret_msg)?;
    }

    let mut validated_msg = decrypted_msg.clone();
    let mut reply_params: Option<(Vec<u8>, u64)> = None;
    if was_msg_encrypted.into() {
        let x = validate_msg(&decrypted_msg, contract_code.hash())?;
        validated_msg = x.0;
        reply_params = x.1;
    }

    trace!(
        "handle input afer decryption: {:?}",
        String::from_utf8_lossy(&validated_msg)
    );

    trace!("Successfully authenticated the contract!");

    trace!("Handle: Contract Key: {:?}", hex::encode(contract_key));

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

    let (contract_env_bytes, contract_msg_info_bytes) =
        env_to_env_msg_info_bytes(&engine, &mut env_v010)?;

    let env_ptr = engine.write_to_memory(&contract_env_bytes)?;
    let msg_info_ptr = engine.write_to_memory(&contract_msg_info_bytes)?;
    let msg_ptr = engine.write_to_memory(&validated_msg)?;

    // This wrapper is used to coalesce all errors in this block to one object
    // so we can `.map_err()` in one place for all of them
    let output = coalesce!(EnclaveError, {
        let vec_ptr = engine.handle(env_ptr, msg_info_ptr, msg_ptr, parsed_handle_type)?;

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
            &env_v010.contract_code_hash,
            reply_params,
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

    let canonical_contract_address = CanonicalAddr::from_human(&env_v010.contract.address).map_err(|err| {
        warn!(
            "got an error while trying to deserialize env_v010.contract.address from bech32 string to bytes {:?}: {}",
            env_v010.contract.address, err
        );
        EnclaveError::FailedToDeserialize
    })?;

    let contract_key = extract_contract_key(&env_v010)?;

    if !validate_contract_key(&contract_key, &canonical_contract_address, &contract_code) {
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
    let validated_msg = validate_msg(&decrypted_msg, contract_code.hash())?.0;

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
            &CanonicalAddr(Binary(Vec::new())), // Not used for queries (can't init a new contract from a query)
            &"".to_string(), // Not used for queries (can't call a sub-message from a query),
            None,            // Not used for queries (Query response is not replied to the caller),
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
    let module = create_module_instance(contract_code, operation)?;

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
        CosmWasmApiVersion::V1 => {
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
