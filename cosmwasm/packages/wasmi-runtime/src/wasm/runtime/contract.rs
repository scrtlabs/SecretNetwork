use bech32::{FromBase32, ToBase32};
use log::*;
use wasmi::{Error as InterpreterError, MemoryInstance, MemoryRef, ModuleRef, RuntimeValue, Trap};

use enclave_ffi_types::Ctx;

use crate::consts::BECH32_PREFIX_ACC_ADDR;
use crate::wasm::contract_validation::ContractKey;
use crate::wasm::db::{read_encrypted_key, write_encrypted_key};
use crate::wasm::errors::WasmEngineError;
use crate::wasm::runtime::traits::WasmiApi;

/// SecretContract maps function index to implementation
/// When instantiating a module we give it the SecretNetworkImportResolver resolver
/// When invoking a function inside the module we give it this runtime which is the actual functions implementation ()
pub struct ContractInstance {
    pub context: Ctx,
    pub memory: MemoryRef,
    pub gas_limit: u64,
    pub gas_used: u64,
    pub contract_key: ContractKey,
    pub module: ModuleRef,
}

impl ContractInstance {
    fn get_memory(&self) -> &MemoryInstance {
        &*self.memory
    }

    pub fn new(context: Ctx, module: ModuleRef, gas_limit: u64, contract_key: ContractKey) -> Self {
        let memory = (&*module)
            .export_by_name("memory")
            .expect("Module expected to have 'memory' export")
            .as_memory()
            .cloned()
            .expect("'memory' export should be of memory type");

        Self {
            context,
            memory,
            gas_limit,
            gas_used: 0,
            contract_key,
            module,
        }
    }
    /// extract_vector extracts key into a buffer
    fn extract_vector(&self, vec_ptr_ptr: u32) -> Result<Vec<u8>, InterpreterError> {
        let ptr: u32 = self.get_memory().get_value(vec_ptr_ptr)?;
        let len: u32 = self.get_memory().get_value(vec_ptr_ptr + 8)?;

        self.get_memory().get(ptr, len as usize)
    }

    pub fn allocate(&mut self, len: u32) -> Result<u32, InterpreterError> {
        match self.module.clone().invoke_export(
            "allocate",
            &[RuntimeValue::I32(len as i32)],
            self,
        )? {
            Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
            other => Err(InterpreterError::Value(format!(
                "allocate method returned value which wasn't u32: {:?}",
                other
            ))),
        }
    }
}

