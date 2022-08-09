use std::cell::RefCell;
use std::convert::{TryFrom, TryInto};
use std::marker::PhantomData;
use std::ops::DerefMut;

use log::*;

use bech32::{FromBase32, ToBase32};
use rand_chacha::ChaChaRng;
use rand_core::SeedableRng;
use wasm3::error::Trap;

use enclave_cosmos_types::types::ContractCode;
use enclave_cosmwasm_types::consts::BECH32_PREFIX_ACC_ADDR;
use enclave_crypto::{sha_256, Ed25519PublicKey, WasmApiCryptoError};
use enclave_ffi_types::{Ctx, EnclaveError};

use crate::contract_validation::ContractKey;
use crate::db::read_encrypted_key;
#[cfg(not(feature = "query-only"))]
use crate::db::{remove_encrypted_key, write_encrypted_key};
use crate::errors::{wasm3_error_to_enclave_error, WasmEngineError, WasmEngineResult};
use crate::gas::WasmCosts;
use crate::query_chain::encrypt_and_query_chain;
use crate::types::IoNonce;
use crate::wasm::ContractOperation;

mod gas;
mod validation;

type Wasm3RsError = wasm3::error::Error;
type Wasm3RsResult<T> = wasm3::error::Result<T>;

macro_rules! debug_err {
    ($message: literal) => {
        |err| { debug!($message); err }
    };
    ($message: literal, $($args: tt)*) => {
        |err| { debug!($message, $($args)*); err }
    };
    ($err: ident => $message: literal) => {
        |$err| { debug!($message, $err = $err); $err }
    };
    ($err: ident => $message: literal, $($args: tt)*) => {
        |$err| { debug!($message, $($args)*, $err = $err); $err }
    };
}

macro_rules! set_last_error {
    ($context: expr) => {
        |err| {
            $context.set_last_error(err);
            wasm3::error::Trap::Exit
        }
    };
}

macro_rules! link_fn {
    ($module: expr, $context_ptr: expr, $name: expr, $implementation: expr) => {
        $module
            .link_closure("env", $name, move |call_context, args| {
                debug!("{} was called", $name);
                let context = unsafe { &*$context_ptr };
                let mut context = context.borrow_mut();
                let ret = $implementation(&mut context, call_context, args)
                    .map_err(set_last_error!(context));
                debug!("{} finished", $name);
                ret
            })
            .allow_missing_import()
    };
}

trait Wasm3ResultEx {
    fn allow_missing_import(self) -> Self;
}

impl Wasm3ResultEx for Wasm3RsResult<()> {
    fn allow_missing_import(self) -> Self {
        match self {
            Err(Wasm3RsError::FunctionNotFound) => Ok(()),
            // Workaround for erroneous non-enumerated error in this case in wasm3.
            // Search for the string "function signature mismatch" in the C source
            Err(Wasm3RsError::Wasm3(wasm3_error)) if Trap::from(wasm3_error) == Trap::Abort => {
                Err(Wasm3RsError::InvalidFunctionSignature)
            }
            other => other,
        }
    }
}

pub struct Context {
    context: Ctx,
    gas_limit: u64,
    /// Gas used by wasmi
    gas_used: u64,
    /// Gas used by external services. This is tracked separately so we don't double-charge for external services later.
    gas_used_externally: u64,
    gas_costs: WasmCosts,
    contract_key: ContractKey,
    #[cfg_attr(feature = "query-only", allow(unused))]
    operation: ContractOperation,
    user_nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
    last_error: Option<WasmEngineError>,
}

impl Context {
    pub fn take_last_error(&mut self) -> Option<WasmEngineError> {
        self.last_error.take()
    }

    pub fn set_last_error(&mut self, error: WasmEngineError) {
        self.last_error = Some(error);
    }

    /// Track gas used inside wasmi
    fn use_gas(&mut self, gas_amount: u64) -> Result<(), WasmEngineError> {
        self.gas_used = self.gas_used.saturating_add(gas_amount);
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

    fn use_gas_externally(&mut self, gas_amount: u64) -> WasmEngineResult<()> {
        self.gas_used_externally = self.gas_used_externally.saturating_add(gas_amount);
        self.check_gas_usage()
    }

    fn gas_left(&self) -> u64 {
        self.gas_limit
            .saturating_sub(self.gas_used)
            .saturating_sub(self.gas_used_externally)
    }
}

pub struct Engine {
    // WARNING!
    // This box is dropped when the Engine is dropped. You MUST NOT use the pointer
    // after destroying the engine. Using this pointer in the `host_*` functions
    // is only legal because we do not provide direct access to the `runtime` field outside
    // the Engine. We also use a RefCell to ensure that we don't access the Context incorrectly.
    context: *mut RefCell<Context>,
    environment: wasm3::Environment,
    runtime: wasm3::Runtime,
}

impl Engine {
    pub fn new(
        context: Ctx,
        gas_limit: u64,
        gas_costs: WasmCosts,
        contract_code: ContractCode,
        contract_key: ContractKey,
        operation: ContractOperation,
        user_nonce: IoNonce,
        user_public_key: Ed25519PublicKey,
    ) -> Result<Engine, EnclaveError> {
        let mut module = walrus::ModuleConfig::new()
            .generate_producers_section(false)
            .parse(contract_code.code())
            .map_err(|_| EnclaveError::InvalidWasm)?;

        validation::validate_memory(&mut module)?;

        if let ContractOperation::Init = operation {
            validation::deny_floating_point(&module)?;
        }

        gas::add_metering(&mut module);

        let transformed_code = module.emit_wasm();

        let context = Context {
            context,
            gas_limit,
            gas_used: 0,
            gas_used_externally: 0,
            gas_costs,
            contract_key,
            operation,
            user_nonce,
            user_public_key,
            last_error: None,
        };
        let context = Box::new(RefCell::new(context));
        let context_ptr = Box::into_raw(context);

        Engine::setup_runtime(context_ptr, &transformed_code, gas_limit).map_err(|err| unsafe {
            let context = &*context_ptr;
            let mut context = context.borrow_mut();
            wasm3_error_to_enclave_error(context.deref_mut(), err)
        })
    }

