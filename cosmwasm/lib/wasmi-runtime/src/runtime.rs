use bech32;
use bech32::{FromBase32, ToBase32};
use sgx_types::{sgx_status_t, SgxError, SgxResult};
use std::str;

use wasmi::{
    Error as InterpreterError, Externals, FuncInstance, FuncRef, MemoryRef, ModuleImportResolver,
    ModuleRef, RuntimeArgs, RuntimeValue, Signature, Trap, ValueType,
};

use enclave_ffi_types::{Ctx, EnclaveBuffer};

use super::errors::WasmEngineError;

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
use super::exports;
use super::imports;

/// Safe wrapper around reads from the contract storage
fn read_db(context: Ctx, key: &[u8]) -> SgxResult<Option<Vec<u8>>> {
    let mut enclave_buffer = std::mem::MaybeUninit::<EnclaveBuffer>::uninit();
    unsafe {
        match imports::ocall_read_db(
            enclave_buffer.as_mut_ptr(),
            context,
            key.as_ptr(),
            key.len(),
        ) {
            sgx_status_t::SGX_SUCCESS => { /* continue */ }
            error_status => return Err(error_status),
        }
        let enclave_buffer = enclave_buffer.assume_init();
        // TODO add validation of this pointer before returning its contents.
        Ok(exports::recover_buffer(enclave_buffer))
    }
}

/// Safe wrapper around writes to the contract storage
fn write_db(context: Ctx, key: &[u8], value: &[u8]) -> SgxError {
    match unsafe {
        imports::ocall_write_db(
            context,
            key.as_ptr(),
            key.len(),
            value.as_ptr(),
            value.len(),
        )
    } {
        sgx_status_t::SGX_SUCCESS => Ok(()),
        err => Err(err),
    }
}

pub struct Runtime {
    pub context: Ctx,
    pub memory: MemoryRef,
}

