#[cfg(all(feature = "debug-print", target_arch = "wasm32"))]
mod inner {
    use crate::memory::{build_region, Region};

    extern "C" {
        fn debug_print(text: u32);
    }

    pub fn _debug_print<S: AsRef<str>>(message: S) {
        let message_ref = message.as_ref();
        let region = build_region(message_ref.as_bytes());
        let message_ptr = &*region as *const Region as u32;
        unsafe { debug_print(message_ptr) }
    }
}

#[cfg(all(feature = "debug-print", target_arch = "wasm32"))]
pub use inner::_debug_print as debug_print;

#[cfg(not(all(feature = "debug-print", target_arch = "wasm32")))]
#[inline(always)]
pub fn debug_print<S: AsRef<str>>(_message: S) {}

#[macro_export]
macro_rules! debug_print {
    ($($tts: tt)*) => {
        $crate::debug_print(&format!($($tts)*))
    };
}
