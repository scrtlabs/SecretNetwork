mod cache;
mod calls;
mod compatability;
mod context;
pub mod errors;
pub mod instance;
mod mock;
pub mod testing;
pub mod traits;

mod seed;
mod wasm_store;
mod wasmi;

pub mod attestation;

// extern crate sgx_signal;
// use sgx_signal;

pub use crate::cache::CosmCache;
pub use crate::calls::{
    call_handle, call_handle_raw, call_init, call_init_raw, call_query, call_query_raw,
};
pub use crate::instance::Instance;
pub use crate::traits::{Extern, ReadonlyStorage, Storage};

pub use crate::instance::{create_attestation_report_u, init_seed_u, untrusted_get_encrypted_seed, untrusted_init_bootstrap};
