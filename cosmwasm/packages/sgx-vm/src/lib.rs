mod backends;
mod cache;
mod calls;
mod checksum;
mod compatability;
mod context;
mod conversion;
mod errors;
mod features;
mod ffi;
mod instance;
mod serde;
pub mod testing;
mod traits;

// Secret Network specific modules
mod attestation;
mod enclave;
mod enclave_config;
mod proofs;
mod seed;
mod wasmi;

mod random;

#[cfg(feature = "enclave-tests")]
pub mod enclave_tests;

pub use crate::cache::CosmCache;
pub use crate::calls::{call_handle_raw, call_init_raw, call_query_raw};
pub use crate::checksum::Checksum;
pub use crate::errors::{
    CommunicationError, CommunicationResult, RegionValidationError, RegionValidationResult,
    VmError, VmResult,
};
pub use crate::features::features_from_csv;
pub use crate::ffi::{FfiError, FfiResult, GasInfo};
pub use crate::instance::{GasReport, Instance};
pub use crate::serde::{from_slice, to_vec};
pub use crate::traits::{Api, Extern, Querier, Storage};

#[cfg(feature = "iterator")]
pub use crate::traits::StorageIterator;

// Secret Network specific exports
pub use crate::attestation::{create_attestation_report_u, untrusted_get_encrypted_seed};
pub use crate::proofs::untrusted_submit_store_roots;
pub use crate::random::untrusted_submit_block_signatures;
pub use crate::seed::{
    untrusted_health_check, untrusted_init_bootstrap, untrusted_init_node, untrusted_key_gen,
};

pub use enclave_config::{configure_enclave, EnclaveRuntimeConfig};
