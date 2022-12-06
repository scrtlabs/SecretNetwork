use std::convert::{TryFrom, TryInto};

use log::*;

use bech32::{FromBase32, ToBase32};
use cw_types_generic::{CosmWasmApiVersion, CwEnv};
use rand_chacha::ChaChaRng;
use rand_core::SeedableRng;
use wasm3::{Instance, Memory, Trap};

use cw_types_v010::consts::BECH32_PREFIX_ACC_ADDR;
use enclave_cosmos_types::types::{ContractCode, HandleType};
use enclave_crypto::{sha_256, Ed25519PublicKey, WasmApiCryptoError};
use enclave_ffi_types::{Ctx, EnclaveError};

use crate::contract_validation::ContractKey;
#[cfg(not(feature = "query-only"))]
use crate::db::encrypt_key;

use crate::cosmwasm_config::ContractOperation;
use crate::db::read_encrypted_key;
#[cfg(not(feature = "query-only"))]
use crate::db::{remove_encrypted_key, write_multiple_keys};
use crate::errors::{ToEnclaveError, ToEnclaveResult, WasmEngineError, WasmEngineResult};
use crate::gas::{WasmCosts, READ_BASE_GAS, WRITE_BASE_GAS};
use crate::query_chain::encrypt_and_query_chain;
use crate::types::IoNonce;

use gas::{get_exhausted_amount, get_remaining_gas, use_gas};
use module_cache::create_module_instance;

mod gas;
pub mod module_cache;
mod validation;
// use std::time::Instant;

type Wasm3RsError = wasm3::Error;
type Wasm3RsResult<T> = Result<T, wasm3::Error>;

use enclave_utils::kv_cache::KvCache;

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

trait Wasm3ResultEx {
    fn allow_missing_import(self) -> Self;
}

impl Wasm3ResultEx for Wasm3RsResult<()> {
    fn allow_missing_import(self) -> Self {
        match self {
            Err(Wasm3RsError::FunctionNotFound) => Ok(()),
            // TODO check how this looks like in oasis's version
            // Workaround for erroneous non-enumerated error in this case in wasm3.
            // Search for the string "function signature mismatch" in the C source
            // Err(Wasm3RsError::Wasm3(wasm3_error)) if Trap::from(wasm3_error) == Trap::Abort => {
            //     Err(Wasm3RsError::InvalidFunctionSignature)
            // }
            other => other,
        }
    }
}

trait Wasm3RuntimeEx {
    fn try_with_memory_or<F, R, E>(&self, error: E, f: F) -> Result<R, E>
    where
        F: FnOnce(wasm3::Memory<'_>) -> R;
}

impl<'env, C> Wasm3RuntimeEx for wasm3::Runtime<'env, C> {
    fn try_with_memory_or<F, R, E>(&self, error: E, f: F) -> Result<R, E>
    where
        F: FnOnce(Memory<'_>) -> R,
    {
        self.try_with_memory(f).map_err(|_err| error)
    }
}

pub struct Context {
    context: Ctx,
    gas_limit: u64,
    gas_used_externally: u64,
    gas_costs: WasmCosts,
    query_depth: u32,
    #[cfg_attr(feature = "query-only", allow(unused))]
    operation: ContractOperation,
    contract_key: ContractKey,
    user_nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
    kv_cache: KvCache,
    last_error: Option<WasmEngineError>,
}

impl Context {
    pub fn use_gas_externally(&mut self, amount: u64) {
        self.gas_used_externally = self.gas_used_externally.saturating_add(amount);
    }

    pub fn get_gas_used_externally(&self) -> u64 {
        self.gas_used_externally
    }

    pub fn take_last_error(&mut self) -> Option<WasmEngineError> {
        self.last_error.take()
    }

