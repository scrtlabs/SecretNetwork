use cosmwasm_std::{DepsMut, StdResult};

const READ_LARGE_STORAGE_KEY: &[u8] = b"large_storage_key";

// as long as keys is > 10 the single write shouldn't produce high enough variance
// if anyone wants to they can set up test.key in the go side of things to make it more accurate
pub fn bench_read_storage_same_key(deps: DepsMut, keys: u64) -> StdResult<()> {
    deps.storage.set(b"test.key", b"test.value");

    for _ in 1..keys {
        deps.storage.get(b"test.key");
    }

    Ok(())
}

/// call this test only after setting up the test with write storage, so the keys are populated
pub fn bench_read_storage_different_key(deps: DepsMut, keys: u64) -> StdResult<()> {
    for i in 0..keys {
        deps.storage.get(&i.to_be_bytes()).unwrap();
    }

    Ok(())
}

/// call this test only after setting up the test with write storage, so the keys are populated
pub fn bench_read_large_key_from_storage(deps: DepsMut, keys: u64) -> StdResult<()> {
    // deps.storage.set(b"test.key", crate::benches::LARGE_VALUE);
    for _ in 0..keys {
        deps.storage.get(READ_LARGE_STORAGE_KEY);
    }

    Ok(())
}

pub fn setup_read_large_from_storage(deps: DepsMut) -> StdResult<()> {
    deps.storage
        .set(READ_LARGE_STORAGE_KEY, crate::benches::LARGE_VALUE);

    Ok(())
}
