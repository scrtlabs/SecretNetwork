use log::*;
use wasmi::{ModuleRef, RuntimeValue};

use super::contract::ContractInstance;
use crate::wasm::errors::{wasmi_error_to_enclave_error, WasmEngineError};
use enclave_ffi_types::EnclaveError;

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

    pub fn write_to_memory(&mut self, buffer: &[u8]) -> Result<u32, WasmEngineError> {
        self.contract_instance.write_to_memory(buffer)
    }

    pub fn extract_vector(&self, vec_ptr_ptr: u32) -> Result<Vec<u8>, WasmEngineError> {
        self.contract_instance.extract_vector(vec_ptr_ptr)
    }

    pub fn init(&mut self, env_ptr: u32, msg_ptr: u32) -> Result<u32, EnclaveError> {
        trace!("Invoking init() in wasm");

        match self
            .module
            .invoke_export(
                "init",
                &[
                    RuntimeValue::I32(env_ptr as i32),
                    RuntimeValue::I32(msg_ptr as i32),
                ],
                &mut self.contract_instance,
            )
            .map_err(wasmi_error_to_enclave_error)?
        {
            Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
            other => {
                error!("init method returned value which wasn't u32: {:?}", other);
                Err(EnclaveError::FailedFunctionCall)
            }
        }
    }

    pub fn handle(&mut self, env_ptr: u32, msg_ptr: u32) -> Result<u32, EnclaveError> {
        trace!("Invoking handle() in wasm");

        match self
            .module
            .invoke_export(
                "handle",
                &[
                    RuntimeValue::I32(env_ptr as i32),
                    RuntimeValue::I32(msg_ptr as i32),
                ],
                &mut self.contract_instance,
            )
            .map_err(wasmi_error_to_enclave_error)?
        {
            Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
            other => {
                error!("handle method returned value which wasn't u32: {:?}", other);
                Err(EnclaveError::FailedFunctionCall)
            }
        }
    }

    pub fn query(&mut self, msg_ptr: u32) -> Result<u32, EnclaveError> {
        trace!("Invoking query() in wasm");

        match self
            .module
            .invoke_export(
                "query",
                &[RuntimeValue::I32(msg_ptr as i32)],
                &mut self.contract_instance,
            )
            .map_err(wasmi_error_to_enclave_error)?
        {
            Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
            other => {
                error!("query method returned value which wasn't u32: {:?}", other);
                Err(EnclaveError::FailedFunctionCall)
            }
        }
    }
}
