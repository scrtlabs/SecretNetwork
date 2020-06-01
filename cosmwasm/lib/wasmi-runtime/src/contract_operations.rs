use base64;
use enclave_ffi_types::{Ctx, EnclaveError};
use log::*;
use parity_wasm::elements;
use serde_json::Value;
use wasmi::{ImportsBuilder, ModuleInstance};

use super::results::{HandleSuccess, InitSuccess, QuerySuccess};

use crate::crypto::{key_manager::KEY_MANAGER, AESKey, SIVEncryptable};
use crate::errors::wasmi_error_to_enclave_error;
use crate::gas::{gas_rules, WasmCosts};
use crate::runtime::{Engine, EnigmaImportResolver, Runtime};

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
    let mut engine = start_engine(context, gas_limit, contract)?;

    let env_ptr = engine
        .write_to_memory(env)
        .map_err(wasmi_error_to_enclave_error)?;

    let msg = decrypt_msg(msg)?;

    let msg_ptr = engine
        .write_to_memory(&msg)
        .map_err(wasmi_error_to_enclave_error)?;

    let vec_ptr = engine
        .init(env_ptr, msg_ptr)
        .map_err(wasmi_error_to_enclave_error)?;

    let output = engine
        .extract_vector(vec_ptr)
        .map_err(wasmi_error_to_enclave_error)?;

    let output = encrypt_output(&output)?;

    Ok(InitSuccess {
        output,
        used_gas: engine.gas_used(),
        signature: [0; 65], // TODO this is needed anymore as output is already authenticated
    })
}

pub fn handle(
    context: Ctx,
    gas_limit: u64,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
) -> Result<HandleSuccess, EnclaveError> {
    let mut engine = start_engine(context, gas_limit, contract)?;

    let env_ptr = engine
        .write_to_memory(env)
        .map_err(wasmi_error_to_enclave_error)?;

    let msg = decrypt_msg(msg)?;

    let msg_ptr = engine
        .write_to_memory(&msg)
        .map_err(wasmi_error_to_enclave_error)?;

    let vec_ptr = engine
        .handle(env_ptr, msg_ptr)
        .map_err(wasmi_error_to_enclave_error)?;

    let output = engine
        .extract_vector(vec_ptr)
        .map_err(wasmi_error_to_enclave_error)?;

    let output = encrypt_output(&output)?;

    Ok(HandleSuccess {
        output,
        used_gas: engine.gas_used(),
        signature: [0; 65], // TODO this is needed anymore as output is already authenticated
    })
}

pub fn query(
    context: Ctx,
    gas_limit: u64,
    contract: &[u8],
    msg: &[u8],
) -> Result<QuerySuccess, EnclaveError> {
    let mut engine = start_engine(context, gas_limit, contract)?;

    let msg = decrypt_msg(msg)?;

    let msg_ptr = engine
        .write_to_memory(&msg)
        .map_err(wasmi_error_to_enclave_error)?;

    let vec_ptr = engine
        .query(msg_ptr)
        .map_err(wasmi_error_to_enclave_error)?;

    let output = engine
        .extract_vector(vec_ptr)
        .map_err(wasmi_error_to_enclave_error)?;

    let output = encrypt_output(&output)?;

    Ok(QuerySuccess {
        output,
        used_gas: engine.gas_used(),
        signature: [0; 65], // TODO this is needed anymore as output is already authenticated
    })
}

fn decrypt_msg(msg: &[u8]) -> Result<Vec<u8>, EnclaveError> {
    // TODO check msg.len > 65

    let nonce = &msg[0..32]; // to then kdf with aes key
    let tx_sender_wallet_pubkey = &msg[32..65]; // 33 bytes compressed secp256k1 pubkey
    let encrypted_msg = &msg[65..];

    /////////////////////////////////// ASSAF TAKE IT FROM HERE
    // derive decryption key

    // TODO KEY_MANAGER should be initialized in the boot process and after that it'll never panic, if it panics on boot than the node is in a broken state and should panic
    let key = AESKey::new_from_slice(&[7_u8; 32]);

    // let (msg, aad) = msg.split_at(msg.len() - 89);

    // ad = first 33 bytes of msg
    // let ad = msg[(msg.len() - 89)..];
    // let msg = mag[0..(msg.len() - 89)];

    // pass
    let msg = key.decrypt_siv(encrypted_msg, &vec![&[]]).map_err(|err| {
        error!(
            "handle() got an error while trying to decrypt the msg: {}",
            err
        );
        EnclaveError::FailedUnseal
    })?;

    Ok(msg)
}

