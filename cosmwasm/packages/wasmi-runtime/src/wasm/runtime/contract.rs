use bech32::{FromBase32, ToBase32};
use log::*;
use wasmi::{Error as InterpreterError, MemoryInstance, MemoryRef, RuntimeValue, Trap};

use enclave_ffi_types::Ctx;

use crate::consts::BECH32_PREFIX_ACC_ADDR;
use crate::wasm::contract_validation::ContractKey;
use crate::wasm::db::{read_encrypted_key, write_encrypted_key};
use crate::wasm::errors::{DbError, WasmEngineError};
use crate::wasm::runtime::traits::WasmiApi;

/// An unknown error occurred when writing to region
const ERROR_READING_DB: i32 = -5;
/// An unknown error occurred when writing to region
const ERROR_WRITING_DB: i32 = -6;

/// An unknown error occurred when writing to region
const ERROR_WRITE_TO_REGION_UNKNONW: i32 = -1_000_001;
/// Could not write to region because it is too small
const ERROR_WRITE_TO_REGION_TOO_SMALL: i32 = -1_000_002;

/// SecretContract maps function index to implementation
/// When instantiating a module we give it the SecretNetworkImportResolver resolver
/// When invoking a function inside the module we give it this runtime which is the actual functions implementation ()
pub struct ContractInstance {
    pub context: Ctx,
    pub memory: MemoryRef,
    pub gas_limit: u64,
    pub gas_used: u64,
    pub contract_key: ContractKey,
}

impl ContractInstance {
    fn get_memory(&self) -> &MemoryInstance {
        &*self.memory
    }

    pub fn new(context: Ctx, memory: MemoryRef, gas_limit: u64, contract_key: ContractKey) -> Self {
        Self {
            context,
            memory,
            gas_limit,
            gas_used: 0,
            contract_key,
        }
    }
    /// extract_vector extracts key into a buffer
    fn extract_vector(&self, vec_ptr_ptr: u32) -> Result<Vec<u8>, InterpreterError> {
        let ptr: u32 = self.get_memory().get_value(vec_ptr_ptr)?;
        let len: u32 = self.get_memory().get_value(vec_ptr_ptr + 4)?;

        self.get_memory().get(ptr, len as usize)
    }
}

