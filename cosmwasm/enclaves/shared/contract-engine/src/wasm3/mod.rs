use core::ops::DerefMut;
use std::cell::RefCell;
use std::convert::TryInto;
use std::marker::PhantomData;

use wasm3::error::{Trap, TrappedResult};

use enclave_cosmos_types::types::ContractCode;
use enclave_crypto::Ed25519PublicKey;
use enclave_ffi_types::{Ctx, EnclaveError};

use crate::contract_validation::ContractKey;
use crate::db::read_encrypted_key;
use crate::errors::{wasm3_error_to_enclave_error, WasmEngineError, WasmEngineResult};
use crate::gas::WasmCosts;
use crate::types::IoNonce;
use crate::wasm::ContractOperation;

type Wasm3RsError = wasm3::error::Error;
type Wasm3RsResult<T> = wasm3::error::Result<T>;

trait Wasm3ResultEx {
    fn allow_missing_import(self) -> Self;
}

impl Wasm3ResultEx for Wasm3RsResult<()> {
    fn allow_missing_import(self) -> Self {
        match self {
            Err(wasm3::error::Error::FunctionNotFound) => Ok(()),
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

    fn use_gas_externally(&mut self, gas: u64) -> TrappedResult<()> {
        // todo implement gas consumption
        Ok(())
    }
}

macro_rules! link_fn {
    ($module: expr, $context_ptr: expr, $name: expr, $implementation: expr) => {
        $module
            .link_closure("env", $name, move |call_context, args| {
                $implementation($context_ptr, call_context, args)
            })
            .allow_missing_import()
    };
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

        Engine::setup_runtime(context_ptr, contract_code).map_err(|err| unsafe {
            let context = &*context_ptr;
            let mut context = context.borrow_mut();
            wasm3_error_to_enclave_error(context.deref_mut(), err)
        })
    }

    fn setup_runtime(
        context_ptr: *mut RefCell<Context>,
        contract_code: ContractCode,
    ) -> Wasm3RsResult<Engine> {
        let environment = wasm3::Environment::new()?;
        let runtime = environment.create_runtime(1024 * 60)?;

        let mut module = runtime.parse_and_load_module(contract_code.code())?;

        // TODO link to implementations
        link_fn!(module, context_ptr, "db_read", host_read_db)?;
        link_fn!(module, context_ptr, "db_read", host_read_db)?;
        link_fn!(module, context_ptr, "db_read", host_read_db)?;
        link_fn!(module, context_ptr, "db_read", host_read_db)?;

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
        let context = unsafe { &*self.context };
        let context = context.borrow();
        context.gas_used
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
            let context = &*self.context;
            let mut context = context.borrow_mut();
            wasm3_error_to_enclave_error(context.deref_mut(), err)
        };

        self.init_fn()
            .map_err(handle_wasm3_err)?
            .call(env_ptr, msg_ptr)
            .map_err(handle_wasm3_err)
    }

    pub fn handle(&mut self, env_ptr: u32, msg_ptr: u32) -> Result<u32, EnclaveError> {
        let handle_wasm3_err = |err| unsafe {
            let context = &*self.context;
            let mut context = context.borrow_mut();
            wasm3_error_to_enclave_error(context.deref_mut(), err)
        };

        self.handle_fn()
            .map_err(handle_wasm3_err)?
            .call(env_ptr, msg_ptr)
            .map_err(handle_wasm3_err)
    }

    pub fn query(&mut self, msg_ptr: u32) -> Result<u32, EnclaveError> {
        let handle_wasm3_err = |err| unsafe {
            let context = &*self.context;
            let mut context = context.borrow_mut();
            wasm3_error_to_enclave_error(context.deref_mut(), err)
        };

        self.query_fn()
            .map_err(handle_wasm3_err)?
            .call(msg_ptr)
            .map_err(handle_wasm3_err)
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
            eprintln!("vec_ptr_ptr is null");
            return Err(WasmEngineError::MemoryReadError);
        }

        let vec_ptr = self.get_u32_at(region_ptr)? as usize;
        let vec_len = self.get_u32_at(region_ptr + SIZE_OF_U32 as u32)? as usize;
        if vec_ptr == 0 {
            return Err(WasmEngineError::MemoryReadError);
        }

        match self.memory.get(vec_ptr..vec_ptr + vec_len) {
            Some(slice) => Ok(slice.to_owned()),
            None => Err(WasmEngineError::MemoryReadError),
        }
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
        let alloc_fn = runtime.find_function::<u32, u32>("alloc")?;
        alloc_fn.call(buffer.len() as u32)
    })()
    .map_err(|_| WasmEngineError::MemoryAllocationError)?;
    let mut memory = CWMemory::new(runtime);
    memory.write_to_allocated_memory(region_ptr, buffer)?;
    Ok(region_ptr)
}

macro_rules! set_last_error {
    ($context: expr) => {
        |err| {
            $context.set_last_error(err);
            wasm3::error::Trap::Exit
        }
    };
}

fn host_read_db(
    context: *mut RefCell<Context>,
    mut call_context: wasm3::CallContext,
    state_key_region_ptr: u32,
) -> TrappedResult<u32> {
    let context = unsafe { &*context };
    let mut context = context.borrow_mut();

    let memory = CWMemory::new(&mut call_context.runtime);

    let state_key_name = memory
        .extract_vector(state_key_region_ptr)
        .map_err(set_last_error!(context))?;

    let (value, gas_used) =
        read_encrypted_key(&state_key_name, &context.context, &context.contract_key)
            .map_err(set_last_error!(context))?;
    context.use_gas_externally(gas_used)?;

    let value = match value {
        // Return 0 (null ponter) if value is empty
        Some(value) => value,
        None => return Ok(0),
    };

    let region_ptr =
        write_to_memory(&mut call_context.runtime, &value).map_err(set_last_error!(context))?;

    Ok(region_ptr)
}
