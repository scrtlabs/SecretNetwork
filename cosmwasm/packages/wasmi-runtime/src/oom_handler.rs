use core::sync::atomic::{AtomicBool, Ordering};
use lazy_static::lazy_static;
use std::sync::SgxMutex;

/// SafetyBuffer is meant to occupy space on the heap, so when a memory
/// allocation fails we will free this buffer to allow safe panic unwinding
/// This is needed because while unwinding from panic some destructors try
/// to allocate more memory which causes a double fault. This way we can
/// make sure the unwind process has enough free memory to work properly.
struct SafetyBuffer {
    length: usize,
    capacity: usize,
    buffer: *mut u8,
}

impl SafetyBuffer {
    /// Allocate `length` bytes on the heap
    pub fn new(length: usize) -> Self {
        let mut buffer: Vec<u8> = vec![0; length];
        buffer[length - 1] = 1;
        let ptr = buffer.as_mut_ptr();
        let capacity = buffer.capacity();
        std::mem::forget(buffer);
        SafetyBuffer {
            length,
            capacity,
            buffer: ptr,
        }
    }

    /// Free the buffer to allow panic to safely unwind
    pub fn clear(&mut self) {
        let buffer = unsafe { Vec::<u8>::from_raw_parts(self.buffer, self.length, self.capacity) };
        drop(buffer);
        self.buffer = std::ptr::null_mut();
    }

    // Reallocate the buffer, use this after a successful unwind
    pub fn restore(&mut self) {
        if self.buffer.is_null() {
            let mut buffer: Vec<u8> = vec![0; self.length];
            buffer[self.length - 1] = 1;
            self.buffer = buffer.as_mut_ptr();
            self.capacity = buffer.capacity();
            std::mem::forget(buffer);
        }
    }
}

unsafe impl Send for SafetyBuffer {}

lazy_static! {
    /// SAFETY_BUFFER is a 32 MiB of SafetyBuffer. This occupying 50% of available memory
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
