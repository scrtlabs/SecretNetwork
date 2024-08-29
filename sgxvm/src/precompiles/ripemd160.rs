use std::vec::Vec;
use k256::sha2::Digest;
use crate::precompiles::{
    ExitSucceed, 
    LinearCostPrecompile, 
    PrecompileFailure
};

/// The ripemd precompile.
pub struct Ripemd160;

impl LinearCostPrecompile for Ripemd160 {
    const BASE: u64 = 600;
    const WORD: u64 = 120;

    fn raw_execute(input: &[u8], _cost: u64) -> Result<(ExitSucceed, Vec<u8>), PrecompileFailure> {
        let mut ret = [0u8; 32];
        ret[12..32].copy_from_slice(&ripemd::Ripemd160::digest(input));
        Ok((ExitSucceed::Returned, ret.to_vec()))
    }
}