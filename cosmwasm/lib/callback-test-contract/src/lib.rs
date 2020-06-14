pub mod contract;
pub mod msg;
pub mod state;

#[cfg(target_arch = "wasm32")]
mod wasm {
    use super::contract;
    use cosmwasm::{exports, imports};
    use std::ffi::c_void;

    #[no_mangle]
    extern "C" fn init(params_ptr: *mut c_void, msg_ptr: *mut c_void) -> *mut c_void {
        exports::do_init(
            &contract::init::<imports::ExternalStorage, imports::ExternalApi>,
            params_ptr,
            msg_ptr,
        )
    }

    #[no_mangle]
    extern "C" fn handle(params_ptr: *mut c_void, msg_ptr: *mut c_void) -> *mut c_void {
        exports::do_handle(
            &contract::handle::<imports::ExternalStorage, imports::ExternalApi>,
            params_ptr,
            msg_ptr,
        )
    }

    #[no_mangle]
    extern "C" fn query(msg_ptr: *mut c_void) -> *mut c_void {
        exports::do_query(
            &contract::query::<imports::ExternalStorage, imports::ExternalApi>,
            msg_ptr,
        )
    }

    // Other C externs like cosmwasm_api_0_6, allocate, deallocate are available
    // automatically because we `use cosmwasm`.
}
