use bech32::{FromBase32, ToBase32};
use log::*;
use rand_chacha::ChaChaRng;
use rand_core::SeedableRng;

use std::convert::TryFrom;

use wasmi::{Error as InterpreterError, MemoryInstance, MemoryRef, ModuleRef, RuntimeValue, Trap};

use enclave_ffi_types::{Ctx, EnclaveError};

use cw_types_generic::CosmWasmApiVersion;
use cw_types_v010::consts::BECH32_PREFIX_ACC_ADDR;

use enclave_crypto::{sha_256, Ed25519PublicKey, WasmApiCryptoError};

use crate::contract_validation::ContractKey;
use crate::db::read_encrypted_key;
// #[cfg(not(feature = "query-only"))]
use crate::db::{remove_encrypted_key, write_encrypted_key};
use crate::errors::WasmEngineError;
use crate::gas::{WasmCosts, OCALL_BASE_GAS};
use crate::query_chain::encrypt_and_query_chain;
use crate::types::IoNonce;
use crate::wasm::traits::WasmiApi;

/// api_marker is based on this compatibility chart:
/// https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/vm/README.md#compatibility
pub mod api_marker {
    pub const V0_10: &str = "cosmwasm_vm_version_3";
    pub const V1: &str = "interface_version_8";
}

/// Right now ContractOperation is used to detect queris and prevent state changes
#[derive(Clone, Copy, Debug)]
pub enum ContractOperation {
    Init,
    Handle,
    Query,
}

#[allow(unused)]
impl ContractOperation {
    pub fn is_init(&self) -> bool {
        matches!(self, ContractOperation::Init)
    }

    pub fn is_handle(&self) -> bool {
        matches!(self, ContractOperation::Handle)
    }

    pub fn is_query(&self) -> bool {
        matches!(self, ContractOperation::Query)
    }
}

const MAX_LOG_LENGTH: usize = 8192;

/// SecretContract maps function index to implementation
/// When instantiating a module we give it the SecretNetworkImportResolver resolver
/// When invoking a function inside the module we give it this runtime which is the actual functions implementation ()
pub struct ContractInstance {
    pub context: Ctx,
    pub memory: MemoryRef,
    pub gas_limit: u64,
    /// Gas used by the WASM code and WASM host
    pub gas_used: u64,
    /// Gas used by external services. This is tracked separately so we don't double-charge for external services later.
    pub gas_used_externally: u64,
    pub gas_costs: WasmCosts,
    pub contract_key: ContractKey,
    pub module: ModuleRef,
    // #[cfg_attr(feature = "query-only", allow(unused))]
    operation: ContractOperation,
    query_depth: u32,
    pub user_nonce: IoNonce,
    pub user_public_key: Ed25519PublicKey,
    pub cosmwasm_api_version: CosmWasmApiVersion,
}

impl ContractInstance {
    #[allow(clippy::too_many_arguments)]
    #[allow(dead_code)]
    pub fn new(
        context: Ctx,
        module: ModuleRef,
        gas_limit: u64,
        gas_costs: WasmCosts,
        contract_key: ContractKey,
        operation: ContractOperation,
        query_depth: u32,
        user_nonce: IoNonce,
        user_public_key: Ed25519PublicKey,
    ) -> Result<Self, EnclaveError> {
        let memory = module
            .export_by_name("memory")
            .expect("Module expected to have 'memory' export")
            .as_memory()
            .cloned()
            .expect("'memory' export should be of memory type");

        let cosmwasm_api_version;
        if module.export_by_name(api_marker::V0_10).is_some() {
            cosmwasm_api_version = CosmWasmApiVersion::V010
        } else if module.export_by_name(api_marker::V1).is_some() {
            cosmwasm_api_version = CosmWasmApiVersion::V1
        } else {
            return Err(EnclaveError::InvalidWasm);
        };

        Ok(Self {
            context,
            memory,
            gas_limit,
            gas_used: 0,
            gas_used_externally: 0,
            gas_costs,
            contract_key,
            module,
            operation,
            query_depth,
            user_nonce,
            user_public_key,
            cosmwasm_api_version,
        })
    }

    fn get_memory(&self) -> &MemoryInstance {
        &*self.memory
    }

    /// decode_sections extracts a vector of vectors from the wasm memory space
    ///
    /// Each encoded section is suffixed by a section length, encoded as big endian uint32.
    ///
    /// See also: `encode_section`.
    pub fn decode_sections(&self, vec_ptr_ptr: u32) -> Result<Vec<Vec<u8>>, WasmEngineError> {
        self.decode_sections_inner(vec_ptr_ptr).map_err(|err| {
            debug!(
                "error while trying to read the buffer at {:?} : {:?}",
                vec_ptr_ptr, err
            );
            WasmEngineError::MemoryReadError
        })
    }

    fn decode_sections_inner(&self, vec_ptr_ptr: u32) -> Result<Vec<Vec<u8>>, InterpreterError> {
        let data_ptr: u32 = self.get_memory().get_value(vec_ptr_ptr)?;

        if data_ptr == 0 {
            return Err(InterpreterError::Memory(String::from(
                "Main vector: trying to read from null pointer in WASM memory",
            )));
        }

        let data_len: u32 = self.get_memory().get_value(vec_ptr_ptr + 8)?;
        let data = self.get_memory().get(data_ptr, data_len as usize)?;

        let mut remaining_len = data_len as usize;

        let mut result: Vec<Vec<u8>> = vec![];
        while remaining_len >= 4 {
            let tail_len = u32::from_be_bytes([
                data[remaining_len - 4],
                data[remaining_len - 3],
                data[remaining_len - 2],
                data[remaining_len - 1],
            ]) as usize;
            let mut new_element = vec![0; tail_len];
            new_element.copy_from_slice(&data[remaining_len - 4 - tail_len..remaining_len - 4]);
            result.push(new_element);
            remaining_len -= 4 + tail_len;
        }
        result.reverse();

        Ok(result)
    }

    /// extract_vector extracts a vector from the wasm memory space
    pub fn extract_vector(&self, ptr: u32) -> Result<Vec<u8>, WasmEngineError> {
        self.extract_vector_inner(ptr).map_err(|err| {
            debug!(
                "error while trying to read the buffer at {:?} : {:?}",
                ptr, err
            );
            WasmEngineError::MemoryReadError
        })
    }

    /// extract_vector_inner extracts a vector from a WASM pointer
    /// vec_ptr is a pointer to a region "struct" of "pointer" and "length"
    /// A Region looks like { ptr: u32, len: u32 }
    fn extract_vector_inner(&self, ptr: u32) -> Result<Vec<u8>, InterpreterError> {
        let ptr_offset: u32 = self.get_memory().get_value(ptr)?;

        if ptr_offset == 0 {
            return Err(InterpreterError::Memory(String::from(
                "Trying to read from null pointer in WASM memory",
            )));
        }

        let ptr_size: u32 = self.get_memory().get_value(ptr + 8)?;

        self.get_memory().get(ptr_offset, ptr_size as usize)
    }