    fn setup_runtime(
        context_ptr: *mut RefCell<Context>,
        contract_bytes: &[u8],
        gas_limit: u64,
    ) -> Wasm3RsResult<Engine> {
        debug!("setting up runtime");
        let environment = wasm3::Environment::new()?;
        debug!("initialized environment");
        let runtime = environment.create_runtime(1024 * 60)?;
        debug!("initialized runtime");

        let mut module = runtime.parse_and_load_module(contract_bytes)?;
        debug!("parsed module");

        gas::set_gas_limit(&module, gas_limit);
        debug!("set gas limit");

        #[rustfmt::skip] {
        link_fn!(module, context_ptr, "db_read", host_read_db)?;
        #[cfg(not(feature = "query-only"))] {
            link_fn!(module, context_ptr, "db_write", host_write_db)?;
            link_fn!(module, context_ptr, "db_remove", host_remove_db)?;
        }
        link_fn!(module, context_ptr, "canonicalize_address", host_canonicalize_address)?;
        link_fn!(module, context_ptr, "humanize_address", host_humanize_address)?;
        link_fn!(module, context_ptr, "query_chain", host_query_chain)?;
        link_fn!(module, context_ptr, "debug_print", host_debug_print)?;
        link_fn!(module, context_ptr, "gas", host_gas)?;
        link_fn!(module, context_ptr, "secp256k1_verify", host_secp256k1_verify)?;
        link_fn!(module, context_ptr, "secp256k1_recover_pubkey", host_secp256k1_recover_pubkey)?;
        link_fn!(module, context_ptr, "ed25519_verify", host_ed25519_verify)?;
        link_fn!(module, context_ptr, "ed25519_batch_verify", host_ed25519_batch_verify)?;
        link_fn!(module, context_ptr, "secp256k1_sign", host_secp256k1_sign)?;
        link_fn!(module, context_ptr, "ed25519_sign", host_ed25519_sign)?;
        }
        debug!("linked functions");

        Ok(Self {
            context: context_ptr,
            environment,
            runtime,
        })
    }

    fn init_fn(&self) -> Wasm3RsResult<wasm3::Function<(u32, u32), u32>> {
        self.runtime.find_function::<(u32, u32), u32>("init")
    }

    fn handle_fn(&self) -> Wasm3RsResult<wasm3::Function<(u32, u32), u32>> {
        self.runtime.find_function::<(u32, u32), u32>("handle")
    }

    fn query_fn(&self) -> Wasm3RsResult<wasm3::Function<u32, u32>> {
        self.runtime.find_function::<u32, u32>("query")
    }

    pub fn gas_used(&self) -> u64 {
        // let context = unsafe { &*self.context };
        // let context = context.borrow();
        // context.gas_used
        0
    }

    pub fn write_to_memory(&mut self, buffer: &[u8]) -> Result<u32, WasmEngineError> {
        write_to_memory(&mut self.runtime, buffer)
    }

    pub fn extract_vector(&mut self, region_ptr: u32) -> Result<Vec<u8>, WasmEngineError> {
        let mem = CWMemory::new(&mut self.runtime);
        mem.extract_vector(region_ptr)
    }

    pub fn init(&mut self, env_ptr: u32, msg_ptr: u32) -> Result<u32, EnclaveError> {
        let handle_wasm3_err = |err| unsafe {
            debug!("init failed with {:?}", err);
            let context = &*self.context;
            let mut context = context.borrow_mut();
            debug!("borrowed context");
            wasm3_error_to_enclave_error(context.deref_mut(), err)
        };

        debug!("init starting");
        let ret = self
            .init_fn()
            .map_err(handle_wasm3_err)?
            .call(env_ptr, msg_ptr)
            .map_err(handle_wasm3_err);
        debug!("init returned {:?}", ret);
        ret
    }

    pub fn handle(&mut self, env_ptr: u32, msg_ptr: u32) -> Result<u32, EnclaveError> {
        let handle_wasm3_err = |err| unsafe {
            debug!("handle failed with {:?}", err);
            let context = &*self.context;
            let mut context = context.borrow_mut();
            debug!("borrowed context");
            wasm3_error_to_enclave_error(context.deref_mut(), err)
        };

        debug!("handle starting");
        let ret = self
            .handle_fn()
            .map_err(handle_wasm3_err)?
            .call(env_ptr, msg_ptr)
            .map_err(handle_wasm3_err);
        debug!("handle returned {:?}", ret);
        ret
    }

    pub fn query(&mut self, msg_ptr: u32) -> Result<u32, EnclaveError> {
        let handle_wasm3_err = |err| unsafe {
            debug!("query failed with {:?}", err);
            let context = &*self.context;
            let mut context = context.borrow_mut();
            debug!("borrowed context");
            wasm3_error_to_enclave_error(context.deref_mut(), err)
        };

        debug!("query starting");
        let ret = self
            .query_fn()
            .map_err(handle_wasm3_err)?
            .call(msg_ptr)
            .map_err(handle_wasm3_err);
        debug!("query returned {:?}", ret);
        ret
    }
}

