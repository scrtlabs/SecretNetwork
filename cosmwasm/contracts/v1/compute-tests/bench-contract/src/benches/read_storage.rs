use super::{LARGE_VALUE, LARGE_VALUE_SIZE};
use cosmwasm_std::{DepsMut, StdResult};

pub const READ_LARGE_STORAGE_KEY: &[u8] = b"large_storage_key";
pub const READ_LARGE_STORAGE_KEYS: &[u8] = b"large_storage_keys";

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
pub fn bench_read_large_key_from_storage(
    deps: DepsMut,
    key_tag: &[u8],
    keys: u64,
) -> StdResult<()> {
    // deps.storage.set(b"test.key", crate::benches::LARGE_VALUE);
    for _ in 0..keys {
        deps.storage.get(key_tag);
    }

    Ok(())
}

pub fn setup_read_large_from_storage(
    deps: DepsMut,
    key_tag: &[u8],
    amount_of_chunks: usize,
) -> StdResult<()> {
    let mut chunked_large_value = vec![0 as u8; amount_of_chunks * LARGE_VALUE_SIZE];
    for _ in 0..amount_of_chunks {
        chunked_large_value.extend_from_slice(LARGE_VALUE);
    }

    deps.storage.set(key_tag, &chunked_large_value);

    Ok(())
}