    pub fn set_last_error(&mut self, error: WasmEngineError) {
        self.last_error = Some(error);
    }
}

/// Wrap the hook function such that we expect the context to be passed in,
/// and we save the WasmEngineError in the Context.
fn expect_context<F, A, R>(
    mut func: F,
) -> impl FnMut(wasm3::CallContext<Context>, A) -> Result<R, Trap> + 'static
where
    F: FnMut(&mut Context, &wasm3::Instance<Context>, A) -> Result<R, WasmEngineError> + 'static,
    A: wasm3::Arg,
    R: wasm3::Arg,
{
    move |call_context, input| {
        let err_msg = "module functions must be called with a context";
        let context = call_context.context.expect(err_msg);
        let instance = call_context.instance;
        func(context, instance, input).map_err(|err| {
            context.set_last_error(err);
            wasm3::Trap::Abort
        })
    }
}

fn link_fn<F, A, R>(instance: &mut Instance<Context>, name: &str, func: F) -> Wasm3RsResult<()>
where
    F: FnMut(&mut Context, &wasm3::Instance<Context>, A) -> Result<R, WasmEngineError> + 'static,
    A: wasm3::Arg + 'static,
    R: wasm3::Arg + 'static,
{
    let func = expect_context(func);
    instance
        .link_function("env", name, func)
        .allow_missing_import()
}

fn check_execution_result<T>(
    instance: &Instance<Context>,
    context: &mut Context,
    result: Result<T, wasm3::Error>,
) -> Result<T, EnclaveError> {
    result.map_err(|err| match err {
        // If Unreachable was executed, and "exhausted" isn't 0, that means we ran out of gas.
        wasm3::Error::UnreachableExecuted if get_exhausted_amount(instance) != 0 => {
            debug!(
                "Detected out of gas! Limit: {}, Remaining: {}, Exhausted: {}",
                context.gas_limit,
                get_remaining_gas(instance),
                get_exhausted_amount(instance)
            );
            EnclaveError::OutOfGas
        }
        // Otherwise, check if a hook set an error, in which case we propagate it.
        err => match context.take_last_error() {
            Some(err) => err.into(),
            None => err.to_enclave_error(),
        },
    })
}

pub struct Engine {
    context: Context,
    gas_limit: u64,
    used_gas: u64,
    environment: wasm3::Environment,
    code: Vec<u8>,
    api_version: CosmWasmApiVersion,
}

impl Engine {
    #[allow(clippy::too_many_arguments)]
    pub fn new(
        context: Ctx,
        gas_limit: u64,
        gas_costs: WasmCosts,
        contract_code: &ContractCode,
        contract_key: ContractKey,
        operation: ContractOperation,
        user_nonce: IoNonce,
        user_public_key: Ed25519PublicKey,
        query_depth: u32,
    ) -> Result<Engine, EnclaveError> {
        let versioned_code = create_module_instance(contract_code, &gas_costs, operation)?;
        let kv_cache = KvCache::new();
        let context = Context {
            context,
            query_depth,
            gas_limit,
            gas_used_externally: 0,
            gas_costs,
            operation,
            contract_key,
            user_nonce,
            user_public_key,
            kv_cache,
            last_error: None,
        };

        debug!("setting up runtime");
        // let start = Instant::now();

        let environment = wasm3::Environment::new().to_enclave_result()?;
        // let duration = start.elapsed();
        // trace!("Time elapsed in Environment::new() is: {:?}", duration);
        debug!("initialized environment");

        Ok(Self {
            context,
            gas_limit,
            used_gas: 0,
            environment,
            code: versioned_code.code,
            api_version: versioned_code.version,
        })
    }

    fn with_instance<F>(&mut self, func: F) -> Result<Vec<u8>, EnclaveError>
    where
        F: FnOnce(&mut wasm3::Instance<Context>, &mut Context) -> Result<Vec<u8>, EnclaveError>,
    {
        // let start = Instant::now();
        let runtime = self
            .environment
            .new_runtime::<Context>(1024 * 60, Some(192 /* 12 MiB */))
            .to_enclave_result()?;
        // let duration = start.elapsed();
        // trace!("Time elapsed in environment.new_runtime is: {:?}", duration);
        debug!("initialized runtime");

        // let start = Instant::now();
        let module = self
            .environment
            .parse_module(&self.code)
            .to_enclave_result()?;
        // let duration = start.elapsed();
        // trace!(
        // "Time elapsed in environment.parse_module is: {:?}",
        // duration
        // );
        debug!("parsed module");

        // let start = Instant::now();
        let mut instance = runtime.load_module(module).to_enclave_result()?;
        // let duration = start.elapsed();
        // trace!("Time elapsed in runtime.load_module is: {:?}", duration);
        debug!("created instance");

        // let start = Instant::now();
        gas::set_gas_limit(&instance, self.gas_limit)?;
        // let duration = start.elapsed();
        // trace!("Time elapsed in set_gas_limit is: {:?}", duration);
        debug!("set gas limit");

        // let start = Instant::now();
        Self::link_host_functions(&mut instance).to_enclave_result()?;
        // let duration = start.elapsed();
        // trace!("Time elapsed in link_host_functions is: {:?}", duration);
        debug!("linked functions");

        // let start = Instant::now();
        let result = func(&mut instance, &mut self.context);
        // let duration = start.elapsed();
        // trace!("Instance: elapsed time for running func is: {:?}", duration);
        debug!("function returned {:?}", result);

        self.used_gas = self
            .gas_limit
            .saturating_sub(get_remaining_gas(&instance))
            .saturating_sub(self.context.get_gas_used_externally())
            .saturating_add(get_exhausted_amount(&instance));

        result
    }

