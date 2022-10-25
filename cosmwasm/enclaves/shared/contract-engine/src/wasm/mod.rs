#[cfg(feature = "wasmi-engine")]
pub(crate) mod contract;
#[cfg(feature = "wasmi-engine")]
mod engine;
#[cfg(feature = "wasmi-engine")]
mod externals;
#[cfg(feature = "wasmi-engine")]
mod import_resolver;
#[cfg(feature = "wasmi-engine")]
mod memory;
#[cfg(feature = "wasmi-engine")]
pub(crate) mod module_cache;
#[cfg(feature = "wasmi-engine")]
pub mod traits;

#[cfg(feature = "wasmi-engine")]
pub use contract::{ContractInstance, ContractOperation};
#[cfg(feature = "wasmi-engine")]
pub use cw_types_generic::CosmWasmApiVersion;
#[cfg(feature = "wasmi-engine")]
pub use engine::Engine;
#[cfg(feature = "wasmi-engine")]
pub use import_resolver::{create_builder, WasmiImportResolver};
