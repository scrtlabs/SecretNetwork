use enclave_ffi_types::{Ctx, EnclaveError};

use super::imports;
use super::results::{HandleSuccess, InitSuccess, QuerySuccess};
use crate::exports;


use wasmi::{
    ImportsBuilder, ModuleInstance
};

use crate::runtime::{EnigmaImportResolver, Runtime, Engine};

/// Safe wrapper around reads from the contract storage
fn read_db(context: Ctx, key: &[u8]) -> Option<Vec<u8>> {
    unsafe { exports::recover_buffer(imports::ocall_read_db(context, key.as_ptr(), key.len())) }
}

/// Safe wrapper around writes to the contract storage
fn write_db(context: Ctx, key: &[u8], value: &[u8]) {
    unsafe {
        imports::ocall_write_db(
            context,
            key.as_ptr(),
            key.len(),
            value.as_ptr(),
            value.len(),
        )
    }
}

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
    contract: &[u8], // contract wasm bytes
    env: &[u8],      // blockchain state
    msg: &[u8],      // probably function call and args
) -> Result<InitSuccess, EnclaveError> {
    let mut engine = start_engine(contract)?;

    let env_ptr = engine.write_to_memory(env).map_err(|_err| EnclaveError::FailedFunctionCall)?;
    let msg_ptr = engine.write_to_memory(msg).map_err(|_err| EnclaveError::FailedFunctionCall)?;

    //.invoke_export("init" with both pointers that we got from allocate
    let vec_ptr = engine.init(env_ptr, msg_ptr).map_err(|_err| EnclaveError::FailedFunctionCall)?;

    let output = engine.extract_vector(vec_ptr).map_err(|_err| EnclaveError::FailedFunctionCall)?;
    Ok(InitSuccess{
        output,
        used_gas: 0,
        signature: [0; 65],
    })
}

pub fn handle(
    context: Ctx,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
) -> Result<HandleSuccess, EnclaveError> {
    todo!()
    // init wasmi - maybe the same as init for now?
}

pub fn query(context: Ctx, contract: &[u8], msg: &[u8]) -> Result<QuerySuccess, EnclaveError> {
    todo!()
    // init wasmi - maybe the same as init for now except env?
}

fn start_engine(contract: &[u8]) -> Result<Engine, EnclaveError> {
    // Load wasm binary and prepare it for instantiation.
    let module = wasmi::Module::from_buffer(contract).map_err(|_err| EnclaveError::InvalidWasm)?;

    // Create new imports resolver.
    // These are the signatures of rust functions available to invoke from wasm code.
    let imports = EnigmaImportResolver::with_limit(4 * 1024 * 1024 * 1024); // 4GiB
    let module_imports = ImportsBuilder::new().with_resolver("env", &imports);

    // Instantiate a module with our imports and assert that there is no `start` function.
    let instance =
        ModuleInstance::new(&module, &module_imports).map_err(|_err| EnclaveError::InvalidWasm)?;
    if instance.has_start() {
        return Err(EnclaveError::WasmModuleWithStart)
    }
    let instance = instance.not_started_instance().clone();
    let runtime = Runtime;

    Ok(Engine::new(runtime, instance, imports))
}