    pub fn allocate(&mut self, len: u32) -> Result<u32, WasmEngineError> {
        self.allocate_inner(len).map_err(|err| {
            debug!("Failed to allocate {} bytes in wasm: {}", len, err);
            WasmEngineError::MemoryAllocationError
        })
    }

    fn allocate_inner(&mut self, len: u32) -> Result<u32, InterpreterError> {
        match self.module.clone().invoke_export(
            "allocate",
            &[RuntimeValue::I32(len as i32)],
            self,
        )? {
            Some(RuntimeValue::I32(0)) => Err(InterpreterError::Memory(String::from(
                "Allocate returned null pointer from WASM",
            ))),
            Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
            other => Err(InterpreterError::Value(format!(
                "allocate method returned value which wasn't u32: {:?}",
                other
            ))),
        }
    }

    pub fn write_to_allocated_memory(
        &mut self,
        buffer: &[u8],
        ptr_to_region_in_wasm_vm: u32,
    ) -> Result<u32, WasmEngineError> {
        self.write_to_allocated_memory_inner(buffer, ptr_to_region_in_wasm_vm)
            .map_err(|err| {
                debug!(
                    "error while trying to write the buffer {:?} to the destination buffer at {:?} : {:?}",
                    buffer, ptr_to_region_in_wasm_vm, err
                );
                WasmEngineError::MemoryWriteError
            })
    }

    fn write_to_allocated_memory_inner(
        &mut self,
        buffer: &[u8],
        ptr_to_region_in_wasm_vm: u32,
    ) -> Result<u32, InterpreterError> {
        // WASM pointers are pointers to "Region"
        // Region is a struct that looks like this:
        // ptr_to_region -> | 4byte = buffer_addr | 4bytes = buffer_cap | 4bytes = buffer_len |

        // extract the buffer pointer from the region
        let buffer_addr_in_wasm: u32 = self
            .get_memory()
            .get_value::<u32>(ptr_to_region_in_wasm_vm)?;

        if buffer_addr_in_wasm == 0 {
            return Err(InterpreterError::Memory(String::from(
                "Trying to write to null pointer in WASM memory",
            )));
        }

        let buffer_cap_in_wasm: u32 = self
            .get_memory()
            .get_value::<u32>(ptr_to_region_in_wasm_vm + 4)?;

        if buffer_cap_in_wasm < buffer.len() as u32 {
            return Err(InterpreterError::Memory(format!(
                "Tried to write {} bytes but only got {} bytes in destination buffer",
                buffer.len(),
                buffer_cap_in_wasm
            )));
        }

        self.get_memory().set(buffer_addr_in_wasm, buffer)?;

        self.get_memory()
            .set_value::<u32>(ptr_to_region_in_wasm_vm + 8, buffer.len() as u32)?;

        // return the WASM pointer
        Ok(ptr_to_region_in_wasm_vm)
    }

    pub fn write_to_memory(&mut self, buffer: &[u8]) -> Result<u32, WasmEngineError> {
        // allocate return a pointer to a region
        let ptr_to_region_in_wasm_vm = self.allocate(buffer.len() as u32)?;
        self.write_to_allocated_memory(buffer, ptr_to_region_in_wasm_vm)
    }

    /// Gas used by the WASM code and WASM host
    fn use_gas(&mut self, gas_amount: u64) -> Result<(), WasmEngineError> {
        self.gas_used = self.gas_used.saturating_add(gas_amount);
        self.check_gas_usage()
    }

    /// Gas used by external services. This is tracked separately so we don't double-charge for external services later.
    fn use_gas_externally(&mut self, gas_amount: u64) -> Result<(), WasmEngineError> {
        self.gas_used_externally = self.gas_used_externally.saturating_add(gas_amount);
        self.check_gas_usage()
    }

    fn check_gas_usage(&self) -> Result<(), WasmEngineError> {
        // Check if new amount is bigger than gas limit
        // If is above the limit, halt execution
        if self.is_gas_depleted() {
            debug!(
                "Out of gas! Gas limit: {}, gas used: {}, gas used externally: {}",
                self.gas_limit, self.gas_used, self.gas_used_externally
            );
            Err(WasmEngineError::OutOfGas)
        } else {
            Ok(())
        }
    }

    fn is_gas_depleted(&self) -> bool {
        self.gas_limit < self.gas_used.saturating_add(self.gas_used_externally)
    }

    fn gas_left(&self) -> u64 {
        self.gas_limit
            .saturating_sub(self.gas_used)
            .saturating_sub(self.gas_used_externally)
    }
}