fn encrypt_output(output: &Vec<u8>) -> Result<Vec<u8>, EnclaveError> {
    // TODO:
    // extract "challenge" & "wallet_pubkey" from AAD
    // validate that "env.message.signer" is derived from "wallet_pubkey"
    // calculate "shared_key_base" from "ECDH(wallet_pubkey, sk_consensus_io_exchange_keypair)"
    // calculate "shared_key" from "HKDF(shared_key_base + challenge)"
    // decrypt(shared_key, msg)
    // ?? need to authenticate ADD or doest it happen inside decrypt ??

    // pseudo code:
    // [challenge, pk_wallet] = get_AAD(encrypted_input)
    // base_key = ECDH(sk_io, pk_wallet)
    // encryption_key = HKDF(base_key + challenge)
    // decrypted_input = decrypt(key=encryption_key, data=encrypted_input)
    // ?? need to authenticate ADD or doest it happen inside decrypt ??

    // TODO KEY_MANAGER should be initialized in the boot process and after that it'll never panic, if it panics on boot than the node is in a broken state and should panic

    let key = AESKey::new_from_slice(&[7_u8; 32]);

    debug!("@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@");
    debug!("before: {:?}", String::from_utf8_lossy(&output));
    debug!("@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@");

    let mut v: Value = serde_json::from_slice(output).map_err(|err| {
        error!(
            "got an error while trying to deserialize output bytes into json {:?}: {}",
            output, err
        );
        EnclaveError::FailedSeal
    })?;

    if let Value::String(err) = &v["err"] {
        v["err"] = Value::String(base64::encode(
            &key.encrypt_siv(&err.to_owned().into_bytes(), &vec![&[]])
                .map_err(|err| {
                    error!(
                        "got an error while trying to encrypt output error {:?}: {}",
                        err, err
                    );
                    EnclaveError::FailedSeal
                })?,
        ));
    } else if let Value::String(ok) = &v["ok"] {
        // query
        v["ok"] = Value::String(base64::encode(
            &key.encrypt_siv(&ok.to_owned().into_bytes(), &vec![&[]])
                .map_err(|err| {
                    error!(
                        "got an error while trying to encrypt query output {:?}: {}",
                        ok, err
                    );
                    EnclaveError::FailedSeal
                })?,
        ));
    } else if let Value::Object(ok) = &mut v["ok"] {
        // init of handle
        if let Value::Array(msgs) = &mut ok["messages"] {
            for msg in msgs {
                if let Value::String(msg_to_next_call) = &mut msg["contract"]["msg"] {
                    msg["contract"]["msg"] = Value::String(base64::encode(
                        &key.encrypt_siv(&msg_to_next_call.to_owned().into_bytes(), &vec![&[]])
                            .map_err(|err| {
                                error!(
                            "got an error while trying to encrypt the msg to next call {:?}: {}",
                            msg["contract"], err
                        );
                                EnclaveError::FailedSeal
                            })?,
                    ));
                }
            }
        }

        if let Value::Array(events) = &mut v["ok"]["log"] {
            for e in events {
                if let Value::String(k) = &mut e["key"] {
                    e["key"] = Value::String(base64::encode(
                        &key.encrypt_siv(&k.to_owned().into_bytes(), &vec![&[]])
                            .map_err(|err| {
                                error!(
                                    "got an error while trying to encrypt the event key {}: {}",
                                    k, err
                                );
                                EnclaveError::FailedSeal
                            })?,
                    ));
                }
                if let Value::String(v) = &mut e["value"] {
                    e["value"] = Value::String(base64::encode(
                        &key.encrypt_siv(&v.to_owned().into_bytes(), &vec![&[]])
                            .map_err(|err| {
                                error!(
                                    "got an error while trying to encrypt the event value {}: {}",
                                    v, err
                                );
                                EnclaveError::FailedSeal
                            })?,
                    ));
                }
            }
        }

        if let Value::String(data) = &mut v["ok"]["data"] {
            v["ok"]["data"] = Value::String(base64::encode(
                &key.encrypt_siv(&data.to_owned().into_bytes(), &vec![&[]])
                    .map_err(|err| {
                        error!(
                            "got an error while trying to encrypt the data section {}: {}",
                            data, err
                        );
                        EnclaveError::FailedSeal
                    })?,
            ));
        }
    }

    let output = serde_json::ser::to_vec(&v).map_err(|err| {
        error!(
            "got an error while trying to serialize output json into bytes {:?}: {}",
            v, err
        );
        EnclaveError::FailedSeal
    })?;

    debug!("%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%");
    debug!("after: {:?}", String::from_utf8_lossy(&output));
    debug!("%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%%");

    Ok(output)
}

fn start_engine(context: Ctx, gas_limit: u64, contract: &[u8]) -> Result<Engine, EnclaveError> {
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
    );

    Ok(Engine::new(runtime, instance))
}
