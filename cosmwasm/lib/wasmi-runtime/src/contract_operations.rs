use enclave_ffi_types::{Ctx, EnclaveError, UserSpaceBuffer};

use super::imports;
use super::results::{HandleSuccess, InitSuccess, QuerySuccess};
use crate::exports;

use std::borrow::ToOwned;
use std::cell::RefCell;
use std::ptr;
use std::slice;

extern crate wasmi;
use wasmi::{
    memory_units, Error as InterpreterError, Externals, FuncInstance, FuncRef, ImportsBuilder,
    MemoryDescriptor, MemoryInstance, MemoryRef, ModuleImportResolver, ModuleInstance, RuntimeArgs,
    RuntimeValue, Signature, Trap, ValueType,
};

/// Safe wrapper around reads from the contract storage
fn read_db(context: Ctx, key: &[u8]) -> Option<Vec<u8>> {
    let _ = std::slice::from_raw_parts::<u8>;

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
    // Load wasm binary and prepare it for instantiation.
    let module = wasmi::Module::from_buffer(contract).map_err(|_err| EnclaveError::InvalidWasm)?;

    // Create new imports resolver.
    // These are the signatures of rust functions available to invoke from wasm code.
    let enigma_import_resolver = EnigmaImportResolver::with_limit(4 * 1024 * 1024 * 1024); // 4GiB
    let imports = ImportsBuilder::new().with_resolver("env", &enigma_import_resolver);

    // Instantiate a module with our imports and assert that there is no `start` function.
    let not_started_instance =
        ModuleInstance::new(&module, &imports).map_err(|_err| EnclaveError::InvalidWasm)?;
    if not_started_instance.has_start() {
        return Err(EnclaveError::WasmModuleWithStart);
    }
    let instance = not_started_instance.not_started_instance();

    let mut runtime = Runtime {};

    // Allocate memory inside WASM VM to house the `env` buffer
    let env_offset_in_instance_memory = match instance
        .invoke_export(
            "allocate",
            &[RuntimeValue::I32(env.len() as i32)],
            &mut runtime,
        )
        .map_err(|_err| EnclaveError::FailedFunctionCall)?
    {
        Some(RuntimeValue::I32(env_offset_in_instance_memory)) => env_offset_in_instance_memory,
        _ => panic!("TEST"), // TODO: return error here
    };

    let instance_memory_ref = enigma_import_resolver.memory_ref();
    instance_memory_ref
        .set(env_offset_in_instance_memory as u32, env)
        .expect("Error copying `env` into the memory region allocated for it by the WASM VM");

    // Allocate memory inside WASM VM to house the `msg` buffer
    let msg_offset_in_instance_memory = match instance
        .invoke_export(
            "allocate",
            &[RuntimeValue::I32(msg.len() as i32)],
            &mut runtime,
        )
        .map_err(|_err| EnclaveError::FailedFunctionCall)?
    {
        Some(RuntimeValue::I32(msg_offset_in_instance_memory)) => msg_offset_in_instance_memory,
        _ => panic!("TEST"), // TODO: return error here
    };

    instance_memory_ref
        .set(msg_offset_in_instance_memory as u32, msg)
        .expect("Error copying `msg` into the memory region allocated for it by the WASM VM");

    //.invoke_export("init" with both pointers that we got from allocate
    let x = instance
        .invoke_export(
            "init",
            &[
                RuntimeValue::I32(env_offset_in_instance_memory),
                RuntimeValue::I32(msg_offset_in_instance_memory),
            ],
            &mut runtime,
        )
        .map_err(|_err| EnclaveError::FailedFunctionCall)?; // TODO return _err to user

    todo!()
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

// --------------------------------
// Functions to expose to WASM code
// --------------------------------
// TODO find better name for `Runtime`

// EnigmaImportResolver maps function name to its function signature and also to function index in Runtime
// When instansiating a module we give it this resolver
// When invoking a function inside the module we can give it different runtimes (which we probably won't do)
#[derive(Debug, Clone)]
pub struct EnigmaImportResolver {
    max_memory: u32,
    memory: RefCell<MemoryRef>,
}

impl EnigmaImportResolver {
    /// New import resolver with specifed maximum amount of inital memory (in wasm pages = 64kb)
    pub fn with_limit(max_memory: u32) -> EnigmaImportResolver {
        EnigmaImportResolver {
            max_memory,
            memory: RefCell::new(
                MemoryInstance::alloc(
                    memory_units::Pages(0),
                    Some(memory_units::Pages(max_memory as usize)),
                )
                .expect("Reuven to fix this"),
            ),
        }
    }

    /// Returns memory that was instantiated during the contract module
    /// start. If contract does not use memory at all, the dummy memory of length (0, 0)
    /// will be created instead. So this method always returns memory instance
    /// unless errored.
    pub fn memory_ref(&self) -> MemoryRef {
        self.memory.borrow().clone()
    }

    /// Returns current memory that is in use
    pub fn memory_size(&self) -> Result<u32, InterpreterError> {
        Ok(self.memory_ref().current_size().0 as u32)
    }
}

// These functions should be available to invoke from wasm code
// These should pass the request up to go-cosmwasm:
// fn read_db(key: *const c_void, value: *mut c_void) -> i32;
// fn write_db(key: *const c_void, value: *mut c_void);
// These should be implemented here: + TODO: Check Cosmwasm implementation for these:
// fn canonicalize_address(human: *const c_void, canonical: *mut c_void) -> i32;
// fn humanize_address(canonical: *const c_void, human: *mut c_void) -> i32;
impl ModuleImportResolver for EnigmaImportResolver {
    fn resolve_memory(
        &self,
        field_name: &str,
        descriptor: &MemoryDescriptor,
    ) -> Result<MemoryRef, InterpreterError> {
        if field_name == "memory" {
            let effective_max = descriptor.maximum().unwrap_or(self.max_memory + 1);
            if descriptor.initial() > self.max_memory || effective_max > self.max_memory {
                Err(InterpreterError::Instantiation(
                    "Module requested too much memory".to_owned(),
                ))
            } else {
                let mem = MemoryInstance::alloc(
                    memory_units::Pages(descriptor.initial() as usize),
                    descriptor
                        .maximum()
                        .map(|x| memory_units::Pages(x as usize)),
                )?;
                *self.memory.borrow_mut() = mem.clone();
                Ok(mem)
            }
        } else {
            Err(InterpreterError::Instantiation(
                "Memory imported under unknown name".to_owned(),
            ))
        }
    }

    fn resolve_func(
        &self,
        func_name: &str,
        signature: &Signature,
    ) -> Result<FuncRef, InterpreterError> {
        let func_ref = match func_name {
            "read_db" => FuncInstance::alloc_host(
                Signature::new(&[][..], Some(ValueType::I32)),
                READ_DB_INDEX,
            ),
            "write_db" => FuncInstance::alloc_host(Signature::new(&[][..], None), WRITE_DB_INDEX),
            "canonicalize_address" => FuncInstance::alloc_host(
                Signature::new(&[][..], Some(ValueType::I32)),
                CANONICALIZE_ADDRESS_INDEX,
            ),
            "humanize_address" => FuncInstance::alloc_host(
                Signature::new(&[][..], Some(ValueType::I32)),
                HUMANIZE_ADDRESS_INDEX,
            ),
            _ => {
                return Err(InterpreterError::Function(format!(
                    "host module doesn't export function with name {}",
                    func_name
                )));
            }
        };
        Ok(func_ref)
    }
}

// Runtime maps function index to implementation
// When instansiating a module we give it the EnigmaImportResolver resolver
// When invoking a function inside the module we give it this runtime which is the acctual functions implementation ()
struct Runtime;

const READ_DB_INDEX: usize = 0;
const WRITE_DB_INDEX: usize = 1;
const CANONICALIZE_ADDRESS_INDEX: usize = 2;
const HUMANIZE_ADDRESS_INDEX: usize = 3;

impl Externals for Runtime {
    fn invoke_index(
        &mut self,
        index: usize,
        args: RuntimeArgs,
    ) -> Result<Option<RuntimeValue>, Trap> {
        match index {
            READ_DB_INDEX => Ok(Some(RuntimeValue::I32(2))), // TODO implement
            WRITE_DB_INDEX => Ok(Some(RuntimeValue::I32(2))), // TODO implement
            CANONICALIZE_ADDRESS_INDEX => Ok(Some(RuntimeValue::I32(2))), // TODO implement
            HUMANIZE_ADDRESS_INDEX => Ok(Some(RuntimeValue::I32(2))), // TODO implement
            _ => panic!("unknown function index"),
        }
    }
}