impl WasmiApi for ContractInstance {
    /// Read the value of a key in the contract's storage
    /// v0.10 + v1
    ///
    /// Args:
    /// 1. "key" to read from Tendermint (buffer of bytes)
    /// key is a pointer to a region "struct" of "pointer" and "length"
    /// A Region looks like { ptr: u32, len: u32 }
    fn read_db(&mut self, state_key_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap> {
        let state_key_name = self
            .extract_vector(state_key_ptr_ptr as u32)
            .map_err(|err| {
                debug!("read_db() error while trying to read state_key_name from wasm memory");
                err
            })?;

        trace!(
            "read_db() was called from WASM code with state_key_name: {:?}",
            String::from_utf8_lossy(&state_key_name)
        );

        self.use_gas(OCALL_BASE_GAS)?;

        // Call read_db (this bubbles up to Tendermint via ocalls and FFI to Go code)
        // This returns the value from Tendermint
        let (value, gas_used_by_storage) =
            read_encrypted_key(&state_key_name, &self.context, &self.contract_key)?;
        self.use_gas_externally(gas_used_by_storage)?;

        let value = match value {
            // return 0 if key doesn't exist
            // https://github.com/scrtlabs/SecretNetwork/blob/2aacc3333ba3a10ed54c03c56576d72c7c9dcc59/cosmwasm/packages/std/src/imports.rs?plain=1#L75
            None => return Ok(Some(RuntimeValue::I32(0))),
            Some(value) => value,
        };

        trace!(
            "read_db() got value with len {}: '{:?}'",
            value.len(),
            value
        );

        let ptr_to_region_in_wasm_vm = self.write_to_memory(&value).map_err(|err| {
            debug!(
                "read_db() error while trying to allocate {} bytes for the value",
                value.len(),
            );
            err
        })?;

        // Return pointer to the allocated buffer with the value written to it
        // https://github.com/scrtlabs/SecretNetwork/blob/2aacc3333ba3a10ed54c03c56576d72c7c9dcc59/cosmwasm/packages/std/src/imports.rs?plain=1#L80
        Ok(Some(RuntimeValue::I32(ptr_to_region_in_wasm_vm as i32)))
    }

    // /// Remove a key from the contract's storage
    // /// v0.10 + v1
    // ///
    // /// Args:
    // /// 1. "key" to delete from Tendermint (buffer of bytes)
    // /// key is a pointer to a region "struct" of "pointer" and "length"
    // /// A Region looks like { ptr: u32, len: u32 }
    // #[cfg(feature = "query-only")]
    // fn remove_db(&mut self, _state_key_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap> {
    //     Err(WasmEngineError::UnauthorizedWrite.into())
    // }

    /// Remove a key from the contract's storage
    /// v0.10 + v1
    ///
    /// Args:
    /// 1. "key" to delete from Tendermint (buffer of bytes)
    /// key is a pointer to a region "struct" of "pointer" and "length"
    /// A Region looks like { ptr: u32, len: u32 }
    // #[cfg(not(feature = "query-only"))]
    fn remove_db(&mut self, state_key_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap> {
        if self.operation.is_query() {
            return Err(WasmEngineError::UnauthorizedWrite.into());
        }

        let state_key_name = self
            .extract_vector(state_key_ptr_ptr as u32)
            .map_err(|err| {
                debug!("remove_db() error while trying to read state_key_name from wasm memory");
                err
            })?;

        trace!(
            "remove_db() was called from WASM code with state_key_name: {:?}",
            String::from_utf8_lossy(&state_key_name)
        );

        // Call remove_db (this bubbles up to Tendermint via ocalls and FFI to Go code)
        let gas_used_by_storge =
            remove_encrypted_key(&state_key_name, &self.context, &self.contract_key)?;
        self.use_gas_externally(gas_used_by_storge)?;

        // return value from here is never read
        // https://github.com/scrtlabs/SecretNetwork/blob/2aacc3333ba3a10ed54c03c56576d72c7c9dcc59/cosmwasm/packages/std/src/imports.rs?plain=1#L102
        Ok(None)
    }

    // /// Write (key,value) into the contract's storage
    // /// v0.10 + v1
    // ///
    // /// Args:
    // /// 1. "key" to write to Tendermint (buffer of bytes)
    // /// 2. "value" to write to Tendermint (buffer of bytes)
    // /// Both of them are pointers to a region "struct" of "pointer" and "length"
    // /// Lets say Region looks like { ptr: u32, len: u32 }
    // #[cfg(feature = "query-only")]
    // fn write_db(
    //     &mut self,
    //     _state_key_ptr_ptr: i32,
    //     _value_ptr_ptr: i32,
    // ) -> Result<Option<RuntimeValue>, Trap> {
    //     Err(WasmEngineError::UnauthorizedWrite.into())
    // }

    /// Write (key,value) into the contract's storage
    /// v0.10 + v1
    ///
    /// Args:
    /// 1. "key" to write to Tendermint (buffer of bytes)
    /// 2. "value" to write to Tendermint (buffer of bytes)
    /// Both of them are pointers to a region "struct" of "pointer" and "length"
    /// Lets say Region looks like { ptr: u32, len: u32 }
    // #[cfg(not(feature = "query-only"))]
    fn write_db(
        &mut self,
        state_key_ptr: i32,
        value_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        if self.operation.is_query() {
            return Err(WasmEngineError::UnauthorizedWrite.into());
        }

        let state_key_name = self.extract_vector(state_key_ptr as u32).map_err(|err| {
            debug!("write_db() error while trying to read state_key_name from wasm memory");
            err
        })?;
        let value = self.extract_vector(value_ptr as u32).map_err(|err| {
            debug!("write_db() error while trying to read value from wasm memory");
            err
        })?;

        trace!(
            "write_db() was called from WASM code with state_key_name: {:?} value: {:?}",
            String::from_utf8_lossy(&state_key_name),
            String::from_utf8_lossy(&value),
        );

        self.use_gas(OCALL_BASE_GAS)?;

        let used_gas_by_storage =
            write_encrypted_key(&state_key_name, &value, &self.context, &self.contract_key)
                .map_err(|err| {
                    debug!(
                        "write_db() error while trying to write the value to state: {:?}",
                        err
                    );
                    err
                })?;
        self.use_gas_externally(used_gas_by_storage)?;

        // return value from here is never read
        // https://github.com/scrtlabs/SecretNetwork/blob/2aacc3333ba3a10ed54c03c56576d72c7c9dcc59/cosmwasm/packages/std/src/imports.rs?plain=1#L95
        Ok(None)
    }

    /// Convert a human readable address into its bytes representation
    /// v0.10
    ///
    /// Args:
    /// 1. "human" to convert to canonical address (string)
    /// 2. "canonical" a buffer to write the result into (buffer of bytes)
    /// Both of them are pointers to a region "struct" of "pointer" and "length"
    /// A Region looks like { ptr: u32, len: u32 }
    fn canonicalize_address(
        &mut self,
        human_ptr: i32,
        canonical_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        self.use_gas(self.gas_costs.external_canonicalize_address as u64)?;

        let human = self.extract_vector(human_ptr as u32).map_err(|err| {
            debug!(
                "canonicalize_address() error while trying to read human address from wasm memory"
            );
            err
        })?;

        trace!(
            "canonicalize_address() was called from WASM code with {:?}",
            String::from_utf8_lossy(&human)
        );

        // Turn Vec<u8> to str
        let mut human_addr_str = match std::str::from_utf8(&human) {
            Err(err) => {
                debug!(
                    "canonicalize_address() error while trying to parse human address from bytes to string: {:?}",
                    err
                );
                return Ok(Some(RuntimeValue::I32(
                    self.write_to_memory(b"input is not valid UTF-8")? as i32,
                )));
            }
            Ok(x) => x,
        };

        // Assaf: This trim() + is_empty() check is redundant, but it's part
        // of an undocumented API between the chain and contracts that was
        // introduces in secret-2. Therfore to remove this check will break
        // this (stupid) API.
        human_addr_str = human_addr_str.trim();
        if human_addr_str.is_empty() {
            return Ok(Some(RuntimeValue::I32(
                self.write_to_memory(b"input is empty")? as i32,
            )));
        }

        let (decoded_prefix, data) = match bech32::decode(&human_addr_str) {
            Err(err) => {
                debug!(
                    "canonicalize_address() error while trying to decode human address {:?} as bech32: {:?}",
                    human_addr_str, err
                );
                return Ok(Some(RuntimeValue::I32(
                    self.write_to_memory(err.to_string().as_bytes())? as i32,
                )));
            }
            Ok(x) => x,
        };

        if decoded_prefix != BECH32_PREFIX_ACC_ADDR {
            debug!(
                "canonicalize_address() wrong prefix {:?} (expected {:?}) while decoding human address {:?} as bech32",
                decoded_prefix,
                BECH32_PREFIX_ACC_ADDR,
                human_addr_str
            );
            return Ok(Some(RuntimeValue::I32(
                self.write_to_memory(
                    format!("wrong address prefix: {:?}", decoded_prefix).as_bytes(),
                )? as i32,
            )));
        }

        let canonical = Vec::<u8>::from_base32(&data).map_err(|err| {
            // Assaf: From reading https://docs.rs/bech32/0.7.2/src/bech32/lib.rs.html#607
            // and https://docs.rs/bech32/0.7.2/src/bech32/lib.rs.html#228 I don't think this can fail that way
            debug!(
                "canonicalize_address() error while trying to decode bytes from base32 {:?}: {:?}",
                data, err
            );
            WasmEngineError::Base32Error
        })?;

        // write the result to the output buffer
        // https://github.com/scrtlabs/SecretNetwork/blob/2aacc3333ba3a10ed54c03c56576d72c7c9dcc59/cosmwasm/packages/std/src/imports.rs?plain=1#L189
        self.write_to_allocated_memory(&canonical, canonical_ptr as u32)
            .map_err(|err| {
                debug!(
                    "canonicalize_address() error while trying to write the answer {:?} to the destination buffer",
                    canonical,
                );
                err
            })?;

        // return 0 == ok
        // https://github.com/scrtlabs/SecretNetwork/blob/2aacc3333ba3a10ed54c03c56576d72c7c9dcc59/cosmwasm/packages/std/src/imports.rs?plain=1#L181
        Ok(Some(RuntimeValue::I32(0)))
    }

    /// Convert an address represented as bytes into its human readable form
    /// v0.10
    ///
    /// Args:
    /// 1. "canonical" to convert to human address (buffer of bytes)
    /// 2. "human" a buffer to write the result (humanized string) into (buffer of bytes)
    /// Both of them are pointers to a region "struct" of "pointer" and "length"
    /// A Region looks like { ptr: u32, len: u32 }
    fn humanize_address(
        &mut self,
        canonical_ptr: i32,
        human_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        self.use_gas(self.gas_costs.external_humanize_address as u64)?;

        let canonical = self.extract_vector(canonical_ptr as u32).map_err(|err| {
            debug!(
                "humanize_address() error while trying to read canonical address from wasm memory",
            );
            err
        })?;

        trace!(
            "humanize_address() was called from WASM code with {:?}",
            canonical
        );

        let human_addr_str = match bech32::encode(BECH32_PREFIX_ACC_ADDR, canonical.to_base32()) {
            Err(err) => {
                // Assaf: IMO This can never fail. From looking at bech32::encode, it only fails
                // because input prefix issues. For us the prefix is always "secert" which is valid.
                debug!("humanize_address() error while trying to encode canonical address {:?} to human: {:?}",  canonical, err);
                return Ok(Some(RuntimeValue::I32(
                    self.write_to_memory(err.to_string().as_bytes())? as i32,
                )));
            }
            Ok(x) => x,
        };

        let human_bytes = human_addr_str.into_bytes();

        // write the result to the output buffer
        // https://github.com/scrtlabs/SecretNetwork/blob/2aacc3333ba3a10ed54c03c56576d72c7c9dcc59/cosmwasm/packages/std/src/imports.rs?plain=1#L207
        self.write_to_allocated_memory(&human_bytes, human_ptr as u32)
            .map_err(|err| {
                debug!(
                    "humanize_address() error while trying to write the answer {:?} to the destination buffer",
                    human_bytes,
                );
                err
            })?;

        // return 0 == ok
        // https://github.com/scrtlabs/SecretNetwork/blob/2aacc3333ba3a10ed54c03c56576d72c7c9dcc59/cosmwasm/packages/std/src/imports.rs?plain=1#L199
        Ok(Some(RuntimeValue::I32(0)))
    }

    /// Query another contract
    /// v0.10 + v1
    fn query_chain(&mut self, query_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap> {
        let query_buffer = self.extract_vector(query_ptr_ptr as u32).map_err(|err| {
            debug!("query_chain() error while trying to read canonical address from wasm memory",);
            err
        })?;

        trace!(
            "query_chain() was called from WASM code with {:?}",
            String::from_utf8_lossy(&query_buffer)
        );

        self.use_gas(OCALL_BASE_GAS)?;

        // Call query_chain (this bubbles up to x/compute via ocalls and FFI to Go code)
        // Returns the value from x/compute
        let mut gas_used_by_query: u64 = 0;
        let answer = encrypt_and_query_chain(
            &query_buffer,
            self.query_depth,
            &self.context,
            self.user_nonce,
            self.user_public_key,
            &mut gas_used_by_query,
            self.gas_left(),
        )?;

        info!(
            "query_chain() got an answer from outside with gas {} and result {:?}",
            gas_used_by_query,
            String::from_utf8_lossy(&answer)
        );

        self.use_gas_externally(gas_used_by_query)?;

        // write the result to an output buffer
        // https://github.com/scrtlabs/SecretNetwork/blob/2aacc3333ba3a10ed54c03c56576d72c7c9dcc59/cosmwasm/packages/std/src/imports.rs?plain=1#L353
        let ptr_to_region_in_wasm_vm = self.write_to_memory(&answer).map_err(|err| {
            debug!(
                "query_chain() error while trying to allocate and write the answer {:?} to the WASM VM",
                answer,
            );
            err
        })?;

        // Return pointer to the allocated buffer with the result written to it
        Ok(Some(RuntimeValue::I32(ptr_to_region_in_wasm_vm as i32)))
    }

    fn gas(&mut self, gas_amount: i32) -> Result<Option<RuntimeValue>, Trap> {
        self.use_gas(gas_amount as u64)?;
        Ok(None)
    }

    /// Validates a human readable address
    /// v1
    fn addr_validate(&mut self, human_ptr: i32) -> Result<Option<RuntimeValue>, Trap> {
        self.use_gas(self.gas_costs.external_addr_validate as u64)?;

        let human = self.extract_vector(human_ptr as u32).map_err(|err| {
            debug!("addr_validate() error while trying to read human address from wasm memory");
            err
        })?;

        trace!(
            "addr_validate() was called from WASM code with {:?}",
            String::from_utf8_lossy(&human)
        );

        if human.is_empty() {
            return Ok(Some(RuntimeValue::I32(
                self.write_to_memory(b"Input is empty")? as i32,
            )));
        }

        // Turn Vec<u8> to str
        let source_human_address = match std::str::from_utf8(&human) {
            Err(err) => {
                debug!(
                    "addr_validate() error while trying to parse human address from bytes to string: {:?}",
                    err
                );
                return Ok(Some(RuntimeValue::I32(
                    self.write_to_memory(b"Input is not valid UTF-8")? as i32,
                )));
            }
            Ok(x) => x,
        };

        let canonical_address = match bech32::decode(&source_human_address) {
            Err(err) => {
                debug!(
                    "addr_validate() error while trying to decode human address {:?} as bech32: {:?}",
                    source_human_address, err
                );
                return Ok(Some(RuntimeValue::I32(
                    self.write_to_memory(err.to_string().as_bytes())? as i32,
                )));
            }
            Ok((_prefix, canonical_address)) => canonical_address,
        };

        let normalized_human_address = match bech32::encode(
            BECH32_PREFIX_ACC_ADDR, // like we do in human_address()
            canonical_address.clone(),
        ) {
            Err(err) => {
                // Assaf: IMO This can never fail. From looking at bech32::encode, it only fails
                // because input prefix issues. For us the prefix is always "secert" which is valid.
                debug!("addr_validate() error while trying to encode canonical address {:?} to human: {:?}",  &canonical_address, err);
                return Ok(Some(RuntimeValue::I32(
                    self.write_to_memory(err.to_string().as_bytes())? as i32,
                )));
            }
            Ok(normalized_human_address) => normalized_human_address,
        };

        if source_human_address != normalized_human_address {
            return Ok(Some(RuntimeValue::I32(
                self.write_to_memory(b"Address is not normalized")? as i32,
            )));
        }

        // return 0 == ok
        // https://github.com/scrtlabs/SecretNetwork/blob/2aacc3333ba3a10ed54c03c56576d72c7c9dcc59/cosmwasm/packages/std/src/imports.rs?plain=1#L164
        Ok(Some(RuntimeValue::I32(0)))
    }

    /// addr_canonicalize is just like canonicalize_address but fixes some error messages that are different between v0.10 and v1
    /// v1
    fn addr_canonicalize(
        &mut self,
        human_ptr: i32,
        canonical_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        self.use_gas(self.gas_costs.external_canonicalize_address as u64)?;

        let human = self.extract_vector(human_ptr as u32).map_err(|err| {
            debug!("addr_canonicalize() error while trying to read human address from wasm memory");
            err
        })?;

        trace!(
            "addr_canonicalize() was called from WASM code with {:?}",
            String::from_utf8_lossy(&human)
        );

        if human.is_empty() {
            return Ok(Some(RuntimeValue::I32(
                self.write_to_memory(b"Input is empty")? as i32,
            )));
        }

        // Turn Vec<u8> to str
        let human_addr_str = match std::str::from_utf8(&human) {
            Err(err) => {
                debug!(
                    "addr_canonicalize() error while trying to parse human address from bytes to string: {:?}",
                    err
                );
                return Ok(Some(RuntimeValue::I32(
                    self.write_to_memory(b"Input is not valid UTF-8")? as i32,
                )));
            }
            Ok(x) => x,
        };

        let (decoded_prefix, data) = match bech32::decode(&human_addr_str) {
            Err(err) => {
                debug!(
                    "addr_canonicalize() error while trying to decode human address {:?} as bech32: {:?}",
                    human_addr_str, err
                );
                return Ok(Some(RuntimeValue::I32(
                    self.write_to_memory(err.to_string().as_bytes())? as i32,
                )));
            }
            Ok(x) => x,
        };

        if decoded_prefix != BECH32_PREFIX_ACC_ADDR {
            debug!(
                "addr_canonicalize() wrong prefix {:?} (expected {:?}) while decoding human address {:?} as bech32",
                decoded_prefix,
                BECH32_PREFIX_ACC_ADDR,
                human_addr_str
            );
            return Ok(Some(RuntimeValue::I32(
                self.write_to_memory(
                    format!("wrong address prefix: {:?}", decoded_prefix).as_bytes(),
                )? as i32,
            )));
        }

        let canonical = Vec::<u8>::from_base32(&data).map_err(|err| {
            // Assaf: From reading https://docs.rs/bech32/0.7.2/src/bech32/lib.rs.html#607
            // and https://docs.rs/bech32/0.7.2/src/bech32/lib.rs.html#228 I don't think this can fail that way
            debug!(
                "addr_canonicalize() error while trying to decode bytes from base32 {:?}: {:?}",
                data, err
            );
            WasmEngineError::Base32Error
        })?;

        // write the result to the output buffer
        // https://github.com/scrtlabs/SecretNetwork/blob/2aacc3333ba3a10ed54c03c56576d72c7c9dcc59/cosmwasm/packages/std/src/imports.rs?plain=1#L189
        self.write_to_allocated_memory(&canonical, canonical_ptr as u32)
            .map_err(|err| {
                debug!(
                    "addr_canonicalize() error while trying to write the answer {:?} to the destination buffer",
                    canonical,
                );
                err
            })?;

        // return 0 == ok
        // https://github.com/scrtlabs/SecretNetwork/blob/2aacc3333ba3a10ed54c03c56576d72c7c9dcc59/cosmwasm/packages/std/src/imports.rs?plain=1#L181
        Ok(Some(RuntimeValue::I32(0)))
    }

    /// This is identical to humanize_address from v0.10
    /// v1
    fn addr_humanize(
        &mut self,
        canonical_ptr: i32,
        human_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        self.humanize_address(canonical_ptr, human_ptr)
    }

    fn debug_print_index(&self, message_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap> {
        let mut message_buffer = self.extract_vector(message_ptr_ptr as u32).map_err(|err| {
            debug!("debug_print() error while trying to read message from wasm memory",);
            err
        })?;

        message_buffer.truncate(MAX_LOG_LENGTH);

        info!(
            "debug_print: {:?}",
            String::from_utf8_lossy(&message_buffer)
        );

        Ok(None)
    }

    // This was added in v1 (v0.14?) but we're also backporting it to v0.10
    // to support easy migration from a crate to this API for existing v0.10
    // contracts.
    fn secp256k1_verify(
        &mut self,
        message_hash_ptr: i32,
        signature_ptr: i32,
        public_key_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        self.use_gas(self.gas_costs.external_secp256k1_verify as u64)?;

        let message_hash_data = self
            .extract_vector(message_hash_ptr as u32)
            .map_err(|err| {
                debug!(
                    "secp256k1_verify() error while trying to read message_hash from wasm memory"
                );
                err
            })?;
        let signature_data = self.extract_vector(signature_ptr as u32).map_err(|err| {
            debug!("secp256k1_verify() error while trying to read signature from wasm memory");
            err
        })?;
        let public_key = self.extract_vector(public_key_ptr as u32).map_err(|err| {
            debug!("secp256k1_verify() error while trying to read public_key from wasm memory");
            err
        })?;

        trace!(
            "secp256k1_verify() was called from WASM code with message_hash {:x?} (len {:?} should be 32)",
            &message_hash_data,
            message_hash_data.len()
        );
        trace!(
            "secp256k1_verify() was called from WASM code with signature {:x?} (len {:?} should be 64)",
            &signature_data,
            signature_data.len()
        );
        trace!(
            "secp256k1_verify() was called from WASM code with public_key {:x?} (len {:?} should be 33 or 65)",
            &public_key,
            public_key.len()
        );

        // check message_hash input
        if message_hash_data.len() != 32 {
            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L93
            return Ok(Some(RuntimeValue::I32(
                WasmApiCryptoError::InvalidHashFormat as i32,
            )));
        }

        // check signature input
        if signature_data.len() != 64 {
            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L94
            return Ok(Some(RuntimeValue::I32(
                WasmApiCryptoError::InvalidSignatureFormat as i32,
            )));
        }

        // check pubkey input
        if !match public_key.first() {
            // compressed
            Some(0x02) | Some(0x03) => public_key.len() == 33,
            // uncompressed
            Some(0x04) => public_key.len() == 65,
            // hybrid
            // see https://docs.rs/secp256k1-abc-sys/0.1.2/secp256k1_abc_sys/fn.secp256k1_ec_pubkey_parse.html
            Some(0x06) | Some(0x07) => public_key.len() == 65,
            _ => false,
        } {
            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L95
            return Ok(Some(RuntimeValue::I32(
                WasmApiCryptoError::InvalidPubkeyFormat as i32,
            )));
        }

        let secp256k1_msg = match secp256k1::Message::from_slice(&message_hash_data) {
            Err(err) => {
                debug!("secp256k1_verify() failed to create a secp256k1 message from message_hash: {:?}", err);

                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L98
                return Ok(Some(RuntimeValue::I32(
                    WasmApiCryptoError::GenericErr as i32,
                )));
            }
            Ok(x) => x,
        };

        let secp256k1_sig = match secp256k1::ecdsa::Signature::from_compact(&signature_data) {
            Err(err) => {
                debug!("secp256k1_verify() malformed signature: {:?}", err);

                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L98
                return Ok(Some(RuntimeValue::I32(
                    WasmApiCryptoError::GenericErr as i32,
                )));
            }
            Ok(x) => x,
        };

        let secp256k1_pk = match secp256k1::PublicKey::from_slice(public_key.as_slice()) {
            Err(err) => {
                debug!("secp256k1_verify() malformed pubkey: {:?}", err);

                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L98
                return Ok(Some(RuntimeValue::I32(
                    WasmApiCryptoError::GenericErr as i32,
                )));
            }
            Ok(x) => x,
        };

        match secp256k1::Secp256k1::verification_only().verify_ecdsa(
            &secp256k1_msg,
            &secp256k1_sig,
            &secp256k1_pk,
        ) {
            Err(err) => {
                debug!("secp256k1_verify() failed to verify signature: {:?}", err);

                // return 1 == failed, invalid signature
                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/vm/src/imports.rs#L220
                Ok(Some(RuntimeValue::I32(1)))
            }
            Ok(()) => {
                // return 0 == success, valid signature
                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/vm/src/imports.rs#L220
                Ok(Some(RuntimeValue::I32(0)))
            }
        }
    }

    fn secp256k1_recover_pubkey(
        &mut self,
        message_hash_ptr: i32,
        signature_ptr: i32,
        recovery_param: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        self.use_gas(self.gas_costs.external_secp256k1_recover_pubkey as u64)?;

        let message_hash_data = self
            .extract_vector(message_hash_ptr as u32)
            .map_err(|err| {
                debug!(
                    "secp256k1_recover_pubkey() error while trying to read message_hash from wasm memory"
                );
                err
            })?;
        let signature_data = self.extract_vector(signature_ptr as u32).map_err(|err| {
            debug!(
                "secp256k1_recover_pubkey() error while trying to read signature from wasm memory"
            );
            err
        })?;

        trace!(
                "secp256k1_recover_pubkey() was called from WASM code with message_hash {:x?} (len {:?} should be 32)",
                &message_hash_data,
                message_hash_data.len()
            );
        trace!(
                "secp256k1_recover_pubkey() was called from WASM code with signature {:x?} (len {:?} should be 64)",
                &signature_data,
                signature_data.len()
            );
        trace!(
            "secp256k1_recover_pubkey() was called from WASM code with recovery_param {:?}",
            recovery_param,
        );

        // check message_hash input
        if message_hash_data.len() != 32 {
            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L93
            return Ok(Some(RuntimeValue::I64(to_high_half(
                WasmApiCryptoError::InvalidHashFormat as u32,
            ) as i64)));
        }

        // check signature input
        if signature_data.len() != 64 {
            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L94
            return Ok(Some(RuntimeValue::I64(to_high_half(
                WasmApiCryptoError::InvalidSignatureFormat as u32,
            ) as i64)));
        }

        let secp256k1_msg = match secp256k1::Message::from_slice(&message_hash_data) {
            Err(err) => {
                debug!("secp256k1_recover_pubkey() failed to create a secp256k1 message from message_hash: {:?}", err);

                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L98
                return Ok(Some(RuntimeValue::I64(
                    to_high_half(WasmApiCryptoError::GenericErr as u32) as i64,
                )));
            }
            Ok(x) => x,
        };

        let recovery_id = match secp256k1::ecdsa::RecoveryId::from_i32(recovery_param) {
            Err(err) => {
                debug!("secp256k1_recover_pubkey() failed to create a secp256k1 recovery_id from recovery_param: {:?}", err);

                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L98
                return Ok(Some(RuntimeValue::I64(
                    to_high_half(WasmApiCryptoError::GenericErr as u32) as i64,
                )));
            }
            Ok(x) => x,
        };

        let secp256k1_sig = match secp256k1::ecdsa::RecoverableSignature::from_compact(
            &signature_data,
            recovery_id,
        ) {
            Err(err) => {
                debug!(
                    "secp256k1_recover_pubkey() malformed recoverable signature: {:?}",
                    err
                );

                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L98
                return Ok(Some(RuntimeValue::I64(
                    to_high_half(WasmApiCryptoError::GenericErr as u32) as i64,
                )));
            }
            Ok(x) => x,
        };

        match secp256k1::Secp256k1::verification_only()
            .recover_ecdsa(&secp256k1_msg, &secp256k1_sig)
        {
            Err(err) => {
                debug!(
                    "secp256k1_recover_pubkey() failed to recover pubkey: {:?}",
                    err
                );

                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L98
                Ok(Some(RuntimeValue::I64(
                    to_high_half(WasmApiCryptoError::GenericErr as u32) as i64,
                )))
            }
            Ok(pubkey) => {
                let answer = pubkey.serialize();
                let ptr_to_region_in_wasm_vm = self.write_to_memory(&answer).map_err(|err| {
                    debug!(
                        "secp256k1_recover_pubkey() error while trying to allocate and write the answer {:?} to the WASM VM",
                        &answer,
                    );
                    err
                })?;

                // Return pointer to the allocated buffer with the value written to it
                Ok(Some(RuntimeValue::I64(
                    to_low_half(ptr_to_region_in_wasm_vm) as i64,
                )))
            }
        }
    }

    fn ed25519_verify(
        &mut self,
        message_ptr: i32,
        signature_ptr: i32,
        public_key_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        self.use_gas(self.gas_costs.external_ed25519_verify as u64)?;

        let message_data = self.extract_vector(message_ptr as u32).map_err(|err| {
            debug!("ed25519_verify() error while trying to read message from wasm memory");
            err
        })?;

        let signature_data = self.extract_vector(signature_ptr as u32).map_err(|err| {
            debug!("ed25519_verify() error while trying to read signature from wasm memory");
            err
        })?;

        let public_key_data = self.extract_vector(public_key_ptr as u32).map_err(|err| {
            debug!("ed25519_verify() error while trying to read public_key from wasm memory");
            err
        })?;

        trace!(
            "ed25519_verify() was called from WASM code with message {:x?} (len {:?})",
            &message_data,
            message_data.len()
        );
        trace!(
            "ed25519_verify() was called from WASM code with signature {:x?} (len {:?} should be 64)",
            &signature_data,
            signature_data.len()
        );
        trace!(
            "ed25519_verify() was called from WASM code with public_key {:x?} (len {:?} should be 32)",
            &public_key_data,
            public_key_data.len()
        );

        let signature: ed25519_zebra::Signature =
            match ed25519_zebra::Signature::try_from(signature_data.as_slice()) {
                Ok(x) => x,
                Err(err) => {
                    debug!(
                    "ed25519_verify() failed to create an ed25519 signature from signature: {:?}",
                    err
                );

                    // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L94
                    return Ok(Some(RuntimeValue::I32(
                        WasmApiCryptoError::InvalidSignatureFormat as i32,
                    )));
                }
            };

        let public_key: ed25519_zebra::VerificationKey =
            match ed25519_zebra::VerificationKey::try_from(public_key_data.as_slice()) {
                Ok(x) => x,
                Err(err) => {
                    debug!(
                        "ed25519_verify() failed to create an ed25519 VerificationKey from public_key: {:?}",
                        err
                    );

                    // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L95
                    return Ok(Some(RuntimeValue::I32(
                        WasmApiCryptoError::InvalidPubkeyFormat as i32,
                    )));
                }
            };

        match public_key.verify(&signature, &message_data) {
            Err(err) => {
                debug!("ed25519_verify() failed to verify signature: {:?}", err);

                // return 1 == failed, invalid signature
                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/vm/src/imports.rs#L281
                Ok(Some(RuntimeValue::I32(1)))
            }
            Ok(()) => {
                // return 0 == success, valid signature
                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/vm/src/imports.rs#L281
                Ok(Some(RuntimeValue::I32(0)))
            }
        }
    }

    fn ed25519_batch_verify(
        &mut self,
        messages_ptr: i32,
        signatures_ptr: i32,
        public_keys_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        let messages_data = self.decode_sections(messages_ptr as u32).map_err(|err| {
            debug!("ed25519_batch_verify() error while trying to read messages from wasm memory");
            err
        })?;

        let signatures_data = self.decode_sections(signatures_ptr as u32).map_err(|err| {
            debug!("ed25519_batch_verify() error while trying to read signatures from wasm memory");
            err
        })?;

        let pubkeys_data = self
            .decode_sections(public_keys_ptr as u32)
            .map_err(|err| {
                debug!(
                    "ed25519_batch_verify() error while trying to read public_keys from wasm memory"
                );
                err
            })?;

        let (messages, signatures, pubkeys) = if messages_data.len() == signatures_data.len()
            && messages_data.len() == pubkeys_data.len()
        {
            // All is well, convert to Vec<&[u8]>
            (
                messages_data
                    .iter()
                    .map(|m| m.as_slice())
                    .collect::<Vec<&[u8]>>(),
                signatures_data
                    .iter()
                    .map(|s| s.as_slice())
                    .collect::<Vec<&[u8]>>(),
                pubkeys_data
                    .iter()
                    .map(|p| p.as_slice())
                    .collect::<Vec<&[u8]>>(),
            )
        } else if messages_data.len() == 1 && signatures_data.len() == pubkeys_data.len() {
            // Multisig, replicate message
            (
                vec![messages_data[0].as_slice()].repeat(signatures_data.len()),
                signatures_data
                    .iter()
                    .map(|s| s.as_slice())
                    .collect::<Vec<&[u8]>>(),
                pubkeys_data
                    .iter()
                    .map(|p| p.as_slice())
                    .collect::<Vec<&[u8]>>(),
            )
        } else if pubkeys_data.len() == 1 && messages_data.len() == signatures_data.len() {
            // Replicate pubkey
            (
                messages_data
                    .iter()
                    .map(|m| m.as_slice())
                    .collect::<Vec<&[u8]>>(),
                signatures_data
                    .iter()
                    .map(|s| s.as_slice())
                    .collect::<Vec<&[u8]>>(),
                vec![pubkeys_data[0].as_slice()].repeat(signatures_data.len()),
            )
        } else {
            debug!(
                "ed25519_batch_verify() mismatched number of messages ({}) / signatures ({}) / public keys ({})",
                messages_data.len(),
                signatures_data.len(),
                pubkeys_data.len(),
            );

            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L97
            return Ok(Some(RuntimeValue::I32(WasmApiCryptoError::BatchErr as i32)));
        };

        self.use_gas(
            self.gas_costs.external_ed25519_batch_verify_base as u64
                + (signatures.len() as u64)
                    * self.gas_costs.external_ed25519_batch_verify_each as u64,
        )?;

        let mut batch = ed25519_zebra::batch::Verifier::new();
        for i in 0..signatures.len() {
            let signature: ed25519_zebra::Signature = match ed25519_zebra::Signature::try_from(
                signatures[i],
            ) {
                Ok(x) => x,
                Err(err) => {
                    debug!(
                    "ed25519_batch_verify() failed to create an ed25519 signature from signatures[{}]: {:?}",
                    i, err
                );

                    // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L94
                    return Ok(Some(RuntimeValue::I32(
                        WasmApiCryptoError::InvalidSignatureFormat as i32,
                    )));
                }
            };

            let pubkey: ed25519_zebra::VerificationKeyBytes =
                match ed25519_zebra::VerificationKeyBytes::try_from(pubkeys[i]) {
                    Ok(x) => x,
                    Err(err) => {
                        debug!(
                        "ed25519_batch_verify() failed to create an ed25519 VerificationKey from public_keys[{}]: {:?}",
                        i, err
                    );

                        // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L95
                        return Ok(Some(RuntimeValue::I32(
                            WasmApiCryptoError::InvalidPubkeyFormat as i32,
                        )));
                    }
                };

            batch.queue((pubkey, signature, messages[i]));
        }

        // Assaf:
        // To verify a batch of ed25519 signatures we need to provide an RNG source.
        // In theory this doesn't have to be deterministic because the same signatures
        // should produce the same output (true/false) regardless of the RNG being used.
        // In practice I'm too afraid to do something non-deterministic in concensus code
        // So I've decided to use a PRNG instead.
        // For entropy I'm using the entire ed25519 batch verify input data + the gas consumed
        // up until now in this WASM call. This will be deterministic, but also kinda-random in
        // different situations. Note that the gas includes every WASM opcode and
        // every WASM memory allocation up until now.
        // Secret data from the enclave can also be used here but I'm not sure if that's necessary.
        // A few more notes:
        // 1. The vanilla CosmWasm v1 implementation is using RNG from the OS,
        // meaning that different values are used in differents nodes for the same operation inside
        // consensus code, but the output should be the same (https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/ed25519.rs#L108)
        // 2. In Zcash (zebra) this is also used with RNG from the OS, however Zcash is a PoW chain
        // and therefore there's no risk of consensus breaking (https://github.com/ZcashFoundation/zebra/blob/00aa5d96a30539a609bfdd17146b223c4e6cf424/tower-batch/tests/ed25519.rs#L72-L83).
        // 3. In dalek-ed25519 they warn agains using deterministic RNG, as an attacker can derive a falsy signature from the right signature. For me this is an acceptable risk compared to breaking consensus (https://docs.rs/ed25519-dalek/1.0.1/ed25519_dalek/fn.verify_batch.html#on-deterministic-nonces and https://github.com/dalek-cryptography/ed25519-dalek/pull/147).
        let mut rng_entropy: Vec<u8> = vec![];
        rng_entropy.append(&mut messages_data.into_iter().flatten().collect());
        rng_entropy.append(&mut signatures_data.into_iter().flatten().collect());
        rng_entropy.append(&mut pubkeys_data.into_iter().flatten().collect());
        rng_entropy.append(
            &mut (self.gas_used.saturating_add(self.gas_used_externally))
                .to_be_bytes()
                .to_vec(),
        );

        let rng_seed: [u8; 32] = sha_256(&rng_entropy);
        let mut rng = ChaChaRng::from_seed(rng_seed);

        match batch.verify(&mut rng) {
            Err(err) => {
                debug!(
                    "ed25519_batch_verify() failed to verify signatures: {:?}",
                    err
                );

                // return 1 == failed, invalid signature
                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/vm/src/imports.rs#L329
                Ok(Some(RuntimeValue::I32(1)))
            }
            Ok(()) => {
                // return 0 == success, valid signature
                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/vm/src/imports.rs#L329
                Ok(Some(RuntimeValue::I32(0)))
            }
        }
    }

    fn secp256k1_sign(
        &mut self,
        message_ptr: i32,
        private_key_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        self.use_gas(self.gas_costs.external_secp256k1_sign as u64)?;

        let message_data = self.extract_vector(message_ptr as u32).map_err(|err| {
            debug!("secp256k1_sign() error while trying to read message from wasm memory");
            err
        })?;
        let private_key_data = self.extract_vector(private_key_ptr as u32).map_err(|err| {
            debug!("secp256k1_sign() error while trying to read private_key from wasm memory");
            err
        })?;

        trace!(
            "secp256k1_sign() was called from WASM code with message {:x?} (len {:?} should be 32)",
            &message_data,
            message_data.len()
        );
        trace!(
            "secp256k1_sign() was called from WASM code with private_key {:x?} (len {:?} should be 64)",
            &private_key_data,
            private_key_data.len()
        );

        // check private_key input
        if private_key_data.len() != 32 {
            return Ok(Some(RuntimeValue::I64(to_high_half(
                WasmApiCryptoError::InvalidPrivateKeyFormat as u32,
            ) as i64)));
        }

        let secp = secp256k1::Secp256k1::new();

        let message_hash: [u8; 32] = sha_256(message_data.as_slice());
        let secp256k1_msg = match secp256k1::Message::from_slice(&message_hash) {
            Err(err) => {
                debug!(
                    "secp256k1_sign() failed to create a secp256k1 message from message: {:?}",
                    err
                );

                return Ok(Some(RuntimeValue::I64(
                    to_high_half(WasmApiCryptoError::GenericErr as u32) as i64,
                )));
            }
            Ok(x) => x,
        };

        let secp256k1_signing_key = match secp256k1::SecretKey::from_slice(
            private_key_data.as_slice(),
        ) {
            Err(err) => {
                debug!(
                    "secp256k1_sign() failed to create a secp256k1 secret key from private key: {:?}",
                    err
                );

                return Ok(Some(RuntimeValue::I64(to_high_half(
                    WasmApiCryptoError::InvalidPrivateKeyFormat as u32,
                ) as i64)));
            }
            Ok(x) => x,
        };

        let sig = secp
            .sign_ecdsa(&secp256k1_msg, &secp256k1_signing_key)
            .serialize_compact();

        let ptr_to_region_in_wasm_vm = self.write_to_memory(&sig).map_err(|err| {
            debug!(
                "secp256k1_sign() error while trying to allocate and write the sig {:?} to the WASM VM",
                &sig,
            );
            err
        })?;

        // Return pointer to the allocated buffer with the value written to it
        Ok(Some(RuntimeValue::I64(
            to_low_half(ptr_to_region_in_wasm_vm) as i64,
        )))
    }

    fn ed25519_sign(
        &mut self,
        message_ptr: i32,
        private_key_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap> {
        self.use_gas(self.gas_costs.external_ed25519_sign as u64)?;

        let message_data = self.extract_vector(message_ptr as u32).map_err(|err| {
            debug!("ed25519_sign() error while trying to read message from wasm memory");
            err
        })?;
        let private_key_data = self.extract_vector(private_key_ptr as u32).map_err(|err| {
            debug!("ed25519_sign() error while trying to read private_key from wasm memory");
            err
        })?;

        trace!(
            "ed25519_sign() was called from WASM code with message {:x?} (len {:?} should be 32)",
            &message_data,
            message_data.len()
        );
        trace!(
            "ed25519_sign() was called from WASM code with private_key {:x?} (len {:?} should be 64)",
            &private_key_data,
            private_key_data.len()
        );

        // check private_key input
        if private_key_data.len() != 32 {
            return Ok(Some(RuntimeValue::I64(to_high_half(
                WasmApiCryptoError::InvalidPrivateKeyFormat as u32,
            ) as i64)));
        }

        let ed25519_signing_key =
            match ed25519_zebra::SigningKey::try_from(private_key_data.as_slice()) {
                Ok(x) => x,
                Err(err) => {
                    debug!(
                    "ed25519_sign() failed to create an ed25519 signing key from private_key: {:?}",
                    err
                );

                    return Ok(Some(RuntimeValue::I64(to_high_half(
                        WasmApiCryptoError::InvalidPrivateKeyFormat as u32,
                    ) as i64)));
                }
            };

        let sig: [u8; 64] = ed25519_signing_key.sign(message_data.as_slice()).into();

        let ptr_to_region_in_wasm_vm = self.write_to_memory(&sig).map_err(|err| {
            debug!(
                "ed25519_sign() error while trying to allocate and write the sig {:?} to the WASM VM",
                &sig,
            );
            err
        })?;

        // Return pointer to the allocated buffer with the value written to it
        Ok(Some(RuntimeValue::I64(
            to_low_half(ptr_to_region_in_wasm_vm) as i64,
        )))
    }
}

/// Returns the data shifted by 32 bits towards the most significant bit.
///
/// This is independent of endianness. But to get the idea, it would be
/// `data || 0x00000000` in big endian representation.
#[inline]
fn to_high_half(data: u32) -> u64 {
    // See https://stackoverflow.com/a/58956419/2013738 to understand
    // why this is endianness agnostic.
    (data as u64) << 32
}

/// Returns the data copied to the 4 least significant bytes.
///
/// This is independent of endianness. But to get the idea, it would be
/// `0x00000000 || data` in big endian representation.
#[inline]
fn to_low_half(data: u32) -> u64 {
    data.into()
}