    fn link_host_functions(instance: &mut wasm3::Instance<Context>) -> Wasm3RsResult<()> {
        link_fn(instance, "db_read", host_read_db)?;
        link_fn(instance, "db_write", host_write_db)?;
        link_fn(instance, "db_remove", host_remove_db)?;
        link_fn(instance, "canonicalize_address", host_canonicalize_address)?;
        link_fn(instance, "humanize_address", host_humanize_address)?;
        link_fn(instance, "query_chain", host_query_chain)?;

        link_fn(instance, "addr_canonicalize", host_addr_canonicalize)?;
        link_fn(instance, "addr_humanize", host_humanize_address)?;
        link_fn(instance, "addr_validate", host_addr_validate)?;
        link_fn(instance, "debug_print", host_debug_print)?;

        link_fn(instance, "debug", host_debug_print)?;

        link_fn(instance, "secp256k1_verify", host_secp256k1_verify)?;
        #[rustfmt::skip]
        link_fn(instance, "secp256k1_recover_pubkey", host_secp256k1_recover_pubkey)?;
        link_fn(instance, "ed25519_verify", host_ed25519_verify)?;
        link_fn(instance, "ed25519_batch_verify", host_ed25519_batch_verify)?;
        link_fn(instance, "secp256k1_sign", host_secp256k1_sign)?;
        link_fn(instance, "ed25519_sign", host_ed25519_sign)?;

        //    DbReadIndex = 0,
        //     DbWriteIndex = 1,
        //     DbRemoveIndex = 2,
        //     CanonicalizeAddressIndex = 3,
        //     HumanizeAddressIndex = 4,
        //     GasIndex = 5,
        //     QueryChainIndex = 6,
        //     AddrValidateIndex = 7,
        //     AddrCanonicalizeIndex = 8,
        //     AddrHumanizeIndex = 9,
        //     Secp256k1VerifyIndex = 10,
        //     Secp256k1RecoverPubkeyIndex = 11,
        //     Ed25519VerifyIndex = 12,
        //     Ed25519BatchVerifyIndex = 13,
        //     Secp256k1SignIndex = 14,
        //     Ed25519SignIndex = 15,
        //     DebugIndex = 16,
        //     DebugPrintIndex = 254,
        //     Unknown,

        Ok(())
    }

    /// get the amount of gas used by the last contract execution
    pub fn gas_used(&self) -> u64 {
        self.used_gas
    }

    pub fn get_api_version(&self) -> CosmWasmApiVersion {
        self.api_version
    }

    pub fn init(&mut self, env: &CwEnv, msg: Vec<u8>) -> Result<Vec<u8>, EnclaveError> {
        let api_version = self.get_api_version();

        self.with_instance(|instance, context| {
            debug!("starting init, api version: {:?}", api_version);

            let (env_bytes, msg_info_bytes) = env.get_wasm_ptrs()?;

            // let start = Instant::now();
            let env_ptr = write_to_memory(instance, &env_bytes)?;
            // let duration = start.elapsed();
            // trace!(
            //     "Time elapsed in env_bytes write_to_memory is: {:?}",
            //     duration
            // );

            // let start = Instant::now();
            let msg_ptr = write_to_memory(instance, &msg)?;
            // let duration = start.elapsed();
            // trace!("Time elapsed in msg write_to_memory is: {:?}", duration);

            let result = match api_version {
                CosmWasmApiVersion::V010 => {
                    let (init, args) = (
                        instance
                            .find_function::<(u32, u32), u32>("init")
                            .to_enclave_result()?,
                        (env_ptr, msg_ptr),
                    );
                    init.call_with_context(context, args)
                }
                CosmWasmApiVersion::V1 => {
                    let msg_info_ptr = write_to_memory(instance, &msg_info_bytes)?;

                    let (init, args) = (
                        instance
                            .find_function::<(u32, u32, u32), u32>("instantiate")
                            .to_enclave_result()?,
                        (env_ptr, msg_info_ptr, msg_ptr),
                    );
                    // let start = Instant::now();
                    // let res =
                    init.call_with_context(context, args)
                    // let duration = start.elapsed();
                    // trace!("Time elapsed in call_with_context is: {:?}", duration);
                    // res
                }
                CosmWasmApiVersion::Invalid => {
                    return Err(EnclaveError::InvalidWasm);
                }
            };
            // let start = Instant::now();
            let output_ptr = check_execution_result(instance, context, result)?;
            // let duration = start.elapsed();
            // trace!("Time elapsed in check_execution_result is: {:?}", duration);

            // let start = Instant::now();
            let output = read_from_memory(instance, output_ptr)?;
            // let duration = start.elapsed();
            // trace!("Time elapsed in read_from_memory is: {:?}", duration);

            Ok(output)
        })
    }

    pub fn handle(
        &mut self,
        env: &CwEnv,
        msg: Vec<u8>,
        handle_type: &HandleType,
    ) -> Result<Vec<u8>, EnclaveError> {
        let api_version = self.get_api_version();

        self.with_instance(|instance, context| {
            debug!("starting handle");
            let (env_bytes, msg_info_bytes) = env.get_wasm_ptrs()?;

            let msg_ptr = write_to_memory(instance, &msg)?;
            debug!("handle written msg");
            let env_ptr = write_to_memory(instance, &env_bytes)?;
            debug!("handle written env");

            let result = match api_version {
                CosmWasmApiVersion::V010 => {
                    let (handle, args) = (
                        instance
                            .find_function::<(u32, u32), u32>("handle")
                            .to_enclave_result()?,
                        (env_ptr, msg_ptr),
                    );
                    handle.call_with_context(context, args)
                }
                CosmWasmApiVersion::V1 => {
                    let export_name = HandleType::get_export_name(handle_type);

                    if handle_type == &HandleType::HANDLE_TYPE_EXECUTE {
                        let msg_info_ptr = write_to_memory(instance, &msg_info_bytes)?;
                        let (handle, args) = (
                            instance
                                .find_function::<(u32, u32, u32), u32>(export_name)
                                .to_enclave_result()?,
                            (env_ptr, msg_info_ptr, msg_ptr),
                        );
                        handle.call_with_context(context, args)
                    } else {
                        let (handle, args) = (
                            instance
                                .find_function::<(u32, u32), u32>(export_name)
                                .to_enclave_result()?,
                            (env_ptr, msg_ptr),
                        );
                        handle.call_with_context(context, args)
                    }
                }
                CosmWasmApiVersion::Invalid => {
                    return Err(EnclaveError::InvalidWasm);
                }
            };

            debug!("found handle");

            let output_ptr = check_execution_result(instance, context, result)?;
            debug!("called handle");

            let output = read_from_memory(instance, output_ptr)?;
            debug!("extracted handle output: {:?}", output);

            Ok(output)
        })
    }

