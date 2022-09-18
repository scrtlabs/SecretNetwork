use cosmwasm_std::{Deps, DepsMut, StdResult};

pub fn bench_read_storage_same_key(deps: DepsMut, keys: u64) -> StdResult<()> {
    deps.storage.set(b"test.key", b"test.value");

    for _ in 1..keys {
        deps.storage.get(b"test.key");
    }

    Ok(())
}

#[allow(dead_code)]
/// call this test only after the bench of write storage, so the keys are populated
pub fn bench_read_storage_different_key(deps: Deps, keys: u64) -> StdResult<()> {
    for i in 1..keys {
        deps.storage.get(&i.to_be_bytes()).unwrap();
    }

    Ok(())
}
