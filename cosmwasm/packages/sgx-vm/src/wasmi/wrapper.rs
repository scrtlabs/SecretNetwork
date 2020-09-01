//! This module provides safe wrappers for the calls into the enclave running WASMI.

use std::ffi::c_void;
use std::marker::PhantomData;
use std::mem::MaybeUninit;

use crate::errors::{EnclaveError, VmResult};
use crate::{Querier, Storage};

use enclave_ffi_types::{Ctx, EnclaveBuffer, HandleResult, InitResult, QueryResult};

use sgx_types::{sgx_status_t, SgxResult};
use sgx_urts::SgxEnclave;

use log::*;

use super::exports::FullContext;
use super::imports;
use super::results::{
    handle_result_to_vm_result, init_result_to_vm_result, query_result_to_vm_result, HandleSuccess,
    InitSuccess, QuerySuccess,
};

/// This is a safe wrapper for allocating buffers inside the enclave.
pub(super) fn allocate_enclave_buffer(buffer: &[u8]) -> SgxResult<EnclaveBuffer> {
    let ptr = buffer.as_ptr();
    let len = buffer.len();
    let mut enclave_buffer = EnclaveBuffer::default();

    let enclave_id = crate::enclave::get_enclave()
        .expect("If we got here, surely the enclave has been loaded")
        .geteid();

    trace!(
        target: module_path!(),
        "allocate_enclave_buffer() called with len: {:?} enclave_id: {:?}",
        len,
        enclave_id,
    );

    match unsafe { imports::ecall_allocate(enclave_id, &mut enclave_buffer, ptr, len) } {
        sgx_status_t::SGX_SUCCESS => Ok(enclave_buffer),
        failure_status => Err(failure_status),
    }
}

pub struct Module<S, Q>
where
    S: Storage,
    Q: Querier,
{
    bytecode: Vec<u8>,
    gas_limit: u64,
    used_gas: u64,
    enclave: &'static SgxEnclave,
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
        enclave: &'static SgxEnclave,
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
            enclave,
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

    pub fn init(&mut self, env: &[u8], msg: &[u8], sig_info: &[u8]) -> VmResult<InitSuccess> {
        trace!(
            "init() called with env: {:?} msg: {:?} enclave_id: {:?} gas_left: {}",
            String::from_utf8_lossy(env),
            String::from_utf8_lossy(msg),
            self.enclave.geteid(),
            self.gas_left()
        );

        let mut init_result = MaybeUninit::<InitResult>::uninit();
        let mut used_gas = 0_u64;

        let status = unsafe {
            imports::ecall_init(
                self.enclave.geteid(),
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

    pub fn handle(&mut self, env: &[u8], msg: &[u8], sig_info: &[u8]) -> VmResult<HandleSuccess> {
        trace!(
            "handle() called with env: {:?} msg: {:?} enclave_id: {:?} gas_left: {}",
            String::from_utf8_lossy(env),
            String::from_utf8_lossy(msg),
            self.enclave.geteid(),
            self.gas_left()
        );

        let mut handle_result = MaybeUninit::<HandleResult>::uninit();
        let mut used_gas = 0_u64;

        let status = unsafe {
            imports::ecall_handle(
                self.enclave.geteid(),
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

    pub fn query(&mut self, msg: &[u8]) -> VmResult<QuerySuccess> {
        trace!(
            "query() called with msg: {:?} enclave_id: {:?}",
            String::from_utf8_lossy(msg),
            self.enclave.geteid()
        );

        let mut query_result = MaybeUninit::<QueryResult>::uninit();
        let mut used_gas = 0_u64;

        let status = unsafe {
            imports::ecall_query(
                self.enclave.geteid(),
                query_result.as_mut_ptr(),
                self.ctx.unsafe_clone(),
                self.gas_left(),
                &mut used_gas,
                self.bytecode.as_ptr(),
                self.bytecode.len(),
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
