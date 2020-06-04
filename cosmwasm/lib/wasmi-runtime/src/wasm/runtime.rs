use bech32;
use bech32::{FromBase32, ToBase32};
use log::{error, trace, warn};
use sgx_tcrypto::*;
use sgx_types::{sgx_status_t, SgxError, SgxResult};
use std::str;
use wasmi::{
    Error as InterpreterError, Externals, FuncInstance, FuncRef, MemoryRef, ModuleImportResolver,
    ModuleRef, RuntimeArgs, RuntimeValue, Signature, Trap, ValueType,
};

use enclave_ffi_types::{Ctx, EnclaveBuffer};

use super::contract_validation::ContractKey;
use crate::crypto::key_manager;
use crate::crypto::traits::{Encryptable, Kdf};
use crate::crypto::*;

use super::errors::{DbError, WasmEngineError};
// Runtime maps function index to implementation
// When instansiating a module we give it the EnigmaImportResolver resolver
// When invoking a function inside the module we give it this runtime which is the acctual functions implementation ()
use crate::exports;
use crate::imports;
use crate::wasm::db::{read_encrypted_key, write_encrypted_key};

// --------------------------------
// Functions to expose to WASM code
// --------------------------------
// TODO find better name for `Runtime`

// EnigmaImportResolver maps function name to its function signature and also to function index in Runtime
// When instansiating a module we give it this resolver
// When invoking a function inside the module we can give it different runtimes (which we probably won't do)
#[derive(Debug, Clone)]
pub struct EnigmaImportResolver {}

// These functions should be available to invoke from wasm code
// These should pass the request up to go-cosmwasm:
// fn read_db(key: *const c_void, value: *mut c_void) -> i32;
// fn write_db(key: *const c_void, value: *mut c_void);
// These should be implemented here: + TODO: Check Cosmwasm implementation for these:
// fn canonicalize_address(human: *const c_void, canonical: *mut c_void) -> i32;
// fn humanize_address(canonical: *const c_void, human: *mut c_void) -> i32;
impl ModuleImportResolver for EnigmaImportResolver {
    fn resolve_func(
        &self,
        func_name: &str,
        _signature: &Signature,
    ) -> Result<FuncRef, InterpreterError> {
        let func_ref = match func_name {
            // fn read_db(key: *const c_void, value: *mut c_void) -> i32;
            "read_db" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32, ValueType::I32][..], Some(ValueType::I32)),
                READ_DB_INDEX,
            ),
            // fn write_db(key: *const c_void, value: *mut c_void);
            "write_db" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32, ValueType::I32][..], None),
                WRITE_DB_INDEX,
            ),
            // fn canonicalize_address(human: *const c_void, canonical: *mut c_void) -> i32;
            "canonicalize_address" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32, ValueType::I32][..], Some(ValueType::I32)),
                CANONICALIZE_ADDRESS_INDEX,
            ),
            // fn humanize_address(canonical: *const c_void, human: *mut c_void) -> i32;
            "humanize_address" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32, ValueType::I32][..], Some(ValueType::I32)),
                HUMANIZE_ADDRESS_INDEX,
            ),
            // fn gas(amount: i32);
            "gas" => {
                FuncInstance::alloc_host(Signature::new(&[ValueType::I32][..], None), GAS_INDEX)
            }
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

pub struct Runtime {
    pub context: Ctx,
    pub memory: MemoryRef,
    pub gas_limit: u64,
    pub gas_used: u64,
    pub contract_key: ContractKey,
}

impl Runtime {
    pub fn new(context: Ctx, memory: MemoryRef, gas_limit: u64, contract_key: ContractKey) -> Self {
        Self {
            context,
            memory,
            gas_limit,
            gas_used: 0,
            contract_key,
        }
    }
}

const READ_DB_INDEX: usize = 0;
const WRITE_DB_INDEX: usize = 1;
const CANONICALIZE_ADDRESS_INDEX: usize = 2;
const HUMANIZE_ADDRESS_INDEX: usize = 3;
const GAS_INDEX: usize = 4;

/// An unknown error occurred when writing to region
const ERROR_READING_DB: i32 = -5;
/// An unknown error occurred when writing to region
const ERROR_WRITING_DB: i32 = -6;

/// An unknown error occurred when writing to region
const ERROR_WRITE_TO_REGION_UNKNONW: i32 = -1000001;
/// Could not write to region because it is too small
const ERROR_WRITE_TO_REGION_TOO_SMALL: i32 = -1000002;