    pub fn query(&mut self, env: &CwEnv, msg: Vec<u8>) -> Result<Vec<u8>, EnclaveError> {
        let api_version = self.get_api_version();

        self.with_instance(|instance, context| {
            let msg_ptr = write_to_memory(instance, &msg)?;

            let result = match api_version {
                CosmWasmApiVersion::V010 => {
                    let (query, args) = (
                        instance
                            .find_function::<u32, u32>("query")
                            .to_enclave_result()?,
                        (msg_ptr),
                    );

                    query.call_with_context(context, args)
                }

                CosmWasmApiVersion::V1 => {
                    let (env_bytes, _) = env.get_wasm_ptrs()?;
                    let env_ptr = write_to_memory(instance, &env_bytes)?;
                    let (query, args) = (
                        instance
                            .find_function::<(u32, u32), u32>("query")
                            .to_enclave_result()?,
                        (env_ptr, msg_ptr),
                    );

                    query.call_with_context(context, args)
                }
                CosmWasmApiVersion::Invalid => {
                    return Err(EnclaveError::InvalidWasm);
                }
            };

            debug!("starting query");

            let output_ptr = check_execution_result(instance, context, result)?;

            let output = read_from_memory(instance, output_ptr)?;

            Ok(output)
        })
    }

    #[cfg(feature = "query-only")]
    pub fn flush_cache(&mut self) -> Result<(), EnclaveError> {
        Ok(())
    }

    #[cfg(not(feature = "query-only"))]
    pub fn flush_cache(&mut self) -> Result<(), EnclaveError> {
        let keys: Vec<(Vec<u8>, Vec<u8>)> = self
            .context
            .kv_cache
            .flush()
            .into_iter()
            .map(|(k, v)| {
                let (enc_key, _, enc_v) =
                    encrypt_key(&k, &v, &self.context.context, &self.context.contract_key).unwrap();

                (enc_key.to_vec(), enc_v)
            })
            // todo: fix
            // .map_err(|_|
            //     {
            //         debug!(
            //         "addr_validate() error while trying to parse human address from bytes to string: {:?}",
            //         err
            //     );
            //         return Ok(Some(RuntimeValue::I32(
            //             self.write_to_memory(b"Input is not valid UTF-8")? as i32,
            //         )));
            //     }
            // )?
            .collect();

        let used_gas = write_multiple_keys(&self.context.context, keys).map_err(|err| {
            debug!(
                "write_db() error while trying to write the value to state: {:?}",
                err
            );

            EnclaveError::from(err)
        })?;

        self.with_instance(|instance, _context| {
            use_gas(instance, used_gas)?;
            Ok(vec![])
        })?;

        Ok(())
    }
}

struct CWMemory<'m> {
    memory: wasm3::Memory<'m>,
}

const SIZE_OF_U32: usize = std::mem::size_of::<u32>();

impl<'m> CWMemory<'m> {
    fn new(memory: wasm3::Memory<'m>) -> Self {
        Self { memory }
    }

    fn get_u32_at(&self, idx: u32) -> WasmEngineResult<u32> {
        let idx = idx as usize;
        let bytes: [u8; SIZE_OF_U32] = self
            .memory
            .as_slice()
            .get(idx..idx + SIZE_OF_U32)
            .ok_or(WasmEngineError::MemoryReadError)?
            .try_into()
            .map_err(|_| WasmEngineError::MemoryReadError)?;
        Ok(u32::from_le_bytes(bytes))
    }

