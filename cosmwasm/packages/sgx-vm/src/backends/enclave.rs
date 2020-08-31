#![cfg(any(feature = "enclave", feature = "default-enclave"))]
use crate::traits::{Querier, Storage};
use crate::wasmi::Module;

/// Get how many more gas units can be used in the instance.
#[allow(unused)]
pub fn get_gas_left<S, Q>(instance: &Module<S, Q>) -> u64
where
    S: Storage,
    Q: Querier,
{
    instance.gas_left()
}

/// Get how many gas units were used in the instance.
pub fn get_gas_used<S, Q>(instance: &Module<S, Q>) -> u64
where
    S: Storage,
    Q: Querier,
{
    instance.gas_used()
}