impl Drop for Engine {
    fn drop(&mut self) {
        let context = unsafe { Box::from_raw(self.context) };
        drop(context)
    }
}

struct CWMemory<'m> {
    memory: &'m mut [u8],
    _phantom: PhantomData<&'m mut wasm3::Runtime>,
}

const SIZE_OF_U32: usize = std::mem::size_of::<u32>();

impl<'m> CWMemory<'m> {
    fn new(runtime: &'m mut wasm3::Runtime) -> Self {
        Self {
            memory: runtime.memory(),
            _phantom: PhantomData,
        }
    }

    fn get_u32_at(&self, idx: u32) -> WasmEngineResult<u32> {
        let idx = idx as usize;
        let bytes: [u8; SIZE_OF_U32] = self
            .memory
            .get(idx..idx + SIZE_OF_U32)
            .ok_or(WasmEngineError::MemoryReadError)?
            .try_into()
            .map_err(|_| WasmEngineError::MemoryReadError)?;
        Ok(u32::from_le_bytes(bytes))
    }

    fn set_u32_at(&mut self, idx: u32, val: u32) -> WasmEngineResult<u32> {
        let i = idx as usize;
        self.memory
            .get_mut(i..i + SIZE_OF_U32)
            .ok_or(WasmEngineError::MemoryReadError)?
            .copy_from_slice(&val.to_le_bytes());
        Ok(idx)
    }

    fn extract_vector(&self, region_ptr: u32) -> WasmEngineResult<Vec<u8>> {
        if region_ptr == 0 {
            debug!("vec_ptr_ptr is null");
            return Err(WasmEngineError::MemoryReadError);
        }

        let vec_ptr = self.get_u32_at(region_ptr)? as usize;
        let vec_len = self.get_u32_at(region_ptr + (SIZE_OF_U32 as u32) * 2)? as usize;
        if vec_ptr == 0 {
            return Err(WasmEngineError::MemoryReadError);
        }

        match self.memory.get(vec_ptr..vec_ptr + vec_len) {
            Some(slice) => Ok(slice.to_owned()),
            None => Err(WasmEngineError::MemoryReadError),
        }
    }

    fn decode_sections(&self, region_ptr: u32) -> WasmEngineResult<Vec<Vec<u8>>> {
        if region_ptr == 0 {
            debug!("region_ptr is null");
            return Err(WasmEngineError::MemoryReadError);
        }

        let data_ptr = self.get_u32_at(region_ptr)? as usize;
        if data_ptr == 0 {
            debug!("data_ptr is null");
            return Err(WasmEngineError::MemoryReadError);
        }

        let data_len = self.get_u32_at(region_ptr + (SIZE_OF_U32 as u32) * 2)? as usize;
        let mut remaining_len = data_len as usize;

        let data = self.memory.get(data_ptr..data_ptr + data_len);
        let data = data.ok_or(WasmEngineError::MemoryReadError)?;

        let mut result: Vec<Vec<u8>> = vec![];
        while remaining_len >= 4 {
            let tail_len = u32::from_be_bytes([
                data[remaining_len - 4],
                data[remaining_len - 3],
                data[remaining_len - 2],
                data[remaining_len - 1],
            ]) as usize;
            let mut new_element = vec![0; tail_len];
            let elem_start = remaining_len - SIZE_OF_U32 - tail_len;
            let elem_end = remaining_len - SIZE_OF_U32;
            new_element.copy_from_slice(&data[elem_start..elem_end]);
            result.push(new_element);
            remaining_len -= 4 + tail_len;
        }
        result.reverse();

        Ok(result)
    }

    fn write_to_allocated_memory(
        &mut self,
        region_ptr: u32,
        buffer: &[u8],
    ) -> WasmEngineResult<u32> {
        let vec_ptr = self.get_u32_at(region_ptr)?;
        if vec_ptr == 0 {
            return Err(WasmEngineError::MemoryReadError);
        }
        let vec_len = self.get_u32_at(region_ptr + SIZE_OF_U32 as u32)?;
        if (vec_len as usize) < buffer.len() {
            return Err(WasmEngineError::MemoryReadError);
        }

        let idx = vec_ptr as usize;
        self.memory
            .get_mut(idx..idx + buffer.len())
            .ok_or(WasmEngineError::MemoryReadError)?
            .copy_from_slice(buffer);
        self.set_u32_at(region_ptr + (SIZE_OF_U32 * 2) as u32, buffer.len() as u32)?;

        Ok(region_ptr)
    }
}

fn write_to_memory(runtime: &mut wasm3::Runtime, buffer: &[u8]) -> WasmEngineResult<u32> {
    let region_ptr = (|| {
        let alloc_fn = runtime.find_function::<u32, u32>("allocate")?;
        alloc_fn.call(buffer.len() as u32)
    })()
    .map_err(debug_err!(err => "failed to allocate {} bytes in contract: {err}", buffer.len()))
    .map_err(|_| WasmEngineError::MemoryAllocationError)?;
    let mut memory = CWMemory::new(runtime);
    memory
        .write_to_allocated_memory(region_ptr, buffer)
        .map_err(debug_err!(err => "failed to write to contract memory {err}"))
}