    fn set_u32_at(&mut self, idx: u32, val: u32) -> WasmEngineResult<u32> {
        let i = idx as usize;
        self.memory
            .as_slice_mut()
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

        match self.memory.as_slice().get(vec_ptr..vec_ptr + vec_len) {
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

        let data = self.memory.as_slice().get(data_ptr..data_ptr + data_len);
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
            .as_slice_mut()
            .get_mut(idx..idx + buffer.len())
            .ok_or(WasmEngineError::MemoryReadError)?
            .copy_from_slice(buffer);
        self.set_u32_at(region_ptr + (SIZE_OF_U32 * 2) as u32, buffer.len() as u32)?;

        Ok(region_ptr)
    }
}

fn read_from_memory<C>(
    instance: &wasm3::Instance<C>,
    region_ptr: u32,
) -> WasmEngineResult<Vec<u8>> {
    // let start = Instant::now();
    let runtime = instance.runtime();
    // let duration = start.elapsed();
    // trace!(
    //     "read_from_memory: Time elapsed in instance.runtime(): {:?}",
    //     duration
    // );

    // let start = Instant::now();
    // let res =
    runtime.try_with_memory_or(WasmEngineError::MemoryReadError, |memory| {
        CWMemory::new(memory).extract_vector(region_ptr)
    })?
    // let duration = start.elapsed();
    // trace!(
    //     "read_from_memory: Time elapsed in runtime.try_with_memory_or(): {:?}",
    //     duration
    // );

    // res
}

fn decode_sections_from_memory<C>(
    instance: &wasm3::Instance<C>,
    region_ptr: u32,
) -> WasmEngineResult<Vec<Vec<u8>>> {
    let runtime = instance.runtime();
    runtime.try_with_memory_or(WasmEngineError::MemoryReadError, |memory| {
        CWMemory::new(memory).decode_sections(region_ptr)
    })?
}

fn write_to_memory<C>(instance: &wasm3::Instance<C>, buffer: &[u8]) -> WasmEngineResult<u32> {
    // let start = Instant::now();
    let region_ptr = (|| {
        let alloc_fn = instance.find_function::<u32, u32>("allocate")?;
        alloc_fn.call(buffer.len() as u32)
    })()
    .map_err(debug_err!(err => "failed to allocate {} bytes in contract: {err}", buffer.len()))
    .map_err(|_| WasmEngineError::MemoryAllocationError)?;
    // let duration = start.elapsed();
    // trace!(
    //     "write_to_memory: Time elapsed in allocate function call: {:?}",
    //     duration
    // );

    // let start = Instant::now();
    // let res =
    write_to_allocated_memory(instance, region_ptr, buffer)
    // let duration = start.elapsed();
    // trace!(
    //     "write_to_memory: Time elapsed in write_to_allocated_memory: {:?}",
    //     duration
    // );

    //res
}

fn write_to_allocated_memory<C>(
    instance: &wasm3::Instance<C>,
    region_ptr: u32,
    buffer: &[u8],
) -> WasmEngineResult<u32> {
    instance
        .runtime()
        .try_with_memory_or(WasmEngineError::MemoryWriteError, |memory| {
            CWMemory::new(memory)
                .write_to_allocated_memory(region_ptr, buffer)
                .map_err(debug_err!(err => "failed to write to contract memory {err}"))
        })?
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
    instance: &wasm3::Instance<Context>,
    state_key_region_ptr: i32,
) -> WasmEngineResult<i32> {
    // todo: time this
    use_gas(instance, READ_BASE_GAS)?;

    let state_key_name = read_from_memory(instance, state_key_region_ptr as u32).map_err(
        debug_err!(err => "db_read failed to extract vector from state_key_region_ptr: {err}"),
    )?;

    debug!("db_read reading key {}", show_bytes(&state_key_name));

    let value = context.kv_cache.read(&state_key_name);

    if let Some(unwrapped) = value {
        debug!("Got value from cache");
        let ptr_to_region_in_wasm_vm = write_to_memory(instance, &unwrapped).map_err(|err| {
            debug!(
                "read_db() error while trying to allocate {} bytes for the value",
                unwrapped.len(),
            );
            err
        })?;

        return Ok(ptr_to_region_in_wasm_vm as i32);
    }

    debug!("Missed value in cache");
    let (value, used_gas) = read_encrypted_key(
        &state_key_name,
        &context.context,
        &context.contract_key,
        &mut context.kv_cache,
    )
    .map_err(debug_err!("db_read failed to read key from storage"))?;
    context.use_gas_externally(used_gas);

    debug!(
        "db_read received value {:?}",
        value.as_ref().map(|v| show_bytes(v))
    );

    let value = match value {
        // Return 0 (null ponter) if value is empty
        Some(value) => value,
        None => return Ok(0),
    };

    let region_ptr = write_to_memory(instance, &value)?;

    Ok(region_ptr as i32)
}

#[cfg(not(feature = "query-only"))]
fn host_remove_db(
    context: &mut Context,
    instance: &wasm3::Instance<Context>,
    state_key_region_ptr: i32,
) -> WasmEngineResult<()> {
    if context.operation.is_query() {
        debug!("db_remove was called while in query mode");
        return Err(WasmEngineError::UnauthorizedWrite);
    }

    let state_key_name = read_from_memory(instance, state_key_region_ptr as u32).map_err(
        debug_err!(err => "db_remove failed to extract vector from state_key_region_ptr: {err}"),
    )?;

    debug!("db_remove removing key {}", show_bytes(&state_key_name));

    let used_gas = remove_encrypted_key(&state_key_name, &context.context, &context.contract_key)?;
    context.use_gas_externally(used_gas);

    Ok(())
}

#[cfg(not(feature = "query-only"))]
fn host_write_db(
    context: &mut Context,
    instance: &wasm3::Instance<Context>,
    (state_key_region_ptr, value_region_ptr): (i32, i32),
) -> WasmEngineResult<()> {
    if context.operation.is_query() {
        debug!("db_write was called while in query mode");
        return Err(WasmEngineError::UnauthorizedWrite);
    }

    use_gas(instance, WRITE_BASE_GAS)?;

    // let start = Instant::now();
    let state_key_name = read_from_memory(instance, state_key_region_ptr as u32).map_err(
        debug_err!(err => "db_write failed to extract vector from state_key_region_ptr: {err}"),
    )?;
    let value = read_from_memory(instance, value_region_ptr as u32).map_err(
        debug_err!(err => "db_write failed to extract vector from value_region_ptr: {err}"),
    )?;
    // let duration = start.elapsed();
    // trace!(
    //     "host_write_db: Time elapsed in read_from_memory x2: {:?}",
    //     duration
    // );

    debug!(
        "db_write writing key: {}, value: {}",
        show_bytes(&state_key_name),
        show_bytes(&value)
    );

    context.kv_cache.write(&state_key_name, &value);

    // let used_gas = write_encrypted_key(
    //     &state_key_name,
    //     &value,
    //     &context.context,
    //     &context.contract_key,
    // )
    // .map_err(debug_err!("db_write failed to write key to storage",))?;
    // use_gas(instance, used_gas)?;

    Ok(())
}

#[cfg(feature = "query-only")]
fn host_remove_db(
    _context: &mut Context,
    _instance: &wasm3::Instance<Context>,
    _state_key_region_ptr: i32,
) -> WasmEngineResult<()> {
    Err(WasmEngineError::UnauthorizedWrite)
}

#[cfg(feature = "query-only")]
fn host_write_db(
    _context: &mut Context,
    _instance: &wasm3::Instance<Context>,
    (_state_key_region_ptr, _value_region_ptr): (i32, i32),
) -> WasmEngineResult<()> {
    Err(WasmEngineError::UnauthorizedWrite)
}

fn host_canonicalize_address(
    context: &mut Context,
    instance: &wasm3::Instance<Context>,
    (human_region_ptr, canonical_region_ptr): (i32, i32),
) -> WasmEngineResult<i32> {
    let used_gas = context.gas_costs.external_canonicalize_address as u64;
    use_gas(instance, used_gas)?;

    let human = read_from_memory(instance, human_region_ptr as u32)
        .map_err(debug_err!(err => "canonicalize_address failed to extract vector from human_region_ptr: {err}"))?;

    let mut human_addr_str = match std::str::from_utf8(&human) {
        Ok(addr) => addr,
        Err(_err) => {
            debug!(
                "canonicalize_address input was not valid UTF-8: {}",
                show_bytes(&human)
            );
            return write_to_memory(instance, b"input is not valid UTF-8")
                .map(|n| n as i32)
                .map_err(debug_err!("failed to write error message to contract"));
        }
    };
    human_addr_str = human_addr_str.trim();
    if human_addr_str.is_empty() {
        debug!("canonicalize_address input was empty");
        return write_to_memory(instance, b"input is empty")
            .map(|n| n as i32)
            .map_err(debug_err!("failed to write error message to contract"));
    }

    //debug!("canonicalize_address was called with {:?}", human_addr_str);

    let (decoded_prefix, data) = match bech32::decode(human_addr_str) {
        Ok(ret) => ret,
        Err(err) => {
            debug!(
                "canonicalize_address failed to parse input as bech32: {:?}",
                err
            );
            return write_to_memory(instance, err.to_string().as_bytes())
                .map(|n| n as i32)
                .map_err(debug_err!("failed to write error message to contract"));
        }
    };

    if decoded_prefix != BECH32_PREFIX_ACC_ADDR {
        debug!("canonicalize_address was called with an unexpected address prefix");
        return write_to_memory(
            instance,
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

    write_to_allocated_memory(instance, canonical_region_ptr as u32, &canonical)?;

    // return 0 == ok
    Ok(0)
}

fn host_addr_canonicalize(
    context: &mut Context,
    instance: &wasm3::Instance<Context>,
    (human_region_ptr, canonical_region_ptr): (i32, i32),
) -> WasmEngineResult<i32> {
    let used_gas = context.gas_costs.external_canonicalize_address as u64;
    use_gas(instance, used_gas)?;

    let human = read_from_memory(instance, human_region_ptr as u32)
        .map_err(debug_err!(err => "addr_canonicalize failed to extract vector from human_region_ptr: {err}"))?;

    let human_addr_str = match std::str::from_utf8(&human) {
        Ok(addr) => addr,
        Err(_err) => {
            debug!(
                "addr_canonicalize input was not valid UTF-8: {}",
                show_bytes(&human)
            );
            return write_to_memory(instance, b"input is not valid UTF-8")
                .map(|n| n as i32)
                .map_err(debug_err!("failed to write error message to contract"));
        }
    };
    if human_addr_str.is_empty() {
        debug!("addr_canonicalize input was empty");
        return write_to_memory(instance, b"Input is empty")
            .map(|n| n as i32)
            .map_err(debug_err!("failed to write error message to contract"));
    }

    debug!("addr_canonicalize was called with {:?}", human_addr_str);

    let (decoded_prefix, data) = match bech32::decode(human_addr_str) {
        Ok(ret) => ret,
        Err(err) => {
            debug!(
                "addr_canonicalize failed to parse input as bech32: {:?}",
                err
            );
            return write_to_memory(instance, err.to_string().as_bytes())
                .map(|n| n as i32)
                .map_err(debug_err!("failed to write error message to contract"));
        }
    };

    if decoded_prefix != BECH32_PREFIX_ACC_ADDR {
        debug!("addr_canonicalize was called with an unexpected address prefix");
        return write_to_memory(
            instance,
            format!("wrong address prefix: {:?}", decoded_prefix).as_bytes(),
        )
        .map(|n| n as i32)
        .map_err(debug_err!("failed to write error message to contract"));
    }

    let canonical = Vec::<u8>::from_base32(&data).map_err(|err| {
        // Assaf: From reading https://docs.rs/bech32/0.7.2/src/bech32/lib.rs.html#607
        // and https://docs.rs/bech32/0.7.2/src/bech32/lib.rs.html#228 I don't think this can fail that way
        debug!("addr_canonicalize failed to parse base32: {}", err);
        WasmEngineError::Base32Error
    })?;

    debug!(
        "addr_canonicalize returning address {}",
        hex::encode(human_addr_str)
    );

    write_to_allocated_memory(instance, canonical_region_ptr as u32, &canonical)?;

    // return 0 == ok
    Ok(0)
}

fn host_addr_validate(
    context: &mut Context,
    instance: &wasm3::Instance<Context>,
    (addr_to_validate,): (i32,),
) -> WasmEngineResult<i32> {
    let used_gas = context.gas_costs.external_addr_validate as u64;
    use_gas(instance, used_gas)?;

    let human = read_from_memory(instance, addr_to_validate as u32)
        .map_err(debug_err!(err => "humanize_address failed to extract vector from canonical_region_ptr: {err}"))?;

    trace!(
        "addr_validate() was called from WASM code with {:?}",
        String::from_utf8_lossy(&human)
    );

    if human.is_empty() {
        return write_to_memory(instance, b"Input is empty").map(|n| n as i32);
    }

    // Turn Vec<u8> to str
    let source_human_address = match std::str::from_utf8(&human) {
        Err(err) => {
            debug!(
                    "addr_validate() error while trying to parse human address from bytes to string: {:?}",
                    err
                );
            return write_to_memory(instance, b"Input is not valid UTF-8").map(|n| n as i32);
        }
        Ok(x) => x,
    };

    let canonical_address = match bech32::decode(source_human_address) {
        Err(err) => {
            debug!(
                "addr_validate() error while trying to decode human address {:?} as bech32: {:?}",
                source_human_address, err
            );
            return write_to_memory(instance, err.to_string().as_bytes()).map(|n| n as i32);
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
            return write_to_memory(instance, err.to_string().as_bytes()).map(|n| n as i32);
        }
        Ok(normalized_human_address) => normalized_human_address,
    };

    if source_human_address != normalized_human_address {
        return write_to_memory(instance, b"Address is not normalized").map(|n| n as i32);
    }

    Ok(0)
}

fn host_humanize_address(
    context: &mut Context,
    instance: &wasm3::Instance<Context>,
    (canonical_region_ptr, human_region_ptr): (i32, i32),
) -> WasmEngineResult<i32> {
    let used_gas = context.gas_costs.external_humanize_address as u64;
    use_gas(instance, used_gas)?;

    let canonical = read_from_memory(instance, canonical_region_ptr as u32)
        .map_err(debug_err!(err => "humanize_address failed to extract vector from canonical_region_ptr: {err}"))?;

    debug!(
        "humanize_address was called with {}",
        hex::encode(&canonical)
    );

    let human_addr_str = match bech32::encode(BECH32_PREFIX_ACC_ADDR, canonical.to_base32()) {
        Ok(addr) => addr,
        Err(err) => {
            debug!("humanize_address failed to encode address as bech32");
            return write_to_memory(instance, err.to_string().as_bytes())
                .map(|n| n as i32)
                .map_err(debug_err!("failed to write error message to contract"));
        }
    };

    debug!("humanize_address returning address {}", human_addr_str);

    let human_bytes = human_addr_str.into_bytes();

    write_to_allocated_memory(instance, human_region_ptr as u32, &human_bytes)?;

    // return 0 == ok
    Ok(0)
}

fn host_query_chain(
    context: &mut Context,
    instance: &wasm3::Instance<Context>,
    query_region_ptr: i32,
) -> WasmEngineResult<i32> {
    let query_buffer = read_from_memory(instance, query_region_ptr as u32).map_err(
        debug_err!(err => "query_chain failed to extract vector from query_region_ptr: {err}"),
    )?;

    let mut used_gas: u64 = 0;
    let answer = encrypt_and_query_chain(
        &query_buffer,
        context.query_depth,
        &context.context,
        context.user_nonce,
        context.user_public_key,
        &mut used_gas,
        get_remaining_gas(instance),
    )?;

    context.use_gas_externally(used_gas);

    write_to_memory(instance, &answer).map(|region_ptr| region_ptr as i32)
}

fn host_debug_print(
    _context: &mut Context,
    instance: &wasm3::Instance<Context>,
    message_region_ptr: i32,
) -> WasmEngineResult<()> {
    let message_buffer = read_from_memory(instance, message_region_ptr as u32)?;
    let message =
        String::from_utf8(message_buffer).unwrap_or_else(|err| hex::encode(err.into_bytes()));

    info!("debug_print: {:?}", message);

    Ok(())
}

fn host_secp256k1_verify(
    context: &mut Context,
    instance: &wasm3::Instance<Context>,
    (message_hash_ptr, signature_ptr, public_key_ptr): (i32, i32, i32),
) -> WasmEngineResult<i32> {
    let used_gas = context.gas_costs.external_secp256k1_verify as u64;
    use_gas(instance, used_gas)?;

    let message_hash_data = read_from_memory(instance, message_hash_ptr as u32)
        .map_err(debug_err!(err => "secp256k1_verify error while trying to read message_hash from wasm memory: {err}"))?;
    let signature_data = read_from_memory(instance, signature_ptr as u32)
        .map_err(debug_err!(err => "secp256k1_verify error while trying to read signature from wasm memory: {err}"))?;
    let public_key = read_from_memory(instance, public_key_ptr as u32)
        .map_err(debug_err!(err => "secp256k1_verify error while trying to read public_key from wasm memory: {err}"))?;

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
    instance: &wasm3::Instance<Context>,
    (message_hash_ptr, signature_ptr, recovery_param): (i32, i32, i32),
) -> WasmEngineResult<i64> {
    let used_gas = context.gas_costs.external_secp256k1_recover_pubkey as u64;
    use_gas(instance, used_gas)?;

    let message_hash_data = read_from_memory(instance, message_hash_ptr as u32)
        .map_err(debug_err!(err => "secp256k1_recover_pubkey error while trying to read message_hash from wasm memory: {err}"))?;
    let signature_data = read_from_memory(instance, signature_ptr as u32)
        .map_err(debug_err!(err => "secp256k1_recover_pubkey error while trying to read signature from wasm memory: {err}"))?;

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
            let ptr_to_region_in_wasm_vm = write_to_memory(instance, &answer).map_err(|err| {
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
    instance: &wasm3::Instance<Context>,
    (message_ptr, signature_ptr, public_key_ptr): (i32, i32, i32),
) -> WasmEngineResult<i32> {
    let used_gas = context.gas_costs.external_ed25519_verify as u64;
    use_gas(instance, used_gas)?;

    let message_data = read_from_memory(instance, message_ptr as u32)
        .map_err(debug_err!(err => "ed25519_verify error while trying to read message_hash from wasm memory: {err}"))?;
    let signature_data = read_from_memory(instance, signature_ptr as u32)
        .map_err(debug_err!(err => "ed25519_verify error while trying to read signature from wasm memory: {err}"))?;
    let public_key_data = read_from_memory(instance, public_key_ptr as u32)
        .map_err(debug_err!(err => "ed25519_verify error while trying to read public_key from wasm memory: {err}"))?;

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
    instance: &wasm3::Instance<Context>,
    (messages_ptr, signatures_ptr, public_keys_ptr): (i32, i32, i32),
) -> WasmEngineResult<i32> {
    let messages_data = decode_sections_from_memory(instance, messages_ptr as u32)
        .map_err(debug_err!(err => "ed25519_batch_verify error while trying to read messages from wasm memory: {err}"))?;

    let signatures_data = decode_sections_from_memory(instance, signatures_ptr as u32)
        .map_err(debug_err!(err => "ed25519_batch_verify error while trying to read signatures from wasm memory: {err}"))?;

    let pubkeys_data = decode_sections_from_memory(instance, public_keys_ptr as u32)
        .map_err(debug_err!(err => "ed25519_batch_verify error while trying to read public_keys from wasm memory: {err}"))?;

    let messages_len = messages_data.len();
    let signatures_len = signatures_data.len();
    let pubkeys_len = pubkeys_data.len();

    let lengths = (messages_len, signatures_len, pubkeys_len);

    //todo: fix this
    #[allow(clippy::type_complexity)]
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
    let used_gas = base_cost + (signatures.len() as u64) * each_cost;
    use_gas(instance, used_gas)?;

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
    let remaining_gas = get_remaining_gas(instance);
    let used_gas = context.gas_limit.saturating_sub(remaining_gas);
    rng_entropy.append(&mut used_gas.to_be_bytes().to_vec());

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
    instance: &wasm3::Instance<Context>,
    (message_ptr, private_key_ptr): (i32, i32),
) -> WasmEngineResult<i64> {
    let used_gas = context.gas_costs.external_secp256k1_sign as u64;
    use_gas(instance, used_gas)?;

    let message_data = read_from_memory(instance, message_ptr as u32)
        .map_err(debug_err!(err => "secp256k1_sign error while trying to read message_hash from wasm memory: {err}"))?;
    let private_key_data = read_from_memory(instance, private_key_ptr as u32)
        .map_err(debug_err!(err => "secp256k1_sign error while trying to read private key from wasm memory: {err}"))?;

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

    let ptr_to_region_in_wasm_vm = write_to_memory(instance, &sig).map_err(|err| {
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
    instance: &wasm3::Instance<Context>,
    (message_ptr, private_key_ptr): (i32, i32),
) -> WasmEngineResult<i64> {
    let used_gas = context.gas_costs.external_ed25519_sign as u64;
    use_gas(instance, used_gas)?;

    let message_data = read_from_memory(instance, message_ptr as u32).map_err(
        debug_err!(err => "ed25519_sign error while trying to read message_hash from wasm memory: {err}")
    )?;
    let private_key_data = read_from_memory(instance, private_key_ptr as u32).map_err(
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

    let ptr_to_region_in_wasm_vm = write_to_memory(instance, &sig).map_err(|err| {
        debug!(
            "ed25519_sign() error while trying to allocate and write the sig {:?} to the WASM VM",
            &sig,
        );
        err
    })?;

    // Return pointer to the allocated buffer with the value written to it
    Ok(to_low_half(ptr_to_region_in_wasm_vm) as i64)
}