impl WasmiApi for ContractInstance {
    /// Args:
    /// 1. "key" to read from Tendermint (buffer of bytes)
    /// key is a pointer to a region "struct" of "pointer" and "length"
    /// A Region looks like { ptr: u32, len: u32 }
    fn read_db_index(&mut self, state_key_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap> {
        let state_key_name = self
            .extract_vector(state_key_ptr_ptr as u32)
            .map_err(|err| {
                error!(
                    "read_db() error while trying to read state_key_name from wasm memory: {:?}",
                    err
                );
                WasmEngineError::MemoryReadError
            })?;

        trace!(
            "read_db() was called from WASM code with state_key_name: {:?}",
            String::from_utf8_lossy(&state_key_name)
        );

        // Call read_db (this bubbles up to Tendermint via ocalls and FFI to Go code)
        // This returns the value from Tendermint
        let value = read_encrypted_key(&state_key_name, &self.context, &self.contract_key)
            .map_err(WasmEngineError::from)?;

        let value = match value {
            None => return Ok(Some(RuntimeValue::I32(0))),
            Some(value) => value,
        };

        let value_ptr_ptr = match self.allocate(value.len() as u32).map_err(|err| {
            error!(
                "read_db() error while trying to allocate {} bytes for the value: {:?}",
                value.len(),
                err,
            );
            WasmEngineError::MemoryAllocationError
        })? {
            0 => return Err(WasmEngineError::MemoryAllocationError.into()),
            value_ptr_ptr => value_ptr_ptr as i32,
        };

        // Get pointer to the buffer (this was allocated in WASM)
        let value_ptr_in_wasm = self
            .get_memory()
            .get_value::<u32>(value_ptr_ptr as u32)
            .map_err(|err| {
                error!(
                    "read_db() error while trying to get pointer for the result buffer: {:?}",
                    err,
                );
                WasmEngineError::MemoryReadError
            })?;

        // Get length of the buffer (this was allocated in WASM)
        let value_len_in_wasm = self
            .get_memory()
            .get_value::<u32>((value_ptr_ptr + 8) as u32)
            .map_err(|err| {
                error!(
                    "read_db() error while trying to get length of result buffer: {:?}",
                    err
                );
                WasmEngineError::MemoryReadError
            })?;

        // Check that value is not too big to write into the allocated buffer
        if value_len_in_wasm < value.len() as u32 {
            error!(
                "read_db() result to big ({} bytes) to write to allocated wasm buffer ({} bytes)",
                value.len(),
                value_len_in_wasm
            );
            return Err(WasmEngineError::MemoryAllocationError.into());
        }

        // Write value returned from read_db to WASM memory
        self.get_memory()
            .set(value_ptr_in_wasm, &value)
            .map_err(|err| {
                error!(
                    "read_db() error while trying to write to result buffer: {:?}",
                    err
                );
                WasmEngineError::MemoryWriteError
            })?;

        // Return pointer to the allocated buffer with the value written to it
        Ok(Some(RuntimeValue::I32(value_ptr_ptr)))
    }

    /// Args:
    /// 1. "key" to delete from Tendermint (buffer of bytes)
    /// key is a pointer to a region "struct" of "pointer" and "length"
    /// A Region looks like { ptr: u32, len: u32 }
    fn remove_db_index(&mut self, _state_key_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap> {
        todo!()
    }

    /// Args:
    /// 1. "key" to write to Tendermint (buffer of bytes)
    /// 2. "value" to write to Tendermint (buffer of bytes)
    /// Both of them are pointers to a region "struct" of "pointer" and "length"
    /// Lets say Region looks like { ptr: u32, len: u32 }
    fn write_db_index(
        &mut self,
        state_key_ptr_ptr: i32,
        value_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        let state_key_name = self
            .extract_vector(state_key_ptr_ptr as u32)
            .map_err(|err| {
                error!(
                    "write_db() error while trying to read state_key_name from wasm memory: {:?}",
                    err
                );
                WasmEngineError::MemoryReadError
            })?;
        let value = self.extract_vector(value_ptr_ptr as u32).map_err(|err| {
            error!(
                "write_db() error while trying to read value from wasm memory: {:?}",
                err
            );
            WasmEngineError::MemoryReadError
        })?;

        trace!(
            "write_db() was called from WASM code with state_key_name: {:?} value: {:?}... (first 20 bytes)",
            String::from_utf8_lossy(&state_key_name),
            String::from_utf8_lossy(value.get(0..std::cmp::min(20, value.len())).unwrap())
        );

        write_encrypted_key(&state_key_name, &value, &self.context, &self.contract_key).map_err(
            |err| {
                error!(
                    "write_db() error while trying to write the value to state: {:?}",
                    err
                );
                WasmEngineError::from(err)
            },
        )?;

        Ok(None)
    }

    /// Args:
    /// 1. "human" to convert to canonical address (string)
    /// 2. "canonical" a buffer to write the result into (buffer of bytes)
    /// Both of them are pointers to a region "struct" of "pointer" and "length"
    /// A Region looks like { ptr: u32, len: u32 }
    fn canonicalize_address_index(
        &mut self,
        human_ptr_ptr: i32,
        canonical_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        let human = self.extract_vector(human_ptr_ptr as u32).map_err(|err| {
            error!(
                "canonicalize_address() error while trying to read human address from wasm memory: {:?}",
                err
            );
            WasmEngineError::MemoryReadError
        })?;

        trace!(
            "canonicalize_address() was called from WASM code with {:?}",
            String::from_utf8_lossy(&human)
        );

        // Turn Vec<u8> to str
        let mut human_addr_str = std::str::from_utf8(&human).map_err(|err|{
            error!(
                "canonicalize_address() error while trying to parse human address from bytes to string: {:?}",
                err
            );
            WasmEngineError::InputInvalid
        })?;

        human_addr_str = human_addr_str.trim();
        if human_addr_str.is_empty() {
            return Err(WasmEngineError::InputEmpty.into());
        }
        let (decoded_prefix, data) =   bech32::decode(&human_addr_str).map_err(|err|{
            error!(
                "canonicalize_address() error while trying to decode human address {:?} as bech32: {:?}",
                human_addr_str, err
            );
            WasmEngineError::InputInvalid
        })?;

        if decoded_prefix != BECH32_PREFIX_ACC_ADDR {
            warn!(
                "canonicalize_address() wrong prefix {:?} (expected {:?}) while decoding human address {:?} as bech32",
                decoded_prefix,
                BECH32_PREFIX_ACC_ADDR,
                human_addr_str
            );
            return Err(WasmEngineError::InputWrongPrefix.into());
        }

        let canonical = Vec::<u8>::from_base32(&data).map_err(|err| {
            warn!(
                "canonicalize_address() error while trying to decode bytes from base32 {:?}: {:?}",
                data, err
            );
            WasmEngineError::InputInvalid
        })?;

        if canonical.len() != 20 {
            // cosmos address length is 20
            // https://github.com/cosmos/cosmos-sdk/blob/v0.38.4/types/address.go#L32
            warn!(
                "canonicalize_address() decoded canonical address is not 20 bytes: {:?}",
                canonical
            );
            return Err(WasmEngineError::InputWrongLength.into());
        }

        // Get pointer to the buffer (this was allocated in WASM)
        let canonical_ptr_in_wasm = self
            .get_memory()
            .get_value::<u32>(canonical_ptr_ptr as u32)
            .map_err(|err| {
                error!(
                    "read_db() error while trying to get pointer for the result buffer: {:?}",
                    err,
                );
                WasmEngineError::MemoryReadError
            })?;

        // Get length of the buffer (this was allocated in WASM)
        let canonical_len_in_wasm = self
            .memory
            .get_value::<u32>((canonical_ptr_ptr + 8) as u32)
            .map_err(|err| {
                error!(
                    "read_db() error while trying to get pointer for the result buffer: {:?}",
                    err,
                );
                WasmEngineError::MemoryReadError
            })?;

        // Check that canonical is not too big to write into the allocated buffer (canonical should always be 20 bytes)
        if canonical_len_in_wasm < canonical.len() as u32 {
            error!(
                "canonicalize_address() result to big ({} bytes) to write to allocated wasm buffer ({} bytes)",
                canonical.len(),
                canonical_len_in_wasm
            );
            return Err(WasmEngineError::MemoryAllocationError.into());
        }

        // Write the canonical address to WASM memory
        self.get_memory()
            .set(canonical_ptr_in_wasm, &canonical)
            .map_err(|err| {
                error!(
                    "canonicalize_address() error while trying to write to result buffer: {:?}",
                    err
                );
                WasmEngineError::MemoryWriteError
            })?;

        // return 0 == ok
        Ok(Some(RuntimeValue::I32(0)))
    }

    /// Args:
    /// 1. "canonical" to convert to human address (buffer of bytes)
    /// 2. "human" a buffer to write the result (humanized string) into (buffer of bytes)
    /// Both of them are pointers to a region "struct" of "pointer" and "length"
    /// A Region looks like { ptr: u32, len: u32 }
    fn humanize_address_index(
        &mut self,
        canonical_ptr_ptr: i32,
        human_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        let canonical = self.extract_vector(canonical_ptr_ptr as u32).map_err(|err| {
            error!(
                "humanize_address() error while trying to read canonical address from wasm memory: {:?}",
                err
            );
            WasmEngineError::MemoryReadError
        })?;

        trace!(
            "humanize_address() was called from WASM code with {:?}",
            canonical
        );

        if canonical.len() != 20 {
            // cosmos address length is 20
            // https://github.com/cosmos/cosmos-sdk/blob/v0.38.4/types/address.go#L32
            error!(
                "humanize_address() input canonical address must be 20 bytes: {:?}",
                canonical
            );
            return Err(WasmEngineError::InputWrongLength.into());
        }

        let human_addr_str= bech32::encode(BECH32_PREFIX_ACC_ADDR, canonical.to_base32())
            .map_err(|err| {
            error!("humanize_address() error while trying to encode canonical address {:?} to human: {:?}",  canonical, err);
                WasmEngineError::InputInvalid
            })?;

        let human_bytes = human_addr_str.into_bytes();

        // Get pointer to the region of the human buffer
        let human_ptr_in_wasm: u32 = self
            .get_memory()
            .get_value::<u32>(human_ptr_ptr as u32)
            .map_err(|err| {
                error!(
                    "humanize_address() error while trying to get pointer for the result buffer: {:?}",
                    err,
                );
                WasmEngineError::MemoryReadError
            })?;

        // Get length of the buffer (this was allocated in WASM)
        let human_len_in_wasm: u32 = self
            .get_memory()
            .get_value::<u32>((human_ptr_ptr + 8) as u32)
            .map_err(|err| {
                error!(
                    "humanize_address() error while trying to get length of result buffer: {:?}",
                    err
                );
                WasmEngineError::MemoryReadError
            })?;

        // Check that human_bytes is not too big to write into the allocated buffer (human_bytes should always be 45 bytes)
        if human_len_in_wasm < human_bytes.len() as u32 {
            error!(
                "humanize_address() result to big ({} bytes) to write to allocated wasm buffer ({} bytes)",
                human_bytes.len(),
                human_len_in_wasm
            );
            return Err(WasmEngineError::OutputWrongLength.into());
        }

        // Write the canonical address to WASM memory
        self.get_memory()
            .set(human_ptr_in_wasm, &human_bytes)
            .map_err(|err| {
                error!(
                    "humanize_address() error while trying to write to result buffer: {:?}",
                    err
                );
                WasmEngineError::MemoryWriteError
            })?;

        // return 0 == ok
        Ok(Some(RuntimeValue::I32(0)))
    }

    fn gas_index(&mut self, gas_amount: i32) -> Result<Option<RuntimeValue>, Trap> {
        self.gas_used += gas_amount as u64;

        // Check if new amount is bigger than gas limit
        // If is above the limit, halt execution
        if self.gas_used > self.gas_limit {
            warn!(
                "Out of gas! Gas limit: {}, gas used: {}",
                self.gas_limit, self.gas_used
            );
            return Err(WasmEngineError::OutOfGas.into());
        }

        Ok(None)
    }
}