impl WasmiApi for ContractInstance {
    /// Args:
    /// 1. "key" to write to Tendermint (buffer of bytes)
    /// 2. "value" to write to Tendermint (buffer of bytes)
    /// Both of them are pointers to a region "struct" of "pointer" and "length"
    /// Lets say Region looks like { ptr: u32, len: u32 }
    fn read_db_index(
        &mut self,
        state_key_ptr_ptr: i32,
        value_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        let state_key_name = match self.extract_vector(state_key_ptr_ptr as u32) {
            Err(err) => {
                warn!(
                    "read_db() error while trying to read state_key_name from wasm memory: {:?}",
                    err
                );
                return Ok(Some(RuntimeValue::I32(-1)));
            }
            Ok(value) => value,
        };

        trace!(
            "read_db() was called from WASM code with state_key_name: {:?}",
            String::from_utf8_lossy(&state_key_name)
        );

        // Call read_db (this bubbles up to Tendermint via ocalls and FFI to Go code)
        // This returns the value from Tendermint
        let value: Vec<u8> =
            match read_encrypted_key(&state_key_name, &self.context, &self.contract_key) {
                Err(e) => {
                    return match e {
                        DbError::EmptyValue => Ok(Some(RuntimeValue::I32(0))),
                        _ => Ok(Some(RuntimeValue::I32(ERROR_READING_DB))),
                    }
                }
                Ok(v) => v,
            };

        // Get pointer to the buffer (this was allocated in WASM)
        let value_ptr_in_wasm: u32 = match self.memory.get_value::<u32>(value_ptr_ptr as u32) {
            Ok(x) => x,
            Err(err) => {
                warn!(
                    "read_db() error while trying to get pointer for the result buffer: {:?}",
                    err
                );
                return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
            }
        };
        // Get length of the buffer (this was allocated in WASM)
        let value_len_in_wasm: u32 = match self.memory.get_value::<u32>((value_ptr_ptr + 4) as u32)
        {
            Ok(x) => x,
            Err(err) => {
                warn!(
                    "read_db() error while trying to get length of result buffer: {:?}",
                    err
                );
                return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
            }
        };

        // Check that value is not too big to write into the allocated buffer
        if value_len_in_wasm < value.len() as u32 {
            warn!(
                "read_db() result to big ({} bytes) to write to allocated wasm buffer ({} bytes)",
                value.len(),
                value_len_in_wasm
            );
            return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_TOO_SMALL)));
        }

        // Write value returned from read_db to WASM memory
        if let Err(err) = self.get_memory().set(value_ptr_in_wasm, &value) {
            warn!(
                "read_db() error while trying to write to result buffer: {:?}",
                err
            );
            return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
        }

        // Return how many bytes were written to the buffer
        Ok(Some(RuntimeValue::I32(value.len() as i32)))
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
        let state_key_name = match self.extract_vector(state_key_ptr_ptr as u32) {
            Err(err) => {
                warn!(
                    "write_db() error while trying to read key from wasm memory: {:?}",
                    err
                );
                return Ok(Some(RuntimeValue::I32(-1)));
            }
            Ok(value) => value,
        };

        // extract_vector extracts value into a buffer
        let value = match self.extract_vector(value_ptr_ptr as u32) {
            Err(err) => {
                warn!(
                    "write_db() error while trying to read value from wasm memory: {:?}",
                    err
                );
                return Ok(Some(RuntimeValue::I32(-2)));
            }
            Ok(value) => value,
        };

        trace!(
            "write_db() was called from WASM code with state_key_name: {:?} value: {:?}... (first 20 bytes)",
            String::from_utf8_lossy(&state_key_name),
            String::from_utf8_lossy(value.get(0..std::cmp::min(20, value.len())).unwrap())
        );

        if let Err(_e) =
            write_encrypted_key(&state_key_name, &value, &self.context, &self.contract_key)
        {
            return Ok(Some(RuntimeValue::I32(ERROR_WRITING_DB)));
        }
        Ok(None)
    }

    fn canonicalize_address_index(
        &mut self,
        canonical_ptr_ptr: i32,
        human_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        let human = match self.extract_vector(human_ptr_ptr as u32) {
            Err(err) => {
                warn!(
                    "canonicalize_address() error while trying to read human address from wasm memory: {:?}",
                    err
                );
                return Ok(Some(RuntimeValue::I32(-1)));
            }
            Ok(value) => value,
        };

        trace!(
            "canonicalize_address() was called from WASM code with {:?}",
            String::from_utf8_lossy(&human)
        );

        // Turn Vec<u8> to str
        let mut human_addr_str = match std::str::from_utf8(&human) {
            Err(err) => {
                warn!(
                    "canonicalize_address() error while trying to parse human address from bytes to string: {:?}",
                    err
                );
                return Ok(Some(RuntimeValue::I32(-2)));
            }
            Ok(x) => x,
        };

        human_addr_str = human_addr_str.trim();
        if human_addr_str.is_empty() {
            return Ok(Some(RuntimeValue::I32(0)));
        }
        let (decoded_prefix, data) = match bech32::decode(&human_addr_str) {
            Err(err) => {
                warn!(
                    "canonicalize_address() error while trying to decode human address {:?} as bech32: {:?}",
                    human_addr_str, err
                );
                return Ok(Some(RuntimeValue::I32(-3)));
            }
            Ok(x) => x,
        };

        if decoded_prefix != BECH32_PREFIX_ACC_ADDR {
            warn!(
                "canonicalize_address() wrong prefix {:?} (expected {:?}) while decoding human address {:?} as bech32",
                decoded_prefix,
                BECH32_PREFIX_ACC_ADDR,
                human_addr_str
            );
            return Ok(Some(RuntimeValue::I32(-4)));
        }

        let canonical = match Vec::<u8>::from_base32(&data) {
            Err(err) => {
                warn!(
                    "canonicalize_address() error while trying to decode bytes from base32 {:?}: {:?}",
                    data,
                    err
                );
                return Ok(Some(RuntimeValue::I32(-5)));
            }
            Ok(x) => x,
        };

        if canonical.len() != 20 {
            // cosmos address length is 20
            // https://github.com/cosmos/cosmos-sdk/blob/v0.38.1/types/address.go#L32
            warn!(
                "canonicalize_address() decoded canonical address is not 20 bytes: {:?}",
                canonical
            );
            return Ok(Some(RuntimeValue::I32(-6)));
        }

        // Get pointer to the buffer (this was allocated in WASM)
        let canonical_ptr_in_wasm: u32 = match self
            .memory
            .get_value::<u32>(canonical_ptr_ptr as u32)
        {
            Ok(x) => x,
            Err(err) => {
                warn!(
                    "canonicalize_address() error while trying to get pointer for the result buffer: {:?}", err
                );
                return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
            }
        };
        // Get length of the buffer (this was allocated in WASM)
        let canonical_len_in_wasm: u32 = match self
            .memory
            .get_value::<u32>((canonical_ptr_ptr + 4) as u32)
        {
            Ok(x) => x,
            Err(err) => {
                warn!(
                    "canonicalize_address() error while trying to get length of result buffer: {:?}", err
                );
                return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
            }
        };

        // Check that canonical is not too big to write into the allocated buffer (canonical should always be 20 bytes)
        if canonical_len_in_wasm < canonical.len() as u32 {
            warn!(
                "canonicalize_address() result to big ({} bytes) to write to allocated wasm buffer ({} bytes)",
                canonical.len(),
                canonical_len_in_wasm
            );
            return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_TOO_SMALL)));
        }

        // Write the canonical address to WASM memory
        if let Err(err) = self.get_memory().set(canonical_ptr_in_wasm, &canonical) {
            warn!(
                "canonicalize_address() error while trying to write to result buffer: {:?}",
                err
            );
            return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
        }
        // return AccAddress(bz), nil
        Ok(Some(RuntimeValue::I32(canonical.len() as i32)))
    }
    /// Args:
    /// 1. "key" to write to Tendermint (buffer of bytes)
    /// 2. "value" to write to Tendermint (buffer of bytes)
    /// Both of them are pointers to a region "struct" of "pointer" and "length"
    /// Lets say Region looks like { ptr: u32, len: u32 }
    fn humanize_address_index(
        &mut self,
        canonical_ptr_ptr: i32,
        human_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        let canonical = match self.extract_vector(canonical_ptr_ptr as u32) {
            Err(err) => {
                warn!(
                    "humanize_address() error while trying to read human address from wasm memory: {:?}",
                    err
                );
                return Ok(Some(RuntimeValue::I32(-1)));
            }
            Ok(value) => value,
        };

        trace!(
            "humanize_address() was called from WASM code with {:?}",
            canonical
        );

        if canonical.len() != 20 {
            // cosmos address length is 20
            // https://github.com/cosmos/cosmos-sdk/blob/v0.38.1/types/address.go#L32
            warn!(
                "humanize_address() input canonical address must be 20 bytes: {:?}",
                canonical
            );
            return Ok(Some(RuntimeValue::I32(-2)));
        }

        let human_addr_str = match bech32::encode(BECH32_PREFIX_ACC_ADDR, canonical.to_base32()) {
            Err(err) => {
                warn!(
                    "humanize_address() error while trying to encode canonical address {:?} to human: {:?}",
                    canonical,
                    err
                );
                return Ok(Some(RuntimeValue::I32(-3)));
            }
            Ok(value) => value,
        };

        let human_bytes = human_addr_str.into_bytes();

        // Get pointer to the region of the human buffer
        let human_ptr_in_wasm: u32 = match self.memory.get_value::<u32>(human_ptr_ptr as u32) {
            Ok(x) => x,
            Err(err) => {
                warn!("humanize_address() error while trying to get pointer for the result buffer: {:?}", err);
                return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
            }
        };
        // Get length of the buffer (this was allocated in WASM)
        let human_len_in_wasm: u32 = match self.memory.get_value::<u32>((human_ptr_ptr + 4) as u32)
        {
            Ok(x) => x,
            Err(err) => {
                warn!(
                    "humanize_address() error while trying to get length of result buffer: {:?}",
                    err
                );
                return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
            }
        };

        // Check that human_bytes is not too big to write into the allocated buffer (human_bytes should always be 45 bytes)
        if human_len_in_wasm < human_bytes.len() as u32 {
            warn!(
                "humanize_address() result to big ({} bytes) to write to allocated wasm buffer ({} bytes)",
                human_bytes.len(),
                human_len_in_wasm
            );
            return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_TOO_SMALL)));
        }

        // Write the canonical address to WASM memory
        if let Err(err) = self.get_memory().set(human_ptr_in_wasm, &human_bytes) {
            warn!(
                "humanize_address() error while trying to write to result buffer: {:?}",
                err
            );
            return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
        }

        Ok(Some(RuntimeValue::I32(human_bytes.len() as i32)))
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
