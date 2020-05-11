/**
Internal details to be used by instance.rs only
**/
// use std::convert::TryInto;
use std::ffi::c_void;
// use std::mem;

// use wasmer_runtime_core::vm::Ctx;

// use cosmwasm::traits::{Api, Storage};

use crate::Storage;
// use crate::errors::Error;
// use crate::memory::{read_region, write_region};
// use cosmwasm::encoding::Binary;
// use cosmwasm::types::{CanonicalAddr, HumanAddr};
use enclave_ffi_types::Ctx;

/*
/// An unknown error occurred when writing to region
static ERROR_WRITE_TO_REGION_UNKNONW: i32 = -1000001;
/// Could not write to region because it is too small
static ERROR_WRITE_TO_REGION_TOO_SMALL: i32 = -1000002;
*/

/*
pub fn do_read<T: Storage>(ctx: &Ctx, key_ptr: u32, value_ptr: u32) -> i32 {
    let key = read_region(ctx, key_ptr);
    let mut value: Option<Vec<u8>> = None;
    with_storage_from_context(ctx, |store: &mut T| value = store.get(&key));
    match value {
        Some(buf) => match write_region(ctx, value_ptr, &buf) {
            Ok(bytes_written) => bytes_written.try_into().unwrap(),
            Err(Error::RegionTooSmallErr { .. }) => ERROR_WRITE_TO_REGION_TOO_SMALL,
            Err(_) => ERROR_WRITE_TO_REGION_UNKNONW,
        },
        None => 0,
    }
}

pub fn do_write<T: Storage>(ctx: &Ctx, key_ptr: u32, value_ptr: u32) {
    let key = read_region(ctx, key_ptr);
    let value = read_region(ctx, value_ptr);
    with_storage_from_context(ctx, |store: &mut T| store.set(&key, &value));
}

pub fn do_canonical_address<A: Api>(
    api: A,
    ctx: &mut Ctx,
    human_ptr: u32,
    canonical_ptr: u32,
) -> i32 {
    let human = read_region(ctx, human_ptr);
    let human = match String::from_utf8(human) {
        Ok(human_str) => HumanAddr(human_str),
        Err(_) => return -2,
    };
    match api.canonical_address(&human) {
        Ok(canon) => match write_region(ctx, canonical_ptr, canon.as_slice()) {
            Ok(bytes_written) => bytes_written.try_into().unwrap(),
            Err(Error::RegionTooSmallErr { .. }) => ERROR_WRITE_TO_REGION_TOO_SMALL,
            Err(_) => ERROR_WRITE_TO_REGION_UNKNONW,
        },
        Err(_) => -1,
    }
}

pub fn do_human_address<A: Api>(api: A, ctx: &mut Ctx, canonical_ptr: u32, human_ptr: u32) -> i32 {
    let canon = Binary(read_region(ctx, canonical_ptr));
    match api.human_address(&CanonicalAddr(canon)) {
        Ok(human) => match write_region(ctx, human_ptr, human.as_str().as_bytes()) {
            Ok(bytes_written) => bytes_written.try_into().unwrap(),
            Err(Error::RegionTooSmallErr { .. }) => ERROR_WRITE_TO_REGION_TOO_SMALL,
            Err(_) => ERROR_WRITE_TO_REGION_UNKNONW,
        },
        Err(_) => -1,
    }
}

/** context data **/

struct ContextData<S: Storage> {
    data: Option<S>,
}

pub fn setup_context<S: Storage>() -> (*mut c_void, fn(*mut c_void)) {
    (
        create_unmanaged_storage::<S>(),
        destroy_unmanaged_storage::<S>,
    )
}

fn create_unmanaged_storage<S: Storage>() -> *mut c_void {
    let data = ContextData::<S> { data: None };
    let state = Box::new(data);
    Box::into_raw(state) as *mut c_void
}

unsafe fn get_data<S: Storage>(ptr: *mut c_void) -> Box<ContextData<S>> {
    Box::from_raw(ptr as *mut ContextData<S>)
}

fn destroy_unmanaged_storage<S: Storage>(ptr: *mut c_void) {
    if !ptr.is_null() {
        // auto-dropped with scope
        let _ = unsafe { get_data::<S>(ptr) };
    }
}

pub fn with_storage_from_context<S: Storage, F: FnMut(&mut S)>(ctx: &Ctx, mut func: F) {
    let mut storage: Option<S> = take_storage(ctx);
    if let Some(data) = &mut storage {
        func(data);
    }
    leave_storage(ctx, storage);
}

pub fn take_storage<S: Storage>(ctx: &Ctx) -> Option<S> {
    let mut b = unsafe { get_data(ctx.data) };
    let res = b.data.take();
    mem::forget(b); // we do this to avoid cleanup
    res
}

pub fn leave_storage<S: Storage>(ctx: &Ctx, storage: Option<S>) {
    let mut b = unsafe { get_data(ctx.data) };
    // clean-up if needed
    let _ = b.data.take();
    b.data = storage;
    mem::forget(b); // we do this to avoid cleanup
}
*/

/// This function takes a `Box<Box<dyn Storage>>` because `Box<dyn Storage>` is a trait item. This means it
/// holds the pointer to the real value and a vtable pointer, both inline.
/// Instead, we ask that users move the trait item to the Heap (the second `Box`), and we then return a pointer
/// to that heap location inside of the `Ctx` instance.
pub fn context_from_dyn_storage(storage: &mut Box<Box<dyn Storage>>) -> Ctx {
    let storage: &mut Box<dyn Storage> = &mut *storage;
    Ctx {
        data: storage as *mut Box<dyn Storage> as *mut c_void,
    }
}

/// Using the context, dereference it all the way to the underlying storage, and call the function with a
/// reference to it. This function panics if the type of expected storage differs from the type of storage
/// behind the trait item.
pub fn with_storage_from_context<F: FnMut(&mut dyn Storage)>(ctx: &mut Ctx, mut func: F) {
    // First convert the pointer to the type we saved it as in the `context_from_dyn_storage` function.
    let storage: &mut Box<dyn Storage> = unsafe { &mut *(ctx.data as *mut Box<dyn Storage>) };

    func(&mut **storage);
}
