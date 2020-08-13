use core::sync::atomic::{AtomicBool, Ordering};
use lazy_static::lazy_static;
use std::sync::SgxMutex;

/// SafetyBuffer is meant to occupy space on the heap, so when a memory
/// allocation fails we will free this buffer to allow safe panic unwinding
/// This is needed because while unwinding from panic some destructors try
/// to allocate more memory which causes a double fault. This way we can
/// make sure the unwind process has enough free memory to work properly.
struct SafetyBuffer {
    size: usize,
    buffer: Vec<u8>,
}

impl SafetyBuffer {
    /// Allocate `size` bytes on the heap
    pub fn new(size: usize) -> Self {
        let mut buffer: Vec<u8> = vec![0; size];
        buffer[size - 1] = 1;
        SafetyBuffer { size, buffer }
    }

    /// Free the buffer to allow panic to safely unwind
    pub fn clear(&mut self) {
        self.buffer = vec![];
    }

    // Reallocate the buffer, use this after a successful unwind
    pub fn restore(&mut self) {
        if self.buffer.capacity() < self.size {
            self.buffer = vec![0; self.size];
            self.buffer[self.size - 1] = 1;
        }
    }
}

lazy_static! {
    /// SAFETY_BUFFER is a 2 MiB of SafetyBuffer. We should consider occupying 51% of available memory
    /// to be extra sure this is enough.
    static ref SAFETY_BUFFER: SgxMutex<SafetyBuffer> = SgxMutex::new(SafetyBuffer::new(2 * 1024 * 1204));
}

static OOM_HAPPANED: AtomicBool = AtomicBool::new(false);
use std::backtrace::{self, PrintFormat};

pub fn register_oom_handler() {
    let _ = backtrace::enable_backtrace("librust_cosmwasm_enclave.signed.so", PrintFormat::Full);

    {
        SAFETY_BUFFER.lock().unwrap().restore();
    }

    get_then_clear_oom_happened();

    std::alloc::set_alloc_error_hook(|_| {
        OOM_HAPPANED.store(true, Ordering::SeqCst);
        {
            SAFETY_BUFFER.lock().unwrap().clear();
        }
        panic!("SGX: Memory allocation failed. Trying to recover...\n");
    });
}

pub fn get_then_clear_oom_happened() -> bool {
    OOM_HAPPANED.swap(false, Ordering::SeqCst)
}

pub fn restore_safety_buffer() {
    SAFETY_BUFFER.lock().unwrap().restore()
}
