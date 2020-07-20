// This is thread_local to prevent race conditions in case multiple
// threads are running while OOM happens
// Mutex won't work because you cannot hold the Mutex between throwing
// and catching the panic
thread_local! {
    static OOM_HAPPANED: std::cell::RefCell<bool> = std::cell::RefCell::new(false);
}

pub fn register_oom_handler() {
    return_and_clear_oom_happened();
    std::alloc::set_alloc_error_hook(|layout| {
        OOM_HAPPANED.with(|oom| oom.replace(true));
        panic!("memory allocation of {} bytes failed\n", layout.size());
    });
}

pub fn is_oom_happened() -> bool {
    OOM_HAPPANED.with(|oom| *oom.borrow())
}

pub fn return_and_clear_oom_happened() -> bool {
    OOM_HAPPANED.with(|oom| oom.replace(false))
}
