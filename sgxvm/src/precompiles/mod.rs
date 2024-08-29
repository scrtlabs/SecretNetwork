extern crate sgx_tstd as std;

use evm::{
    executor::stack::{PrecompileFailure, PrecompileHandle, PrecompileOutput, PrecompileSet, IsPrecompileResult},
    ExitError, 
    ExitSucceed,
};
use std::vec::Vec;
use primitive_types::H160 ;
use crate::GoQuerier;

mod blake2f;
mod bn128;
mod curve25519;
mod modexp;
mod sha3fips;
mod ec_recover;
mod sha256;
mod ripemd160;
mod datacopy;
mod compliance_bridge;
mod secp256r1;

pub type PrecompileResult = Result<PrecompileOutput, PrecompileFailure>;

/// One single precompile used by EVM engine.
pub trait Precompile {
    /// Try to execute the precompile with given `handle` which provides all call data
    /// and allow to register costs and logs.
    fn execute(handle: &mut impl PrecompileHandle) -> PrecompileResult;
}

pub trait LinearCostPrecompile {
    const BASE: u64;
    const WORD: u64;

    fn raw_execute(
        input: &[u8],
        cost: u64,
    ) -> Result<(ExitSucceed, Vec<u8>), PrecompileFailure>;
}

/// Precompile with possibility to interact with Cosmos side using GoQuerier
pub trait LinearCostPrecompileWithQuerier {
    const BASE: u64;
    const WORD: u64;

    fn execute(querier: *mut GoQuerier, handle: &mut impl PrecompileHandle) -> PrecompileResult;
}

impl<T: LinearCostPrecompile> Precompile for T {
    fn execute(handle: &mut impl PrecompileHandle) -> PrecompileResult {
        let target_gas = handle.gas_limit();
        let cost = ensure_linear_cost(target_gas, handle.input().len() as u64, T::BASE, T::WORD)?;

        handle.record_cost(cost)?;
        let (exit_status, output) = T::raw_execute(handle.input(), cost)?;
        Ok(PrecompileOutput {
            exit_status,
            output,
        })
    }
}

/// Linear gas cost
pub fn ensure_linear_cost(
    target_gas: Option<u64>,
    len: u64,
    base: u64,
    word: u64,
) -> Result<u64, PrecompileFailure> {
    let cost = base
        .checked_add(word.checked_mul(len.saturating_add(31) / 32).ok_or(
            PrecompileFailure::Error {
                exit_status: ExitError::OutOfGas,
            },
        )?)
        .ok_or(PrecompileFailure::Error {
            exit_status: ExitError::OutOfGas,
        })?;

    if let Some(target_gas) = target_gas {
        if cost > target_gas {
            return Err(PrecompileFailure::Error {
                exit_status: ExitError::OutOfGas,
            });
        }
    }

    Ok(cost)
}

pub struct EVMPrecompiles {
    querier: *mut GoQuerier,
}

impl EVMPrecompiles {
    pub fn new(querier: *mut GoQuerier) -> Self {
        Self{ querier }
    }
    pub fn used_addresses() -> [H160; 17] {
        [
            hash(1),
            hash(2),
            hash(3),
            hash(4),
            hash(5),
            hash(6),
            hash(7),
            hash(8),
            hash(9),
            hash(0x100),
            hash(1024),
            hash(1025),
            hash(1027),
            hash(1028),
            hash(1029),
            hash(1030),
            hash(1031),
        ]
    }
}

impl PrecompileSet for EVMPrecompiles {
    fn execute(&self, handle: &mut impl PrecompileHandle) -> Option<PrecompileResult> {
        match handle.code_address() {
            // Ethereum precompiles:
            a if a == hash(1) => Some(ec_recover::ECRecover::execute(handle)),
            a if a == hash(2) => Some(sha256::Sha256::execute(handle)),
            a if a == hash(3) => Some(ripemd160::Ripemd160::execute(handle)),
            a if a == hash(4) => Some(datacopy::DataCopy::execute(handle)),
            a if a == hash(5) => Some(modexp::Modexp::execute(handle)),
            a if a == hash(6) => Some(bn128::Bn128Add::execute(handle)),
            a if a == hash(7) => Some(bn128::Bn128Mul::execute(handle)),
            a if a == hash(8) => Some(bn128::Bn128Pairing::execute(handle)),
            a if a == hash(9) => Some(blake2f::Blake2F::execute(handle)),
            // RIP-7212
            a if a == hash(0x100) => Some(secp256r1::P256Verify::execute(handle)),
            // Non-Frontier specific nor Ethereum precompiles :
            a if a == hash(1024) => Some(sha3fips::Sha3FIPS256::execute(handle)),
            a if a == hash(1025) => Some(sha3fips::Sha3FIPS512::execute(handle)),
            a if a == hash(1028) => Some(compliance_bridge::ComplianceBridge::execute(self.querier, handle)),
            a if a == hash(1029) => Some(curve25519::Curve25519Add::execute(handle)),
            a if a == hash(1030) => Some(curve25519::Curve25519ScalarMul::execute(handle)),
            a if a == hash(1031) => Some(curve25519::Ed25519Verify::execute(handle)),
            _ => None,
        }
    }

    fn is_precompile(&self, address: H160, _gas: u64) -> IsPrecompileResult {
		IsPrecompileResult::Answer {
			is_precompile: Self::used_addresses().contains(&address),
			extra_cost: 0,
		}
    }
}

#[inline]
fn hash(a: u64) -> H160 {
    H160::from_low_u64_be(a)
}