impl Externals for Runtime {
    fn invoke_index(
        &mut self,
        index: usize,
        args: RuntimeArgs,
    ) -> Result<Option<RuntimeValue>, Trap> {
        match index {
            READ_DB_INDEX => {
                // This function is imported to WASM code

                // We get 2 args:
                // 1. "key" to read from Tendermint (buffer of bytes)
                // 2. "value" - a buffer that was allocated in WASM code - we need to write the read_db result to this buffer
                // Both of them are pointers to a region "struct" of "pointer" and "length"
                // Lets say Region looks like { ptr: u32, len: u32 }

                // Get pointer to the region of the key name
                // extract_vectors extract key into a buffer
                let key_ptr_ptr_in_wasm: i32 = args.nth_checked(0).map_err(|err| {
                    error!(
                        "read_db() error reading arguments, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let state_key_name = match extract_vector(&self.memory, key_ptr_ptr_in_wasm as u32)
                {
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
                        Ok(T) => T,
                    };

                // Get pointer to the region of the value buffer
                let value_ptr_ptr_in_wasm: i32 = args.nth_checked(1)?;

                // Get pointer to the buffer (this was allocated in WASM)
                let value_ptr_in_wasm: u32 = match self
                    .memory
                    .get_value::<u32>(value_ptr_ptr_in_wasm as u32)
                {
                    Ok(x) => x,
                    Err(err) => {
                        warn!("read_db() error while trying to get pointer for the result buffer: {:?}", err);
                        return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
                    }
                };
                // Get length of the buffer (this was allocated in WASM)
                let value_len_in_wasm: u32 = match self
                    .memory
                    .get_value::<u32>((value_ptr_ptr_in_wasm + 4) as u32)
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
                        "read_db() result to big ({} bytes) to write to allocated wasm buffer ({} bytes)" ,value.len(),value_len_in_wasm
                    );
                    return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_TOO_SMALL)));
                }

                // Write value returned from read_db to WASM memory
                if let Err(err) = self.memory.set(value_ptr_in_wasm, &value) {
                    warn!(
                        "read_db() error while trying to write to result buffer: {:?}",
                        err
                    );
                    return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
                }

                // Return how many bytes were written to the buffer
                Ok(Some(RuntimeValue::I32(value.len() as i32)))
            }
            WRITE_DB_INDEX => {
                // This function is imported to WASM code

                // We get 2 args:
                // 1. "key" to write to Tendermint (buffer of bytes)
                // 2. "value" to write to Tendermint (buffer of bytes)
                // Both of them are pointers to a region "struct" of "pointer" and "length"
                // Lets say Region looks like { ptr: u32, len: u32 }

                // Get pointer to the region of the key name
                let key_ptr_ptr_in_wasm: i32 = args.nth_checked(0).map_err(|err| {
                    error!(
                        "write_db() error reading arguments, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                // extract_vector extracts key into a buffer
                let state_key_name = match extract_vector(&self.memory, key_ptr_ptr_in_wasm as u32)
                {
                    Err(err) => {
                        warn!(
                            "write_db() error while trying to read key from wasm memory: {:?}",
                            err
                        );
                        return Ok(Some(RuntimeValue::I32(-1)));
                    }
                    Ok(value) => value,
                };

                // Get pointer to the region of the value
                let value_ptr_ptr_in_wasm: i32 = args.nth_checked(1)?;
                // extract_vector extracts value into a buffer
                let value = match extract_vector(&self.memory, value_ptr_ptr_in_wasm as u32) {
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

                if let Err(e) =
                    write_encrypted_key(&state_key_name, &value, &self.context, &self.contract_key)
                {
                    return Ok(Some(RuntimeValue::I32(ERROR_WRITING_DB)));
                }
                Ok(None)
            }
            // fn canonicalize_address(human: *const c_void, canonical: *mut c_void) -> i32;
            CANONICALIZE_ADDRESS_INDEX => {
                let human_ptr_ptr_in_wasm: i32 = args.nth_checked(0).map_err(|err| {
                    error!(
                        "canonicalize_address() error reading arguments, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                // extract_vector extracts human addr into a buffer
                let human = match extract_vector(&self.memory, human_ptr_ptr_in_wasm as u32) {
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
                let mut human_addr_str = match str::from_utf8(&human) {
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
                if human_addr_str.len() == 0 {
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

                // Get pointer to the region of the canonical buffer
                let canonical_ptr_ptr_in_wasm: i32 = args.nth_checked(1)?;

                // Get pointer to the buffer (this was allocated in WASM)
                let canonical_ptr_in_wasm: u32 = match self
                    .memory
                    .get_value::<u32>(canonical_ptr_ptr_in_wasm as u32)
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
                    .get_value::<u32>((canonical_ptr_ptr_in_wasm + 4) as u32)
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
                if let Err(err) = self.memory.set(canonical_ptr_in_wasm, &canonical) {
                    warn!(
                        "canonicalize_address() error while trying to write to result buffer: {:?}",
                        err
                    );
                    return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
                }
                // return AccAddress(bz), nil
                Ok(Some(RuntimeValue::I32(canonical.len() as i32)))
            }
            // fn humanize_address(canonical: *const c_void, human: *mut c_void) -> i32;
            HUMANIZE_ADDRESS_INDEX => {
                // func humanAddress(canon []byte) (string, error) {
                //     if len(canon) != sdk.AddrLen {
                //         return "", fmt.Errorf("Expected %d byte address", sdk.AddrLen)
                //     }
                //     return sdk.AccAddress(canon).String(), nil
                // }
                let canonical_ptr_ptr_in_wasm: i32 = args.nth_checked(0).map_err(|err| {
                    error!(
                        "humanize_address() error reading arguments, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                // extract_vector extracts canonical address into a buffer
                let canonical = match extract_vector(&self.memory, canonical_ptr_ptr_in_wasm as u32)
                {
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

                let human_addr_str = match bech32::encode(
                    BECH32_PREFIX_ACC_ADDR,
                    canonical.to_base32(),
                ) {
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
                let human_ptr_ptr_in_wasm: i32 = args.nth_checked(1)?;

                // Get pointer to the buffer (this was allocated in WASM)
                let human_ptr_in_wasm: u32 = match self
                    .memory
                    .get_value::<u32>(human_ptr_ptr_in_wasm as u32)
                {
                    Ok(x) => x,
                    Err(err) => {
                        warn!("humanize_address() error while trying to get pointer for the result buffer: {:?}", err);
                        return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
                    }
                };
                // Get length of the buffer (this was allocated in WASM)
                let human_len_in_wasm: u32 = match self
                    .memory
                    .get_value::<u32>((human_ptr_ptr_in_wasm + 4) as u32)
                {
                    Ok(x) => x,
                    Err(err) => {
                        warn!("humanize_address() error while trying to get length of result buffer: {:?}", err);
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
                if let Err(err) = self.memory.set(human_ptr_in_wasm, &human_bytes) {
                    warn!(
                        "humanize_address() error while trying to write to result buffer: {:?}",
                        err
                    );
                    return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
                }

                Ok(Some(RuntimeValue::I32(human_bytes.len() as i32)))
            }
            GAS_INDEX => {
                // Get the gas_amount argument
                let gas_amount: i32 = args.nth_checked(0).map_err(|err| {
                    error!("gas() error reading arguments, stopping wasm: {:?}", err);
                    err
                })?;

                // Add amount to a static counter
                self.gas_used += gas_amount as u64;

                // Check if new amount is bigger than gas limit
                // If is above the limit, halt execution
                if self.gas_used > self.gas_limit {
                    warn!(
                        "Out of gas! Gas limit: {}, gas used: {}",
                        self.gas_limit, self.gas_used
                    );
                    Err(WasmEngineError::OutOfGas)?;
                }

                Ok(None)
            }
            _ => panic!("unknown function index"),
        }
    }
}

const BECH32_PREFIX_ACC_ADDR: &'static str = "enigma";

pub struct Engine {
    runtime: Runtime,
    instance: ModuleRef,
}

impl Engine {
    pub fn new(runtime: Runtime, instance: ModuleRef) -> Self {
        Self { runtime, instance }
    }

    pub fn gas_used(&self) -> u64 {
        self.runtime.gas_used
    }

    pub fn allocate(&mut self, len: u32) -> Result<u32, InterpreterError> {
        match self.instance.invoke_export(
            "allocate",
            &[RuntimeValue::I32(len as i32)],
            &mut self.runtime,
        )? {
            Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
            other => Err(InterpreterError::Value(format!(
                "allocate method returned value which wasn't u32: {:?}",
                other
            ))),
        }
    }

    pub fn memory(&self) -> MemoryRef {
        self.instance
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
        extract_vector(&self.memory(), vec_ptr_ptr)
    }

    pub fn init(&mut self, env_ptr: u32, msg_ptr: u32) -> Result<u32, InterpreterError> {
        trace!("Invoking init() in wasm");

        match self.instance.invoke_export(
            "init",
            &[
                RuntimeValue::I32(env_ptr as i32),
                RuntimeValue::I32(msg_ptr as i32),
            ],
            &mut self.runtime,
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

        match self.instance.invoke_export(
            "handle",
            &[
                RuntimeValue::I32(env_ptr as i32),
                RuntimeValue::I32(msg_ptr as i32),
            ],
            &mut self.runtime,
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

        match self.instance.invoke_export(
            "query",
            &[RuntimeValue::I32(msg_ptr as i32)],
            &mut self.runtime,
        )? {
            Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
            other => Err(InterpreterError::Value(format!(
                "query method returned value which wasn't u32: {:?}",
                other
            ))),
        }
    }
}

fn extract_vector(memory: &MemoryRef, vec_ptr_ptr: u32) -> Result<Vec<u8>, InterpreterError> {
    let ptr: u32 = memory.get_value(vec_ptr_ptr)?;
    let len: u32 = memory.get_value(vec_ptr_ptr + 4)?;

    memory.get(ptr, len as usize)
}
