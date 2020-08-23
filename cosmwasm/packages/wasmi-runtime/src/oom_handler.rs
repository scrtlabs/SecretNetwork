use core::sync::atomic::{AtomicBool, Ordering};
use lazy_static::lazy_static;
use std::sync::SgxMutex;

/// SafetyBuffer is meant to occupy space on the heap, so when a memory
/// allocation fails we will free this buffer to allow safe panic unwinding
/// This is needed because while unwinding from panic some destructors try
/// to allocate more memory which causes a double fault. This way we can
/// make sure the unwind process has enough free memory to work properly.
struct SafetyBuffer {
    chunks: usize,
    buffer: Vec<Vec<u8>>,
}

impl SafetyBuffer {
    /// Allocate `chunks` KiB on the heap
    pub fn new(chunks: usize) -> Self {
        SafetyBuffer {
            chunks,
            buffer: SafetyBuffer::build_buffer(chunks),
        }
    }

    /// Free the buffer to allow panic to safely unwind
    pub fn clear(&mut self) {
        self.buffer = Vec::new();
    }

    fn build_buffer(chunks: usize) -> Vec<Vec<u8>> {
        let mut buffer: Vec<Vec<u8>> = Vec::with_capacity(chunks);
        for _i in 0..chunks {
            let kb: Vec<u8> = Vec::with_capacity(1024);
            buffer.push(kb)
        }
        buffer
    }

    // Reallocate the buffer, use this after a successful unwind
    pub fn restore(&mut self) {
        if self.buffer.capacity() < self.chunks {
            self.buffer = SafetyBuffer::build_buffer(self.chunks);
        }
    }
}

lazy_static! {
    /// SAFETY_BUFFER is a 32 MiB of SafetyBuffer. This occupying 50% of available memory
    /// to be extra sure this is enough.
    static ref SAFETY_BUFFER: SgxMutex<SafetyBuffer> = SgxMutex::new(SafetyBuffer::new(32 * 1024));
}

static OOM_HAPPANED: AtomicBool = AtomicBool::new(false);
use std::backtrace::{self, PrintFormat};

pub fn register_oom_handler() {
    let _ = backtrace::enable_backtrace("librust_cosmwasm_enclave.signed.so", PrintFormat::Full);

    {
        SAFETY_BUFFER.lock().unwrap().restore();
    }

    get_then_clear_oom_happened();

    std::alloc::set_alloc_error_hook(|layout| {
        OOM_HAPPANED.store(true, Ordering::SeqCst);

        {
            SAFETY_BUFFER.lock().unwrap().clear();
        }

        panic!(
            "SGX: Memory allocation of {} bytes failed. Trying to recover...\n",
            layout.size()
        );
    });
}

pub fn get_then_clear_oom_happened() -> bool {
    OOM_HAPPANED.swap(false, Ordering::SeqCst)
}

pub fn restore_safety_buffer() {
    SAFETY_BUFFER.lock().unwrap().restore()
}
