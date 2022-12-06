use sha2::{Digest, Sha256};

// use crate::msg::BenchResponse;
use cosmwasm_std::StdError;

const BENCH_NAME: &str = "bench_cpu_sha256";

pub fn do_cpu_loop(num_of_runs: usize) -> Result<(), StdError> {
    let mut hashed: Vec<u8> = BENCH_NAME.into();
    for _i in 1..num_of_runs {
        hashed = Sha256::digest(&hashed).to_vec()
    }

    Ok(())
    // Ok(BenchResponse {
    //     name: BENCH_NAME.to_string(),
    //     time: 0,
    // })
}
