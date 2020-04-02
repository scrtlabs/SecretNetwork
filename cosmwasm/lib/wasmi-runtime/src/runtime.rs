use std::borrow::ToOwned;
use std::cell::RefCell;

use wasmi::{
    memory_units, Error as InterpreterError, Externals, FuncInstance, FuncRef,
    MemoryDescriptor, MemoryInstance, MemoryRef, ModuleImportResolver, RuntimeArgs,
    RuntimeValue, Signature, Trap, ValueType, ModuleRef,
};

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
pub struct Runtime;

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

pub struct Engine {
    runtime: Runtime,
    instance: ModuleRef,
    imports: EnigmaImportResolver,
}

impl Engine {
    pub fn new(runtime: Runtime, instance: ModuleRef, imports: EnigmaImportResolver) -> Self {
        Self { runtime, instance, imports }
    }

    pub fn allocate(&mut self, len: u32) -> Result<u32, InterpreterError> {
        match self.instance
            .invoke_export(
                "allocate",
                &[RuntimeValue::I32(len as i32)],
                &mut self.runtime,
            )?
        {
            Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
            other => Err(InterpreterError::Value(format!("allocate method returned value which wasn't u32: {:?}", other))),
        }
    }

    pub fn memory(&self) -> MemoryRef {
        self.imports.memory_ref()
    }

    pub fn write_to_memory(&mut self, buffer: &[u8]) -> Result<u32, InterpreterError> {
        let pointer = self.allocate(buffer.len() as u32)?;
        self.memory().set(pointer, buffer)?;
        Ok(pointer)
    }

    pub fn extract_vector(&self, vec_ptr: u32) -> Result<Vec<u8>, InterpreterError> {
        let memory = self.memory();
        let ptr: u32 = memory.get_value(vec_ptr)?;
        let len: u32 = memory.get_value(vec_ptr + 32/8)?;

        memory.get(ptr, len as usize)
    }

    pub fn init(&mut self, env_ptr: u32, msg_ptr: u32) -> Result<u32, InterpreterError> {
        match self.instance
            .invoke_export(
                "init",
                &[
                    RuntimeValue::I32(env_ptr as i32),
                    RuntimeValue::I32(msg_ptr as i32),
                ],
                &mut self.runtime,
            )?
        {
            Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
            other => Err(InterpreterError::Value(format!("allocate method returned value which wasn't u32: {:?}", other))),
        }
    }
}