fn show_bytes(bytes: &[u8]) -> String {
    format!(
        "{:?} ({})",
        String::from_utf8_lossy(bytes),
        hex::encode(bytes)
    )
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

fn host_read_db(
    context: &mut Context,
    mut call_context: wasm3::CallContext,
    state_key_region_ptr: i32,
) -> WasmEngineResult<i32> {
    let memory = CWMemory::new(&mut call_context.runtime);

    let state_key_name = memory.extract_vector(state_key_region_ptr as u32).map_err(
        debug_err!(err => "db_read failed to extract vector from state_key_region_ptr: {err}"),
    )?;

    debug!("db_read reading key {}", show_bytes(&state_key_name));

    let (value, gas_used) =
        read_encrypted_key(&state_key_name, &context.context, &context.contract_key)
            .map_err(debug_err!("db_read failed to read key from storage"))?;
    context.use_gas_externally(gas_used)?;

    debug!(
        "db_read received value {:?}",
        value.as_ref().map(|v| show_bytes(&v))
    );

    let value = match value {
        // Return 0 (null ponter) if value is empty
        Some(value) => value,
        None => return Ok(0),
    };

    let region_ptr = write_to_memory(&mut call_context.runtime, &value)?;

    Ok(region_ptr as i32)
}

#[cfg(not(feature = "query-only"))]
fn host_remove_db(
    context: &mut Context,
    mut call_context: wasm3::CallContext,
    state_key_region_ptr: i32,
) -> WasmEngineResult<()> {
    let memory = CWMemory::new(&mut call_context.runtime);

    if context.operation.is_query() {
        debug!("db_remove was called while in query mode");
        return Err(WasmEngineError::UnauthorizedWrite);
    }

    let state_key_name = memory.extract_vector(state_key_region_ptr as u32).map_err(
        debug_err!(err => "db_remove failed to extract vector from state_key_region_ptr: {err}"),
    )?;

    debug!("db_remove removing key {}", show_bytes(&state_key_name));

    let gas_used = remove_encrypted_key(&state_key_name, &context.context, &context.contract_key)?;
    context.use_gas_externally(gas_used)?;

    Ok(())
}

#[cfg(not(feature = "query-only"))]
fn host_write_db(
    context: &mut Context,
    mut call_context: wasm3::CallContext,
    (state_key_region_ptr, value_region_ptr): (i32, i32),
) -> WasmEngineResult<()> {
    let memory = CWMemory::new(&mut call_context.runtime);

    if context.operation.is_query() {
        debug!("db_write was called while in query mode");
        return Err(WasmEngineError::UnauthorizedWrite);
    }

    let state_key_name = memory.extract_vector(state_key_region_ptr as u32).map_err(
        debug_err!(err => "db_write failed to extract vector from state_key_region_ptr: {err}"),
    )?;

    let value = memory.extract_vector(value_region_ptr as u32).map_err(
        debug_err!(err => "db_write failed to extract vector from value_region_ptr: {err}"),
    )?;

    debug!(
        "db_write writing key: {}, value: {}",
        show_bytes(&state_key_name),
        show_bytes(&value)
    );

    let used_gas = write_encrypted_key(
        &state_key_name,
        &value,
        &context.context,
        &context.contract_key,
    )
    .map_err(debug_err!("db_write failed to write key to storage",))?;
    context.use_gas_externally(used_gas)?;

    Ok(())
}

fn host_canonicalize_address(
    context: &mut Context,
    mut call_context: wasm3::CallContext,
    (human_region_ptr, canonical_region_ptr): (i32, i32),
) -> WasmEngineResult<i32> {
    let mut memory = CWMemory::new(&mut call_context.runtime);

    let cost = context.gas_costs.external_canonicalize_address as u64;
    context.use_gas_externally(cost)?;

    let human = memory
        .extract_vector(human_region_ptr as u32)
        .map_err(
            debug_err!(err => "canonicalize_address failed to extract vector from human_region_ptr: {err}"),
        )?;

    let mut human_addr_str = match std::str::from_utf8(&human) {
        Ok(addr) => addr,
        Err(_err) => {
            debug!(
                "canonicalize_address input was not valid UTF-8: {}",
                show_bytes(&human)
            );
            return write_to_memory(&mut call_context.runtime, b"input is not valid UTF-8")
                .map(|n| n as i32)
                .map_err(debug_err!("failed to write error message to contract"));
        }
    };
    human_addr_str = human_addr_str.trim();
    if human_addr_str.is_empty() {
        debug!("canonicalize_address input was empty");
        return write_to_memory(&mut call_context.runtime, b"input is empty")
            .map(|n| n as i32)
            .map_err(debug_err!("failed to write error message to contract"));
    }

    debug!("canonicalize_address was called with {:?}", human_addr_str);

    let (decoded_prefix, data) = match bech32::decode(&human_addr_str) {
        Ok(ret) => ret,
        Err(err) => {
            debug!(
                "canonicalize_address failed to parse input as bech32: {:?}",
                err
            );
            return write_to_memory(&mut call_context.runtime, err.to_string().as_bytes())
                .map(|n| n as i32)
                .map_err(debug_err!("failed to write error message to contract"));
        }
    };

    if decoded_prefix != BECH32_PREFIX_ACC_ADDR {
        debug!("canonicalize_address was called with an unexpected address prefix");
        return write_to_memory(
            &mut call_context.runtime,
            format!("wrong address prefix: {:?}", decoded_prefix).as_bytes(),
        )
        .map(|n| n as i32)
        .map_err(debug_err!("failed to write error message to contract"));
    }

    let canonical = Vec::<u8>::from_base32(&data).map_err(|err| {
        // Assaf: From reading https://docs.rs/bech32/0.7.2/src/bech32/lib.rs.html#607
        // and https://docs.rs/bech32/0.7.2/src/bech32/lib.rs.html#228 I don't think this can fail that way
        debug!("canonicalize_address failed to parse base32: {}", err);
        WasmEngineError::Base32Error
    })?;

    debug!(
        "canonicalize_address returning address {}",
        hex::encode(human_addr_str)
    );

    memory
        .write_to_allocated_memory(canonical_region_ptr as u32, &canonical)
        .map_err(debug_err!(
            "canonicalize_address failed to write to canonical_region_ptr"
        ))?;

    // return 0 == ok
    Ok(0)
}

fn host_humanize_address(
    context: &mut Context,
    mut call_context: wasm3::CallContext,
    (canonical_region_ptr, human_region_ptr): (i32, i32),
) -> WasmEngineResult<i32> {
    let mut memory = CWMemory::new(&mut call_context.runtime);

    let cost = context.gas_costs.external_canonicalize_address as u64;
    context.use_gas_externally(cost)?;

    let canonical = memory
        .extract_vector(canonical_region_ptr as u32)
        .map_err(
            debug_err!(err => "humanize_address failed to extract vector from canonical_region_ptr: {err}"),
        )?;

    debug!(
        "humanize_address was called with {}",
        hex::encode(&canonical)
    );

    let human_addr_str = match bech32::encode(BECH32_PREFIX_ACC_ADDR, canonical.to_base32()) {
        Ok(addr) => addr,
        Err(err) => {
            debug!("humanize_address failed to encode address as bech32");
            return write_to_memory(&mut call_context.runtime, err.to_string().as_bytes())
                .map(|n| n as i32)
                .map_err(debug_err!("failed to write error message to contract"));
        }
    };

    debug!("humanize_address returning address {}", human_addr_str);

    let human_bytes = human_addr_str.into_bytes();

    memory
        .write_to_allocated_memory(human_region_ptr as u32, &human_bytes)
        .map_err(debug_err!(
            "humanize_address failed to write to canonical_region_ptr"
        ))?;

    // return 0 == ok
    Ok(0)
}

fn host_query_chain(
    context: &mut Context,
    mut call_context: wasm3::CallContext,
    query_region_ptr: i32,
) -> WasmEngineResult<i32> {
    let memory = CWMemory::new(&mut call_context.runtime);

    let query_buffer = memory.extract_vector(query_region_ptr as u32).map_err(
        debug_err!(err => "query_chain failed to extract vector from query_region_ptr: {err}"),
    )?;

    let mut gas_used: u64 = 0;
    let answer = encrypt_and_query_chain(
        &query_buffer,
        &context.context,
        context.user_nonce,
        context.user_public_key,
        &mut gas_used,
        context.gas_left(),
    )?;

    context.use_gas_externally(gas_used)?;

    write_to_memory(&mut call_context.runtime, &answer).map(|region_ptr| region_ptr as i32)
}

fn host_debug_print(
    _context: &mut Context,
    mut call_context: wasm3::CallContext,
    message_region_ptr: i32,
) -> WasmEngineResult<()> {
    let memory = CWMemory::new(&mut call_context.runtime);

    let message_buffer = memory.extract_vector(message_region_ptr as u32)?;

    let message =
        String::from_utf8(message_buffer).unwrap_or_else(|err| hex::encode(err.into_bytes()));

    info!("debug_print: {:?}", message);

    Ok(())
}

fn host_gas(
    context: &mut Context,
    mut _call_context: wasm3::CallContext,
    gas_amount: i32,
) -> WasmEngineResult<()> {
    context.use_gas(gas_amount as u64)?;
    Ok(())
}

fn host_secp256k1_verify(
    context: &mut Context,
    mut call_context: wasm3::CallContext,
    (message_hash_ptr, signature_ptr, public_key_ptr): (i32, i32, i32),
) -> WasmEngineResult<i32> {
    let memory = CWMemory::new(&mut call_context.runtime);

    let cost = context.gas_costs.external_secp256k1_verify as u64;
    context.use_gas_externally(cost)?;

    let message_hash_data = memory.extract_vector(message_hash_ptr as u32).map_err(
    debug_err!(err => "secp256k1_verify error while trying to read message_hash from wasm memory: {err}")
    )?;
    let signature_data = memory.extract_vector(signature_ptr as u32).map_err(
        debug_err!(err => "secp256k1_verify error while trying to read signature from wasm memory: {err}")
    )?;
    let public_key = memory.extract_vector(public_key_ptr as u32).map_err(
        debug_err!(err => "secp256k1_verify error while trying to read public_key from wasm memory: {err}")
    )?;

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
        return Ok(WasmApiCryptoError::InvalidHashFormat as i32);
    }

    // check signature input
    if signature_data.len() != 64 {
        // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L94
        return Ok(WasmApiCryptoError::InvalidSignatureFormat as i32);
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
        return Ok(WasmApiCryptoError::InvalidPubkeyFormat as i32);
    }

    let secp256k1_msg = match secp256k1::Message::from_slice(&message_hash_data) {
        Err(err) => {
            debug!(
                "secp256k1_verify failed to create a secp256k1 message from message_hash: {:?}",
                err
            );
            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L98
            return Ok(WasmApiCryptoError::GenericErr as i32);
        }
        Ok(x) => x,
    };

    let secp256k1_sig = match secp256k1::ecdsa::Signature::from_compact(&signature_data) {
        Err(err) => {
            debug!("secp256k1_verify() malformed signature: {:?}", err);
            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L98
            return Ok(WasmApiCryptoError::GenericErr as i32);
        }
        Ok(x) => x,
    };

    let secp256k1_pk = match secp256k1::PublicKey::from_slice(public_key.as_slice()) {
        Err(err) => {
            debug!("secp256k1_verify() malformed pubkey: {:?}", err);
            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L98
            return Ok(WasmApiCryptoError::GenericErr as i32);
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
            Ok(1)
        }
        Ok(()) => {
            // return 0 == success, valid signature
            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/vm/src/imports.rs#L220
            Ok(0)
        }
    }
}

