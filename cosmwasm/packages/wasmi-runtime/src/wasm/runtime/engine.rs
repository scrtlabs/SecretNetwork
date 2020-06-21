use log::*;
use wasmi::{Error as InterpreterError, MemoryRef, ModuleRef, RuntimeValue};

use super::contract::ContractInstance;

pub struct Engine {
    contract_instance: ContractInstance,
    module: ModuleRef,
}

impl Engine {
    pub fn new(contract_instance: ContractInstance, module: ModuleRef) -> Self {
        Self {
            contract_instance,
            module,
        }
    }

    pub fn gas_used(&self) -> u64 {
        self.contract_instance.gas_used
    }

    pub fn allocate(&mut self, len: u32) -> Result<u32, InterpreterError> {
        match self.module.invoke_export(
            "allocate",
            &[RuntimeValue::I32(len as i32)],
            &mut self.contract_instance,
        )? {
            Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
            other => Err(InterpreterError::Value(format!(
                "allocate method returned value which wasn't u32: {:?}",
                other
            ))),
        }
    }

    pub fn memory(&self) -> MemoryRef {
        self.module
            .export_by_name("memory")
            .expect("Module expected to have 'memory' export")
            .as_memory()
            .cloned()
            .expect("'memory' export should be a memory")
    }

    pub fn write_to_memory(&mut self, buffer: &[u8]) -> Result<u32, InterpreterError> {
        // WASM pointers are pointers to "Region"
        // Region is a struct that looks like this:
        // ptr_to_region -> | 4byte = buffer_addr | 4bytes = buffer_len |

        // allocate return a poiter to a region
        let ptr_to_region_in_wasm_vm = self.allocate(buffer.len() as u32)?;

        // extract the buffer pointer from the region
        let buffer_addr_in_wasm: u32 = self.memory().get_value::<u32>(ptr_to_region_in_wasm_vm)?;

        let buffer_len_in_wasm: u32 = self
            .memory()
            .get_value::<u32>(ptr_to_region_in_wasm_vm + 4)?;
        if buffer_len_in_wasm != buffer.len() as u32 {
            // TODO return an Error? Or maybe this is already covered by allocate?
        }

        self.memory().set(buffer_addr_in_wasm, buffer)?;

        // return the WASM pointer
        Ok(ptr_to_region_in_wasm_vm)
    }

    pub fn extract_vector(&self, vec_ptr_ptr: u32) -> Result<Vec<u8>, InterpreterError> {
        let ptr: u32 = self.memory().get_value(vec_ptr_ptr)?;
        let len: u32 = self.memory().get_value(vec_ptr_ptr + 4)?;

        self.memory().get(ptr, len as usize)
    }

    pub fn init(&mut self, env_ptr: u32, msg_ptr: u32) -> Result<u32, InterpreterError> {
        trace!("Invoking init() in wasm");

        match self.module.invoke_export(
            "init",
            &[
                RuntimeValue::I32(env_ptr as i32),
                RuntimeValue::I32(msg_ptr as i32),
            ],
            &mut self.contract_instance,
        )? {
            Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
            other => Err(InterpreterError::Value(format!(
                "init method returned value which wasn't u32: {:?}",
                other
            ))),
        }
    }

    pub fn handle(&mut self, env_ptr: u32, msg_ptr: u32) -> Result<u32, InterpreterError> {
        trace!("Invoking handle() in wasm");

        match self.module.invoke_export(
            "handle",
            &[
                RuntimeValue::I32(env_ptr as i32),
                RuntimeValue::I32(msg_ptr as i32),
            ],
            &mut self.contract_instance,
        )? {
            Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
            other => Err(InterpreterError::Value(format!(
                "handle method returned value which wasn't u32: {:?}",
                other
            ))),
        }
    }

    pub fn query(&mut self, msg_ptr: u32) -> Result<u32, InterpreterError> {
        trace!("Invoking query() in wasm");

        match self.module.invoke_export(
            "query",
            &[RuntimeValue::I32(msg_ptr as i32)],
            &mut self.contract_instance,
        )? {
            Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
            other => Err(InterpreterError::Value(format!(
                "query method returned value which wasn't u32: {:?}",
                other
            ))),
        }
    }
}
