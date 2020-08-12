use core::sync::atomic::{AtomicBool, Ordering};
use lazy_static::lazy_static;
use std::sync::SgxMutex;

pub struct SafetyBuffer {
    size: usize,
    buffer: Vec<u8>,
}

impl SafetyBuffer {
    pub fn new(size: usize) -> Self {
        let mut buffer: Vec<u8> = vec![0; size];
        buffer[size - 1] = 1;
        SafetyBuffer { size, buffer }
    }

    pub fn clear(&mut self) {
        self.buffer = vec![];
    }

    pub fn restore(&mut self) {
        if self.buffer.capacity() < self.size {
            self.buffer = vec![0; self.size];
            self.buffer[self.size - 1] = 1;
        }
    }
}

lazy_static! {
    pub static ref SAFETY_BUFFER: SgxMutex<SafetyBuffer> = SgxMutex::new(SafetyBuffer::new(2 * 1024 * 1204 /* 1 MiB */));
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
        panic!("SGX: Memory allocation failed. Trying to recover...\n");
    });
}

pub fn get_then_clear_oom_happened() -> bool {
    OOM_HAPPANED.swap(false, Ordering::SeqCst)
}

pub fn restore_safety_buffer() {
    SAFETY_BUFFER.lock().unwrap().restore()
}