fn host_secp256k1_recover_pubkey(
    context: &mut Context,
    mut call_context: wasm3::CallContext,
    (message_hash_ptr, signature_ptr, recovery_param): (i32, i32, i32),
) -> WasmEngineResult<i64> {
    let memory = CWMemory::new(&mut call_context.runtime);

    let cost = context.gas_costs.external_secp256k1_recover_pubkey as u64;
    context.use_gas_externally(cost)?;

    let message_hash_data = memory.extract_vector(message_hash_ptr as u32).map_err(
        debug_err!(err => "secp256k1_recover_pubkey error while trying to read message_hash from wasm memory: {err}")
    )?;
    let signature_data = memory.extract_vector(signature_ptr as u32).map_err(
        debug_err!(err => "secp256k1_recover_pubkey error while trying to read signature from wasm memory: {err}")
    )?;

    trace!(
        "secp256k1_recover_pubkey was called from WASM code with message_hash {:x?} (len {:?} should be 32)",
        &message_hash_data,
        message_hash_data.len()
    );
    trace!(
        "secp256k1_recover_pubkey was called from WASM code with signature {:x?} (len {:?} should be 64)",
        &signature_data,
        signature_data.len()
    );
    trace!(
        "secp256k1_recover_pubkey was called from WASM code with recovery_param {:?}",
        recovery_param,
    );

    // check message_hash input
    if message_hash_data.len() != 32 {
        // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L93
        return Ok(to_high_half(WasmApiCryptoError::InvalidHashFormat as u32) as i64);
    }

    // check signature input
    if signature_data.len() != 64 {
        // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L94
        return Ok(to_high_half(WasmApiCryptoError::InvalidSignatureFormat as u32) as i64);
    }

    let secp256k1_msg = match secp256k1::Message::from_slice(&message_hash_data) {
        Err(err) => {
            debug!("secp256k1_recover_pubkey() failed to create a secp256k1 message from message_hash: {:?}", err);

            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L98
            return Ok(to_high_half(WasmApiCryptoError::GenericErr as u32) as i64);
        }
        Ok(x) => x,
    };

    let recovery_id = match secp256k1::ecdsa::RecoveryId::from_i32(recovery_param) {
        Err(err) => {
            debug!("secp256k1_recover_pubkey() failed to create a secp256k1 recovery_id from recovery_param: {:?}", err);

            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L98
            return Ok(to_high_half(WasmApiCryptoError::GenericErr as u32) as i64);
        }
        Ok(x) => x,
    };

    let secp256k1_sig =
        match secp256k1::ecdsa::RecoverableSignature::from_compact(&signature_data, recovery_id) {
            Err(err) => {
                debug!(
                    "secp256k1_recover_pubkey() malformed recoverable signature: {:?}",
                    err
                );

                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L98
                return Ok(to_high_half(WasmApiCryptoError::GenericErr as u32) as i64);
            }
            Ok(x) => x,
        };

    match secp256k1::Secp256k1::verification_only().recover_ecdsa(&secp256k1_msg, &secp256k1_sig) {
        Err(err) => {
            debug!(
                "secp256k1_recover_pubkey() failed to recover pubkey: {:?}",
                err
            );

            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L98
            Ok(to_high_half(WasmApiCryptoError::GenericErr as u32) as i64)
        }
        Ok(pubkey) => {
            let answer = pubkey.serialize();
            let ptr_to_region_in_wasm_vm = write_to_memory(&mut call_context.runtime, &answer).map_err(|err| {
                debug!(
                        "secp256k1_recover_pubkey() error while trying to allocate and write the answer {:?} to the WASM VM",
                        &answer,
                    );
                err
            })?;

            // Return pointer to the allocated buffer with the value written to it
            Ok(to_low_half(ptr_to_region_in_wasm_vm) as i64)
        }
    }
}

