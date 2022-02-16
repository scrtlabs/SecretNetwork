use core::sync::atomic::{AtomicBool, Ordering};
use enclave_ffi_types::EnclaveError;
use lazy_static::lazy_static;

#[cfg(not(feature = "production"))]
use std::backtrace::{self, PrintFormat};

use std::sync::SgxMutex;
/// SafetyBuffer is meant to occupy space on the heap, so when a memory
/// allocation fails we will free this buffer to allow safe panic unwinding
/// This is needed because while unwinding from panic some destructors try
/// to allocate more memory which causes a double fault. This way we can
/// make sure the unwind process has enough free memory to work properly.
struct SafetyBuffer {
    chunks: usize,
    min_chunks: usize,
    buffer: Vec<Vec<u8>>,
}

impl SafetyBuffer {
    /// Allocate `chunks` KiB on the heap
    pub fn new(chunks: usize, min_chunks: usize) -> Self {
        SafetyBuffer {
            chunks,
            min_chunks,
            buffer: SafetyBuffer::build_buffer(chunks, min_chunks).unwrap(),
        }
    }

    /// Free the buffer to allow panic to safely unwind
    pub fn clear(&mut self) {
        let mut temp = Vec::new();
        std::mem::swap(&mut self.buffer, &mut temp);
        drop(temp)
    }

    fn build_buffer(chunks: usize, min_chunks: usize) -> Result<Vec<Vec<u8>>, EnclaveError> {
        let mut buffer: Vec<Vec<u8>> = Vec::with_capacity(chunks);
        SafetyBuffer::top_up_buffer(&mut buffer, chunks, min_chunks)?;
        Ok(buffer)
    }

    fn top_up_buffer(
        buffer: &mut Vec<Vec<u8>>,
        chunks: usize,
        min_chunks: usize,
    ) -> Result<(), EnclaveError> {
        for i in buffer.len()..chunks {
            let mut kb: Vec<u8> = Vec::new();
            match kb.try_reserve_exact(1024) {
                Ok(_) => { /* continue */ }
                Err(_err) => {
                    if i > min_chunks {
                        break;
                    } else {
                        return Err(EnclaveError::MemorySafetyAllocationError);
                    }
                }
            };
            buffer.push(kb)
        }
        Ok(())
    }

    // Reallocate the buffer, use this after a successful unwind
    pub fn restore(&mut self) -> Result<(), EnclaveError> {
        if self.buffer.capacity() < self.chunks {
            SafetyBuffer::top_up_buffer(&mut self.buffer, self.chunks, self.min_chunks)?;
        }
        Ok(())
    }
}

lazy_static! {
    /// SAFETY_BUFFER is a 4 MiB of SafetyBuffer. This is twice the bare minimum to unwind after
    /// a best-case OOM event. thanks to the recursion limit on queries, together with other memory
    /// limits, we don't expect to hit OOM, and this mechanism remains in place just in case.
    /// 2 MiB is the minimum allowed buffer. If we don't succeed to allocate 2 MiB, we throw a panic,
    /// if we do succeed to allocate 2 MiB but less than 4 MiB than we move on and will try to allocate
    /// the rest on the next entry to the enclave.
    static ref SAFETY_BUFFER: SgxMutex<SafetyBuffer> = SgxMutex::new(SafetyBuffer::new(4 * 1024, 2 * 1024));
}

thread_local! {
    static OOM_HAPPENED: AtomicBool = AtomicBool::new(false);
}

#[cfg(all(not(feature = "production"), not(feature = "query-only")))]
fn enable_backtraces() {
    let _ = backtrace::enable_backtrace("librust_cosmwasm_enclave.signed.so", PrintFormat::Full);
}

#[cfg(all(not(feature = "production"), feature = "query-only"))]
fn enable_backtraces() {
    let _ = backtrace::enable_backtrace(
        "librust_cosmwasm_query_enclave.signed.so",
        PrintFormat::Full,
    );
}

#[cfg(feature = "production")]
fn enable_backtraces() {}

fn oom_handler(layout: std::alloc::Layout) {
    OOM_HAPPENED.with(|oom_happened| oom_happened.store(true, Ordering::SeqCst));

    {
        SAFETY_BUFFER.lock().unwrap().clear();
    }

    panic!(
        "SGX: Memory allocation of {} bytes failed. Trying to recover...\n",
        layout.size()
    );
}

pub fn register_oom_handler() -> Result<(), EnclaveError> {
    enable_backtraces();

    {
        SAFETY_BUFFER.lock().unwrap().restore()?;
    }

    get_then_clear_oom_happened();

    std::alloc::set_alloc_error_hook(oom_handler);

    Ok(())
}

pub fn get_then_clear_oom_happened() -> bool {
    OOM_HAPPENED.with(|oom_happened| oom_happened.swap(false, Ordering::SeqCst))
}

pub fn restore_safety_buffer() -> Result<(), EnclaveError> {
    std::alloc::take_alloc_error_hook();
    let restored = SAFETY_BUFFER.lock().unwrap().restore();
    std::alloc::set_alloc_error_hook(oom_handler);
    restored
}