const READ_DB_INDEX: usize = 0;
const WRITE_DB_INDEX: usize = 1;
const CANONICALIZE_ADDRESS_INDEX: usize = 2;
const HUMANIZE_ADDRESS_INDEX: usize = 3;

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
                let key_ptr_ptr_in_wasm: i32 = args.nth_checked(0)?;
                let key = match extract_vector(&self.memory, key_ptr_ptr_in_wasm as u32) {
                    Err(_) => return Ok(Some(RuntimeValue::I32(0))),
                    Ok(value) => value,
                };

                // Call read_db (this bubbles up to Tendermint via ocalls and FFI to Go code)
                // This returns the value from Tendermint
                // fn read_db(context: Ctx, key: &[u8]) -> Option<Vec<u8>> {
                let value = match read_db(unsafe { self.context.clone() }, &key)
                    .map_err(|_| WasmEngineError::FailedOcall)?
                {
                    None => return Ok(Some(RuntimeValue::I32(0))),
                    Some(value) => value,
                };

                // Get pointer to the region of the value buffer
                let value_ptr_ptr_in_wasm: i32 = args.nth_checked(1)?;

                // Get pointer to the buffer (this was allocated in WASM)
                let value_ptr_in_wasm: u32 = match self
                    .memory
                    .get_value::<u32>(value_ptr_ptr_in_wasm as u32)
                {
                    Ok(x) => x,
                    Err(_) => return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW))),
                };
                // Get length of the buffer (this was allocated in WASM)
                let value_len_in_wasm: u32 = match self
                    .memory
                    .get_value::<u32>((value_ptr_ptr_in_wasm + 4) as u32)
                {
                    Ok(x) => x,
                    Err(_) => return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW))),
                };

                // Check that value is not too big to write into the allocated buffer
                if value_len_in_wasm < value.len() as u32 {
                    return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_TOO_SMALL)));
                }

                // Write value returned from read_db to WASM memory
                if let Err(_) = self.memory.set(value_ptr_in_wasm, &value) {
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
                let key_ptr_ptr_in_wasm: i32 = args.nth_checked(0)?;
                // extract_vector extracts key into a buffer
                let key = match extract_vector(&self.memory, key_ptr_ptr_in_wasm as u32) {
                    Err(_) => return Ok(Some(RuntimeValue::I32(0))),
                    Ok(value) => value,
                };

                // Get pointer to the region of the value
                let value_ptr_ptr_in_wasm: i32 = args.nth_checked(1)?;
                // extract_vector extracts value into a buffer
                let value = match extract_vector(&self.memory, value_ptr_ptr_in_wasm as u32) {
                    Err(_) => return Ok(Some(RuntimeValue::I32(0))),
                    Ok(value) => value,
                };

                // Call write_db (this bubbles up to Tendermint via ocalls and FFI to Go code)
                // fn write_db(context: Ctx, key: &[u8], value: &[u8]) {
                write_db(unsafe { self.context.clone() }, &key, &value)
                    .map_err(|_| WasmEngineError::FailedOcall)?;

                // Return nothing because this is the api ¯\_(ツ)_/¯
                Ok(None)
            }
            // fn canonicalize_address(human: *const c_void, canonical: *mut c_void) -> i32;
            CANONICALIZE_ADDRESS_INDEX => {
                let human_ptr_ptr_in_wasm: i32 = args.nth_checked(0)?;

                // extract_vector extracts human addr into a buffer
                let human = match extract_vector(&self.memory, human_ptr_ptr_in_wasm as u32) {
                    Err(_) => return Ok(Some(RuntimeValue::I32(-1))),
                    Ok(value) => value,
                };

                // Turn Vec<u8> to str
                let mut human_addr_str = match str::from_utf8(&human) {
                    Err(_) => return Ok(Some(RuntimeValue::I32(-2))),
                    Ok(x) => x,
                };

                // if len(strings.TrimSpace(address)) == 0 {
                //     return AccAddress{}, nil
                // }
                human_addr_str = human_addr_str.trim();
                if human_addr_str.len() == 0 {
                    return Ok(Some(RuntimeValue::I32(0)));
                }
                // bz, err := GetFromBech32(address, bech32PrefixAccAddr)
                // if err != nil {
                //     return nil, err
                // }
                let (decoded_prefix, data) = match bech32::decode(&human_addr_str) {
                    Err(_) => return Ok(Some(RuntimeValue::I32(-3))),
                    Ok(x) => x,
                };
                if decoded_prefix != BECH32_PREFIX_ACC_ADDR {
                    return Ok(Some(RuntimeValue::I32(-4)));
                }

                // err = VerifyAddressFormat(bz)
                // func VerifyAddressFormat(bz []byte) error {
                // 	verifier := GetConfig().GetAddressVerifier()
                // 	if verifier != nil { // this is always null
                // 		return verifier(bz)
                // 	}
                // 	if len(bz) != AddrLen { // this is 20
                // 		return errors.New("incorrect address length")
                // 	}
                // 	return nil
                // }
                // if err != nil {
                //     return nil, err
                // }
                let canonical = match Vec::<u8>::from_base32(&data) {
                    Err(_) => return Ok(Some(RuntimeValue::I32(-5))),
                    Ok(x) => x,
                };
                if canonical.len() != 20 {
                    // cosmos address length is 20
                    // https://github.com/cosmos/cosmos-sdk/blob/v0.38.1/types/address.go#L32
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
                    Err(_) => return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW))),
                };
                // Get length of the buffer (this was allocated in WASM)
                let canonical_len_in_wasm: u32 = match self
                    .memory
                    .get_value::<u32>((canonical_ptr_ptr_in_wasm + 4) as u32)
                {
                    Ok(x) => x,
                    Err(_) => return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW))),
                };

                // Check that canonical is not too big to write into the allocated buffer (canonical should always be 20 bytes)
                if canonical_len_in_wasm < canonical.len() as u32 {
                    return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_TOO_SMALL)));
                }

                // Write the canonical address to WASM memory
                if let Err(_) = self.memory.set(canonical_ptr_in_wasm, &canonical) {
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
                let canonical_ptr_ptr_in_wasm: i32 = args.nth_checked(0)?;

                // extract_vector extracts canonical address into a buffer
                let canonical = match extract_vector(&self.memory, canonical_ptr_ptr_in_wasm as u32)
                {
                    Err(_) => return Ok(Some(RuntimeValue::I32(-1))),
                    Ok(value) => value,
                };

                if canonical.len() != 20 {
                    // cosmos address length is 20
                    // https://github.com/cosmos/cosmos-sdk/blob/v0.38.1/types/address.go#L32
                    return Ok(Some(RuntimeValue::I32(-2)));
                }

                let human_addr_str =
                    match bech32::encode(BECH32_PREFIX_ACC_ADDR, canonical.to_base32()) {
                        Err(_) => return Ok(Some(RuntimeValue::I32(-3))),
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
                    Err(_) => return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW))),
                };
                // Get length of the buffer (this was allocated in WASM)
                let human_len_in_wasm: u32 = match self
                    .memory
                    .get_value::<u32>((human_ptr_ptr_in_wasm + 4) as u32)
                {
                    Ok(x) => x,
                    Err(_) => return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW))),
                };

                // Check that human_bytes is not too big to write into the allocated buffer (human_bytes should always be 45 bytes)
                if human_len_in_wasm < human_bytes.len() as u32 {
                    return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_TOO_SMALL)));
                }

                // Write the canonical address to WASM memory
                if let Err(_) = self.memory.set(human_ptr_in_wasm, &human_bytes) {
                    return Ok(Some(RuntimeValue::I32(ERROR_WRITE_TO_REGION_UNKNONW)));
                }

                Ok(Some(RuntimeValue::I32(human_bytes.len() as i32)))
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
        let pointer = self.allocate(buffer.len() as u32)?;
        self.memory().set(pointer, buffer)?;
        Ok(pointer)
    }

    pub fn extract_vector(&self, vec_ptr_ptr: u32) -> Result<Vec<u8>, InterpreterError> {
        extract_vector(&self.memory(), vec_ptr_ptr)
    }

    pub fn init(&mut self, env_ptr: u32, msg_ptr: u32) -> Result<u32, InterpreterError> {
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