fn host_ed25519_verify(
    context: &mut Context,
    mut call_context: wasm3::CallContext,
    (message_ptr, signature_ptr, public_key_ptr): (i32, i32, i32),
) -> WasmEngineResult<i32> {
    let memory = CWMemory::new(&mut call_context.runtime);

    let cost = context.gas_costs.external_ed25519_verify as u64;
    context.use_gas_externally(cost)?;

    let message_data = memory.extract_vector(message_ptr as u32).map_err(
        debug_err!(err => "ed25519_verify error while trying to read message_hash from wasm memory: {err}")
    )?;
    let signature_data = memory.extract_vector(signature_ptr as u32).map_err(
        debug_err!(err => "ed25519_verify error while trying to read signature from wasm memory: {err}")
    )?;
    let public_key_data = memory.extract_vector(public_key_ptr as u32).map_err(
        debug_err!(err => "ed25519_verify error while trying to read public_key from wasm memory: {err}")
    )?;

    trace!(
        "ed25519_verify was called from WASM code with message {:x?} (len {:?})",
        &message_data,
        message_data.len()
    );
    trace!(
        "ed25519_verify was called from WASM code with signature {:x?} (len {:?} should be 64)",
        &signature_data,
        signature_data.len()
    );
    trace!(
        "ed25519_verify was called from WASM code with public_key {:x?} (len {:?} should be 32)",
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
                return Ok(WasmApiCryptoError::InvalidSignatureFormat as i32);
            }
        };

    let public_key: ed25519_zebra::VerificationKey = match ed25519_zebra::VerificationKey::try_from(
        public_key_data.as_slice(),
    ) {
        Ok(x) => x,
        Err(err) => {
            debug!(
                        "ed25519_verify() failed to create an ed25519 VerificationKey from public_key: {:?}",
                        err
                    );

            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L95
            return Ok(WasmApiCryptoError::InvalidPubkeyFormat as i32);
        }
    };

    match public_key.verify(&signature, &message_data) {
        Err(err) => {
            debug!("ed25519_verify() failed to verify signature: {:?}", err);

            // return 1 == failed, invalid signature
            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/vm/src/imports.rs#L281
            Ok(1)
        }
        Ok(()) => {
            // return 0 == success, valid signature
            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/vm/src/imports.rs#L281
            Ok(0)
        }
    }
}

