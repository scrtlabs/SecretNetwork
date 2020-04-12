//! This module provides safe wrappers for the calls into the enclave running WASMI.

use std::mem::MaybeUninit;

use crate::context::context_from_dyn_storage;
use crate::Storage;
use enclave_ffi_types::{Ctx, EnclaveBuffer, HandleResult, InitResult, QueryResult};

use sgx_types::sgx_status_t;
use sgx_urts::SgxEnclave;

use crate::errors::{Error, Result};

use super::imports;
use super::results::{
    handle_result_to_result_handlesuccess, init_result_to_result_initsuccess,
    query_result_to_result_querysuccess, HandleSuccess, InitSuccess, QuerySuccess,
};

/// This is a safe wrapper for allocating buffers inside the enclave.
pub(super) fn allocate_enclave_buffer(buffer: &[u8]) -> EnclaveBuffer {
    let ptr = buffer.as_ptr();
    let len = buffer.len();
    unsafe { imports::ecall_allocate(ptr, len) }
}

pub struct Module {
    bytecode: Vec<u8>,
    storage: Option<Box<Box<dyn Storage>>>,
    gas_limit: u64,
    enclave: &'static SgxEnclave,
}

impl Module {
    pub fn new(bytecode: Vec<u8>, gas_limit: u64, enclave: &'static SgxEnclave) -> Self {
        // TODO add validation of this bytecode?
        Self {
            bytecode,
            storage: None,
            gas_limit,
            enclave,
        }
    }

    pub fn storage_mut(&mut self) -> &mut dyn Storage {
        self.storage
            .as_mut()
            .expect("This method should only be called when we have a configured storage")
            .as_mut()
            .as_mut()
    }

    pub fn set_storage(&mut self, storage: Box<dyn Storage>) {
        self.storage.replace(Box::new(storage));
    }

    pub fn take_storage(&mut self) -> Option<Box<dyn Storage>> {
        // unbox one layer to return the storage trait-item inside
        self.storage.take().map(|boxed| *boxed)
    }

    fn context(&mut self) -> Ctx {
        context_from_dyn_storage(
            &mut self
                .storage
                .as_mut()
                .expect("This method should only be called when we have a configured storage"),
        )
    }

    pub fn gas_limit(&self) -> u64 {
        self.gas_limit
    }

    pub fn init(&mut self, env: &[u8], msg: &[u8]) -> Result<InitSuccess> {
        let mut init_result = MaybeUninit::<InitResult>::uninit();

        match unsafe {
            imports::ecall_init(
                self.enclave.geteid(),
                init_result.as_mut_ptr(),
                self.context(),
                self.gas_limit,
                self.bytecode.as_ptr(),
                self.bytecode.len(),
                env.as_ptr(),
                env.len(),
                msg.as_ptr(),
                msg.len(),
            )
        } {
            sgx_status_t::SGX_SUCCESS => { /* continue */ }
            failure_status => {
                return Err(Error::SdkErr {
                    inner: failure_status,
                })
            }
        }
        // At this point we know that the ecall was successful and init_result was initialized.
        let init_result = unsafe { init_result.assume_init() };

        init_result_to_result_initsuccess(init_result)
            .map(|success| {
                self.gas_limit -= success.used_gas();
                success
            })
            .map_err(|err| Error::EnclaveErr { inner: err })
    }

    pub fn handle(&mut self, env: &[u8], msg: &[u8]) -> Result<HandleSuccess> {
        let mut handle_result = MaybeUninit::<HandleResult>::uninit();

        match unsafe {
            imports::ecall_handle(
                self.enclave.geteid(),
                handle_result.as_mut_ptr(),
                self.context(),
                self.gas_limit,
                self.bytecode.as_ptr(),
                self.bytecode.len(),
                env.as_ptr(),
                env.len(),
                msg.as_ptr(),
                msg.len(),
            )
        } {
            sgx_status_t::SGX_SUCCESS => { /* continue */ }
            failure_status => {
                return Err(Error::SdkErr {
                    inner: failure_status,
                })
            }
        }
        // At this point we know that the ecall was successful and handle_result was initialized.
        let handle_result = unsafe { handle_result.assume_init() };

        handle_result_to_result_handlesuccess(handle_result)
            .map(|success| {
                self.gas_limit -= success.used_gas();
                success
            })
            .map_err(|err| Error::EnclaveErr { inner: err })
    }

    pub fn query(&mut self, msg: &[u8]) -> Result<QuerySuccess> {
        let mut query_result = MaybeUninit::<QueryResult>::uninit();

        match unsafe {
            imports::ecall_query(
                self.enclave.geteid(),
                query_result.as_mut_ptr(),
                self.context(),
                self.gas_limit,
                self.bytecode.as_ptr(),
                self.bytecode.len(),
                msg.as_ptr(),
                msg.len(),
            )
        } {
            sgx_status_t::SGX_SUCCESS => { /* continue */ }
            failure_status => {
                return Err(Error::SdkErr {
                    inner: failure_status,
                })
            }
        }
        // At this point we know that the ecall was successful and query_result was initialized.
        let query_result = unsafe { query_result.assume_init() };

        query_result_to_result_querysuccess(query_result)
            .map(|success| {
                self.gas_limit -= success.used_gas();
                success
            })
            .map_err(|err| Error::EnclaveErr { inner: err })
    }
}
