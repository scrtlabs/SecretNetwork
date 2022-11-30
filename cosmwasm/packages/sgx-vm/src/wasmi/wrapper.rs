//! This module provides safe wrappers for the calls into the enclave running WASMI.

use std::ffi::c_void;
use std::marker::PhantomData;
use std::mem::MaybeUninit;

use crate::enclave::ENCLAVE_DOORBELL;
// #[cfg(feature = "query-node")]
// use crate::enclave::QUERY_ENCLAVE_DOORBELL;
use crate::errors::{EnclaveError, VmResult};
use crate::{Querier, Storage, VmError};

use enclave_ffi_types::{Ctx, GenerateRandomResult, HandleResult, InitResult, QueryResult};

use sgx_types::sgx_status_t;

use log::*;
use serde::Deserialize;

use super::exports::FullContext;
use super::imports;
use super::results::{
    handle_result_to_vm_result, init_result_to_vm_result, query_result_to_vm_result,
    HandleSuccess, InitSuccess, QuerySuccess,
};

pub struct Module<S, Q>
where
    S: Storage,
    Q: Querier,
{
    bytecode: Vec<u8>,
    gas_limit: u64,
    used_gas: u64,
    ctx: Ctx,
    finalizer: fn(*mut c_void),

    // This does not store data but only fixes type information
    type_storage: PhantomData<S>,
    type_querier: PhantomData<Q>,
}

impl<S, Q> Module<S, Q>
where
    S: Storage,
    Q: Querier,
{
    pub fn new(
        bytecode: Vec<u8>,
        gas_limit: u64,
        (data, finalizer): (*mut c_void, fn(*mut c_void)),
    ) -> Self {
        // TODO add validation of this bytecode?

        let data =
            Box::leak(Box::new(FullContext::new::<S, Q>(data))) as *mut FullContext as *mut c_void;
        let ctx = Ctx { data };
        Self {
            bytecode,
            gas_limit,
            used_gas: 0,
            ctx,
            finalizer,
            type_storage: Default::default(),
            type_querier: Default::default(),
        }
    }

    #[allow(unused)]
    pub fn context(&self) -> &Ctx {
        &self.ctx
    }

    pub fn context_mut(&mut self) -> &mut Ctx {
        &mut self.ctx
    }

    pub fn gas_limit(&self) -> u64 {
        self.gas_limit
    }

    pub fn gas_left(&self) -> u64 {
        self.gas_limit.saturating_sub(self.used_gas)
    }

    pub fn gas_used(&self) -> u64 {
        self.used_gas
    }

    // This is here to avoid putting it in the module's scope
    fn busy_enclave_err() -> VmError {
        VmError::generic_err("The enclave is too busy and can not respond to this query")
    }

    pub fn init(&mut self, env: &[u8], msg: &[u8], sig_info: &[u8]) -> VmResult<InitSuccess> {
        trace!(
            "init() called with env: {:?} msg: {:?} gas_left: {}",
            String::from_utf8_lossy(env),
            String::from_utf8_lossy(msg),
            self.gas_left()
        );

        let mut init_result = MaybeUninit::<InitResult>::uninit();
        let mut used_gas = 0_u64;

        // Bind the token to a local variable to ensure its
        // destructor runs in the end of the function
        let enclave_access_token = ENCLAVE_DOORBELL
            .get_access(1) // This can never be recursive
            .ok_or_else(Self::busy_enclave_err)?;

        let enclave = enclave_access_token.map_err(EnclaveError::sdk_err)?;

        let status = unsafe {
            imports::ecall_init(
                enclave.geteid(),
                init_result.as_mut_ptr(),
                self.ctx.unsafe_clone(),
                self.gas_left(),
                &mut used_gas,
                self.bytecode.as_ptr(),
                self.bytecode.len(),
                env.as_ptr(),
                env.len(),
                msg.as_ptr(),
                msg.len(),
                sig_info.as_ptr(),
                sig_info.len(),
            )
        };

        trace!(
            "init() returned with gas_used: {} (gas_limit: {})",
            used_gas,
            self.gas_limit
        );
        self.consume_gas(used_gas);

        match status {
            sgx_status_t::SGX_SUCCESS => {
                let init_result = unsafe { init_result.assume_init() };
                init_result_to_vm_result(init_result)
            }
            failure_status => Err(EnclaveError::sdk_err(failure_status).into()),
        }
    }

    pub fn handle(
        &mut self,
        env: &[u8],
        msg: &[u8],
        sig_info: &[u8],
        handle_type: u8,
    ) -> VmResult<HandleSuccess> {
        trace!(
            "handle() called with env: {:?} msg: {:?} gas_left: {}",
            String::from_utf8_lossy(env),
            String::from_utf8_lossy(msg),
            self.gas_left()
        );

        let mut handle_result = MaybeUninit::<HandleResult>::uninit();
        let mut used_gas = 0_u64;

        // Bind the token to a local variable to ensure its
        // destructor runs in the end of the function
        let enclave_access_token = ENCLAVE_DOORBELL
            .get_access(1) // This can never be recursive
            .ok_or_else(Self::busy_enclave_err)?;
        let enclave = enclave_access_token.map_err(EnclaveError::sdk_err)?;

        let status = unsafe {
            imports::ecall_handle(
                enclave.geteid(),
                handle_result.as_mut_ptr(),
                self.ctx.unsafe_clone(),
                self.gas_left(),
                &mut used_gas,
                self.bytecode.as_ptr(),
                self.bytecode.len(),
                env.as_ptr(),
                env.len(),
                msg.as_ptr(),
                msg.len(),
                sig_info.as_ptr(),
                sig_info.len(),
                handle_type,
            )
        };

        trace!(
            "handle() returned with gas_used: {} (gas_limit: {})",
            used_gas,
            self.gas_limit
        );
        self.consume_gas(used_gas);

        match status {
            sgx_status_t::SGX_SUCCESS => {
                let handle_result = unsafe { handle_result.assume_init() };
                handle_result_to_vm_result(handle_result)
            }
            failure_status => Err(EnclaveError::sdk_err(failure_status).into()),
        }
    }

    pub fn query(&mut self, env: &[u8], msg: &[u8]) -> VmResult<QuerySuccess> {
        trace!(
            "query() called with env: {:?} msg: {:?}",
            String::from_utf8_lossy(env),
            String::from_utf8_lossy(msg),
        );

        let mut query_result = MaybeUninit::<QueryResult>::uninit();
        let mut used_gas = 0_u64;

        // #[cfg(not(feature = "query-node"))]
        let doorbell = &ENCLAVE_DOORBELL;
        // #[cfg(feature = "query-node")]
        // let doorbell = &QUERY_ENCLAVE_DOORBELL;

        // Bind the token to a local variable to ensure its
        // destructor runs in the end of the function
        let enclave_access_token = doorbell
            .get_access(get_query_depth(env)?)
            .ok_or_else(Self::busy_enclave_err)?;
        let enclave = enclave_access_token.map_err(EnclaveError::sdk_err)?;

        let status = unsafe {
            imports::ecall_query(
                // TODO use the _qe variant
                enclave.geteid(),
                query_result.as_mut_ptr(),
                self.ctx.unsafe_clone(),
                self.gas_left(),
                &mut used_gas,
                self.bytecode.as_ptr(),
                self.bytecode.len(),
                env.as_ptr(),
                env.len(),
                msg.as_ptr(),
                msg.len(),
            )
        };

        trace!(
            "query() returned with gas_used: {} (gas_limit: {})",
            used_gas,
            self.gas_limit
        );
        self.consume_gas(used_gas);

        match status {
            sgx_status_t::SGX_SUCCESS => {
                let query_result = unsafe { query_result.assume_init() };
                query_result_to_vm_result(query_result)
            }
            failure_status => Err(EnclaveError::sdk_err(failure_status).into()),
        }
    }

    fn consume_gas(&mut self, used_gas: u64) {
        self.used_gas = self.used_gas.saturating_add(used_gas);
    }
}