fn host_ed25519_batch_verify(
    context: &mut Context,
    mut call_context: wasm3::CallContext,
    (messages_ptr, signatures_ptr, public_keys_ptr): (i32, i32, i32),
) -> WasmEngineResult<i32> {
    let memory = CWMemory::new(&mut call_context.runtime);

    let messages_data = memory
        .decode_sections(messages_ptr as u32)
        .map_err(debug_err!(
            err => "ed25519_batch_verify error while trying to read messages from wasm memory: {err}"
        ))?;

    let signatures_data = memory
        .decode_sections(signatures_ptr as u32)
        .map_err(debug_err!(
            err => "ed25519_batch_verify error while trying to read signatures from wasm memory: {err}"
        ))?;

    let pubkeys_data = memory
        .decode_sections(public_keys_ptr as u32)
        .map_err(debug_err!(
            err => "ed25519_batch_verify error while trying to read public_keys from wasm memory: {err}"
        ))?;

    let messages_len = messages_data.len();
    let signatures_len = signatures_data.len();
    let pubkeys_len = pubkeys_data.len();

    if messages_len == 0 || signatures_len == 0 || pubkeys_len == 0 {
        debug!("ed25519_batch_verify trying to bach verify with empty inputs!");
        return Err(WasmEngineError::MemoryReadError);
    }

    let lengths = (messages_len, signatures_len, pubkeys_len);
    let (messages, signatures, pubkeys): (Vec<&[u8]>, Vec<&[u8]>, Vec<&[u8]>) = match lengths {
        (ml, sl, pl) if ml == sl && sl == pl => {
            let messages = messages_data.iter().map(Vec::as_slice).collect();
            let signatures = signatures_data.iter().map(Vec::as_slice).collect();
            let pubkeys = pubkeys_data.iter().map(Vec::as_slice).collect();
            (messages, signatures, pubkeys)
        }
        (ml, sl, pl) if ml == 1 && sl == pl => {
            let messages = vec![messages_data[0].as_slice()].repeat(signatures_len);
            let signatures = signatures_data.iter().map(Vec::as_slice).collect();
            let pubkeys = pubkeys_data.iter().map(Vec::as_slice).collect();
            (messages, signatures, pubkeys)
        }
        (ml, sl, pl) if ml == sl && pl == 1 => {
            let messages = messages_data.iter().map(Vec::as_slice).collect();
            let signatures = signatures_data.iter().map(Vec::as_slice).collect();
            let pubkeys = vec![pubkeys_data[0].as_slice()].repeat(signatures_len);
            (messages, signatures, pubkeys)
        }
        _ => {
            debug!(
                "ed25519_batch_verify() mismatched number of messages ({}) / signatures ({}) / public keys ({})",
                messages_len,
                signatures_len,
                pubkeys_len,
            );

            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L97
            return Ok(WasmApiCryptoError::BatchErr as i32);
        }
    };

    let base_cost = context.gas_costs.external_ed25519_batch_verify_base as u64;
    let each_cost = context.gas_costs.external_ed25519_batch_verify_each as u64;
    context.use_gas_externally(base_cost + (signatures.len() as u64) * each_cost)?;

    let mut batch = ed25519_zebra::batch::Verifier::new();
    for i in 0..signatures.len() {
        let signature = match ed25519_zebra::Signature::try_from(signatures[i]) {
            Ok(x) => x,
            Err(err) => {
                debug!(
                    "ed25519_batch_verify() failed to create an ed25519 signature from signatures[{}]: {:?}",
                    i, err
                );

                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L94
                return Ok(WasmApiCryptoError::InvalidSignatureFormat as i32);
            }
        };

        let pubkey = match ed25519_zebra::VerificationKeyBytes::try_from(pubkeys[i]) {
            Ok(x) => x,
            Err(err) => {
                debug!(
                        "ed25519_batch_verify() failed to create an ed25519 VerificationKey from public_keys[{}]: {:?}",
                        i, err
                    );

                // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/crypto/src/errors.rs#L95
                return Ok(WasmApiCryptoError::InvalidPubkeyFormat as i32);
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
        &mut (context.gas_used.saturating_add(context.gas_used_externally))
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
            Ok(1)
        }
        Ok(()) => {
            // return 0 == success, valid signature
            // https://github.com/CosmWasm/cosmwasm/blob/v1.0.0-beta5/packages/vm/src/imports.rs#L329
            Ok(0)
        }
    }
}

fn host_secp256k1_sign(
    context: &mut Context,
    mut call_context: wasm3::CallContext,
    (message_ptr, private_key_ptr): (i32, i32),
) -> WasmEngineResult<i64> {
    let memory = CWMemory::new(&mut call_context.runtime);

    let cost = context.gas_costs.external_secp256k1_sign as u64;
    context.use_gas_externally(cost)?;

    let message_data = memory.extract_vector(message_ptr as u32).map_err(
        debug_err!(err => "secp256k1_sign error while trying to read message_hash from wasm memory: {err}")
    )?;
    let private_key_data = memory.extract_vector(private_key_ptr as u32).map_err(
        debug_err!(err => "secp256k1_sign error while trying to read private key from wasm memory: {err}")
    )?;

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

    if private_key_data.len() != 32 {
        return Ok(to_high_half(WasmApiCryptoError::InvalidPrivateKeyFormat as u32) as i64);
    }

    let secp = secp256k1::Secp256k1::new();

    let message_hash: [u8; 32] = sha_256(message_data.as_slice());
    let secp256k1_msg = match secp256k1::Message::from_slice(&message_hash) {
        Err(err) => {
            debug!(
                "secp256k1_sign() failed to create a secp256k1 message from message: {:?}",
                err
            );

            return Ok(to_high_half(WasmApiCryptoError::GenericErr as u32) as i64);
        }
        Ok(x) => x,
    };

    let secp256k1_signing_key = match secp256k1::SecretKey::from_slice(private_key_data.as_slice())
    {
        Err(err) => {
            debug!(
                "secp256k1_sign() failed to create a secp256k1 secret key from private key: {:?}",
                err
            );

            return Ok(to_high_half(WasmApiCryptoError::InvalidPrivateKeyFormat as u32) as i64);
        }
        Ok(x) => x,
    };

    let sig = secp
        .sign_ecdsa(&secp256k1_msg, &secp256k1_signing_key)
        .serialize_compact();

    let ptr_to_region_in_wasm_vm =
        write_to_memory(&mut call_context.runtime, &sig).map_err(|err| {
            debug!(
            "secp256k1_sign() error while trying to allocate and write the sig {:?} to the WASM VM",
            &sig,
        );
            err
        })?;

    // Return pointer to the allocated buffer with the value written to it
    Ok(to_low_half(ptr_to_region_in_wasm_vm) as i64)
}

fn host_ed25519_sign(
    context: &mut Context,
    mut call_context: wasm3::CallContext,
    (message_ptr, private_key_ptr): (i32, i32),
) -> WasmEngineResult<i64> {
    let memory = CWMemory::new(&mut call_context.runtime);

    let cost = context.gas_costs.external_ed25519_sign as u64;
    context.use_gas_externally(cost)?;

    let message_data = memory.extract_vector(message_ptr as u32).map_err(
        debug_err!(err => "ed25519_sign error while trying to read message_hash from wasm memory: {err}")
    )?;
    let private_key_data = memory.extract_vector(private_key_ptr as u32).map_err(
        debug_err!(err => "ed25519_sign error while trying to read private key from wasm memory: {err}")
    )?;

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
        return Ok(to_high_half(WasmApiCryptoError::InvalidPrivateKeyFormat as u32) as i64);
    }

    let ed25519_signing_key = match ed25519_zebra::SigningKey::try_from(private_key_data.as_slice())
    {
        Ok(x) => x,
        Err(err) => {
            debug!(
                "ed25519_sign() failed to create an ed25519 signing key from private_key: {:?}",
                err
            );

            return Ok(to_high_half(WasmApiCryptoError::InvalidPrivateKeyFormat as u32) as i64);
        }
    };

    let sig: [u8; 64] = ed25519_signing_key.sign(message_data.as_slice()).into();

    let ptr_to_region_in_wasm_vm = write_to_memory(&mut call_context.runtime, &sig).map_err(|err| {
        debug!(
                "ed25519_sign() error while trying to allocate and write the sig {:?} to the WASM VM",
                &sig,
            );
        err
    })?;

    // Return pointer to the allocated buffer with the value written to it
    Ok(to_low_half(ptr_to_region_in_wasm_vm) as i64)
}
