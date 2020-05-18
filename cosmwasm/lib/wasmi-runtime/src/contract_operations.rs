use crate::crypto::*;
use enclave_ffi_types::{Ctx, EnclaveError};
use log::*;
use parity_wasm::elements;
use wasmi::{ImportsBuilder, ModuleInstance};

use super::results::{HandleSuccess, InitSuccess, QuerySuccess};

use crate::crypto::key_manager::KEY_MANAGER;
use crate::errors::wasmi_error_to_enclave_error;
use crate::gas::{gas_rules, WasmCosts};
use crate::runtime::{Engine, EnigmaImportResolver, Runtime};

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

    // let output = encrypt_output(&output)?;

    Ok(InitSuccess {
        output,
        used_gas: engine.gas_used(),
        signature: [0; 65], // TODO this is needed anymore as output is alread authenticated
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

    // let output = encrypt_output(&output)?;

    Ok(HandleSuccess {
        output,
        used_gas: engine.gas_used(),
        signature: [0; 65], // TODO this is needed anymore as output is alread authenticated
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

    // let output = encrypt_output(&output)?;

    Ok(QuerySuccess {
        output,
        used_gas: engine.gas_used(),
        signature: [0; 65], // TODO this is needed anymore as output is alread authenticated
    })
}

fn decrypt_msg(msg: &[u8]) -> Result<Vec<u8>, EnclaveError> {
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

    // let (msg, aad) = msg.split_at(msg.len() - 89);

    // aad = last 89 bytes of msg
    // let aad = msg[(msg.len() - 89)..];
    // let msg = mag[0..(msg.len() - 89)];

    // pass
    let msg = key.decrypt(msg).map_err(|err| {
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

    let output = key.encrypt(output).map_err(|err| {
        error!("got an error while trying to encrypt the output: {}", err);
        EnclaveError::FailedSeal
    })?;

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