impl<S, Q> Drop for Module<S, Q>
where
    S: Storage,
    Q: Querier,
{
    fn drop(&mut self) {
        let context_data = unsafe { (*(self.ctx.data as *mut FullContext)).context_data };
        (self.finalizer)(context_data);
    }
}

/// This type is used to extract the `query_depth` field which starts out at 1
/// and is incremented every time a recursive query is called.
/// We do not include the other fields of the Env here
/// to reduce the need to keep this type in sync with the canonical `Env` type.
#[derive(Debug, Deserialize)]
struct Env {
    #[serde(default)]
    query_depth: u32,
}

/// This function parses the `env` parameter using the type above, and extracts the
/// `recursive` field from it.
fn get_query_depth(env: &[u8]) -> VmResult<u32> {
    match serde_json::from_slice::<Env>(env) {
        Ok(env) => Ok(env.query_depth),
        Err(_err) => Err(VmError::generic_err(format!(
            "could not parse the env parameter: {:?}",
            String::from_utf8_lossy(env)
        ))),
    }
}

fn get_busy_enclave_error() -> VmError {
    VmError::generic_err("The enclave is too busy and can not respond to this query")
}

pub fn get_random_number_from_enclave() -> u64 {
    trace!("get_random_number_from_enclave() called");

    let mut random_retval = MaybeUninit::<GenerateRandomResult>::uninit();

    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or_else(get_busy_enclave_error).unwrap();

    let enclave = enclave_access_token.map_err(EnclaveError::sdk_err).unwrap();

    let status = unsafe {
        imports::ecall_generate_random(
            enclave.geteid(),
            random_retval.as_mut_ptr(),
        )
    };

    trace!("generate_random() returned");

    match status {
        sgx_status_t::SGX_SUCCESS => {
            let random_result = unsafe { random_retval.assume_init() };
            match random_result {
                GenerateRandomResult::Success{ encrypted_output } => {
                    encrypted_output
                },
                GenerateRandomResult::Failure { .. } => {
                    222u64
                }
            }
        }
        failure_status => 0,
    }
}
