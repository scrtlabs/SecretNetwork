//! This module provides safe wrappers for the calls into the enclave running WASMI.

use std::ffi::c_void;
use std::marker::PhantomData;
use std::mem::MaybeUninit;

use crate::errors::{EnclaveError, VmResult};
use crate::{Querier, Storage};

use enclave_ffi_types::{Ctx, EnclaveBuffer, HandleResult, InitResult, QueryResult};

use sgx_types::sgx_status_t;
use sgx_urts::SgxEnclave;

use log::trace;

use super::exports::FullContext;
use super::imports;
use super::results::{
    handle_result_to_result_handlesuccess, init_result_to_result_initsuccess,
    query_result_to_result_querysuccess, HandleSuccess, InitSuccess, QuerySuccess,
};

/// This is a safe wrapper for allocating buffers inside the enclave.
pub(super) fn allocate_enclave_buffer(buffer: &[u8]) -> Result<EnclaveBuffer, sgx_status_t> {
    let ptr = buffer.as_ptr();
    let len = buffer.len();
    let mut enclave_buffer = MaybeUninit::<EnclaveBuffer>::uninit();

    let enclave_id = crate::enclave::get_enclave()
        .expect("If we got here, surely the enclave has been loaded")
        .geteid();

    trace!(
        target: module_path!(),
        "allocate_enclave_buffer() called with len: {:?} enclave_id: {:?}",
        len,
        enclave_id
    );

    match unsafe { imports::ecall_allocate(enclave_id, enclave_buffer.as_mut_ptr(), ptr, len) } {
        sgx_status_t::SGX_SUCCESS => Ok(unsafe { enclave_buffer.assume_init() }),
        failure_status => Err(failure_status),
    }
}

pub struct Module<S, Q>
where
    S: Storage,
    Q: Querier,
{
    bytecode: Vec<u8>,
    gas_left: u64,
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
            gas_left: gas_limit,
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

    pub fn gas_left(&self) -> u64 {
        self.gas_left
    }

    pub fn init(&mut self, env: &[u8], msg: &[u8]) -> VmResult<InitSuccess> {
        trace!(
            target: module_path!(),
            "init() called with env: {:?} msg: {:?} enclave_id: {:?} gas_limit: {}",
            String::from_utf8_lossy(env),
            String::from_utf8_lossy(msg),
            self.enclave.geteid(),
            self.gas_left
        );

        let mut init_result = MaybeUninit::<InitResult>::uninit();

        match unsafe {
            imports::ecall_init(
                self.enclave.geteid(),
                init_result.as_mut_ptr(),
                self.ctx.unsafe_clone(),
                self.gas_left,
                self.bytecode.as_ptr(),
                self.bytecode.len(),
                env.as_ptr(),
                env.len(),
                msg.as_ptr(),
                msg.len(),
            )
        } {
            sgx_status_t::SGX_SUCCESS => { /* continue */ }
            failure_status => return Err(EnclaveError::sdk_err(failure_status).into()),
        }
        // At this point we know that the ecall was successful and init_result was initialized.
        let init_result = unsafe { init_result.assume_init() };

        init_result_to_result_initsuccess(init_result)
            .map(|success| {
                trace!(
                    target: module_path!(),
                    "init() returned with gas_used: {} (gas_limit: {})",
                    success.used_gas(),
                    self.gas_left
                );
                self.gas_left -= success.used_gas();
                success
            })
            .map_err(|err| EnclaveError::enclave_err(err).into())
    }

    pub fn handle(&mut self, env: &[u8], msg: &[u8]) -> VmResult<HandleSuccess> {
        trace!(
            target: module_path!(),
            "handle() called with env: {:?} msg: {:?} enclave_id: {:?} gas_limit: {}",
            String::from_utf8_lossy(env),
            String::from_utf8_lossy(msg),
            self.enclave.geteid(),
            self.gas_left
        );

        let mut handle_result = MaybeUninit::<HandleResult>::uninit();

        match unsafe {
            imports::ecall_handle(
                self.enclave.geteid(),
                handle_result.as_mut_ptr(),
                self.ctx.unsafe_clone(),
                self.gas_left,
                self.bytecode.as_ptr(),
                self.bytecode.len(),
                env.as_ptr(),
                env.len(),
                msg.as_ptr(),
                msg.len(),
            )
        } {
            sgx_status_t::SGX_SUCCESS => { /* continue */ }
            failure_status => return Err(EnclaveError::sdk_err(failure_status).into()),
        }
        // At this point we know that the ecall was successful and handle_result was initialized.
        let handle_result = unsafe { handle_result.assume_init() };

        handle_result_to_result_handlesuccess(handle_result)
            .map(|success| {
                trace!(
                    target: module_path!(),
                    "handle() returned with gas_used: {} (gas_limit: {})",
                    success.used_gas(),
                    self.gas_left
                );
                self.gas_left -= success.used_gas();
                success
            })
            .map_err(|err| EnclaveError::enclave_err(err).into())
    }

    pub fn query(&mut self, msg: &[u8]) -> VmResult<QuerySuccess> {
        trace!(
            target: module_path!(),
            "query() called with msg: {:?} enclave_id: {:?}",
            String::from_utf8_lossy(msg),
            self.enclave.geteid()
        );

        let mut query_result = MaybeUninit::<QueryResult>::uninit();

        match unsafe {
            imports::ecall_query(
                self.enclave.geteid(),
                query_result.as_mut_ptr(),
                self.ctx.unsafe_clone(),
                self.gas_left,
                self.bytecode.as_ptr(),
                self.bytecode.len(),
                msg.as_ptr(),
                msg.len(),
            )
        } {
            sgx_status_t::SGX_SUCCESS => { /* continue */ }
            failure_status => return Err(EnclaveError::sdk_err(failure_status).into()),
        }
        // At this point we know that the ecall was successful and query_result was initialized.
        let query_result = unsafe { query_result.assume_init() };

        query_result_to_result_querysuccess(query_result)
            .map(|success| {
                self.gas_left -= success.used_gas();
                success
            })
            .map_err(|err| EnclaveError::enclave_err(err).into())
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
