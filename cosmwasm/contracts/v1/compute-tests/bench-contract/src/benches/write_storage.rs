use cosmwasm_std::{DepsMut, StdResult};

use super::{LARGE_VALUE, LARGE_VALUE_SIZE};

#[allow(dead_code)]
pub fn bench_write_storage_same_key(deps: DepsMut, keys: u64) -> StdResult<()> {
    for i in 1..keys {
        deps.storage.set(b"test.key", i.to_string().as_bytes());
    }

    Ok(())
}

pub fn bench_write_storage_different_key(deps: DepsMut, keys: u64) -> StdResult<()> {
    for i in 1..keys {
        deps.storage.set(&i.to_be_bytes(), i.to_string().as_bytes());
    }

    Ok(())
}

pub fn bench_write_large_storage_key(deps: DepsMut, keys: u64, chunks: String) -> StdResult<()> {
    let amount_of_chunks = chunks.parse::<usize>().unwrap();
    let mut chunked_large_value = vec![0 as u8; amount_of_chunks * LARGE_VALUE_SIZE];
    for _ in 0..amount_of_chunks {
        chunked_large_value.extend_from_slice(LARGE_VALUE);
    }

    for i in 0..keys {
        deps.storage.set(&i.to_be_bytes(), &chunked_large_value);
    }

    Ok(())
}
