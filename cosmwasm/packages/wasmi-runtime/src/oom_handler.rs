use core::sync::atomic::{AtomicBool, Ordering};

// This is thread_local to prevent race conditions in case multiple
// threads are running while OOM happens
// Mutex won't work because you cannot hold the Mutex between throwing
// and catching the panic
static OOM_HAPPANED: AtomicBool = AtomicBool::new(false);

pub fn register_oom_handler() {
    return_and_clear_oom_happened();
    std::alloc::set_alloc_error_hook(|layout| {
        OOM_HAPPANED.store(true, Ordering::SeqCst);
        panic!("memory allocation of {} bytes failed\n", layout.size());
    });
}

pub fn return_and_clear_oom_happened() -> bool {
    OOM_HAPPANED.swap(false, Ordering::SeqCst)
}
