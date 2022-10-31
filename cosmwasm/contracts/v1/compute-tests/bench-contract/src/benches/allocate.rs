use cosmwasm_std::{StdError, StdResult};

pub fn do_allocate_large_memory() -> StdResult<()> {
    // We create memory pages explicitely since Rust's default allocator seems to be clever enough
    // to not grow memory for unused capacity like `Vec::<u8>::with_capacity(100 * 1024 * 1024)`.
    // Even with std::alloc::alloc the memory did now grow beyond 1.5 MiB.

    #[cfg(target_arch = "wasm32")]
    {
        use core::arch::wasm32;
        let pages = 1_600; // 100 MiB
        let ptr = wasm32::memory_grow(0, pages);
        if ptr == usize::max_value() {
            return Err(StdError::generic_err("Error in memory.grow instruction"));
        }
        Ok(())
    }

    #[cfg(not(target_arch = "wasm32"))]
    Err(StdError::generic_err("Unsupported architecture"))
}
