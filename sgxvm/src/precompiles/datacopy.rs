use std::vec::Vec;
use crate::precompiles::{
    ExitSucceed, 
    LinearCostPrecompile, 
    PrecompileFailure
};

/// The DataCopy precompile.
pub struct DataCopy;

impl LinearCostPrecompile for DataCopy {
	const BASE: u64 = 15;
	const WORD: u64 = 3;

	fn raw_execute(input: &[u8], _: u64) -> Result<(ExitSucceed, Vec<u8>), PrecompileFailure> {
		Ok((ExitSucceed::Returned, input.to_vec()))
	}
}