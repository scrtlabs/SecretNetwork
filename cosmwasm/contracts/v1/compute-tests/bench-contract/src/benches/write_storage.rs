use cosmwasm_std::{DepsMut, StdResult};

#[allow(dead_code)]
pub fn bench_write_storage_same_key(deps: DepsMut, keys: u64) -> StdResult<()> {
    for i in 1..keys {
        deps.storage.set(b"test.key", i.to_string().as_bytes());
    }

    Ok(())
}

pub fn bench_write_storage_different_key(deps: DepsMut, keys: u64) -> StdResult<()> {
    for i in 0..keys {
        deps.storage.set(&i.to_be_bytes(), i.to_string().as_bytes());
    }

    Ok(())
}

pub fn bench_write_large_storage_key(deps: DepsMut, keys: u64) -> StdResult<()> {
    for i in 0..keys {
        deps.storage
            .set(&i.to_be_bytes(), crate::benches::LARGE_VALUE);
    }

    Ok(())
}

