#[cfg(not(feature = "test"))]
#[no_mangle]
pub extern "C" fn ecall_run_tests() {
    println!("This enclave was not built for running tests.");
}

#[cfg(feature = "test")]
mod tests {

    #[no_mangle]
    pub extern "C" fn ecall_run_tests() {
        println!("Running tests!");

        // crate::registration::tests::run_tests();
        crate::crypto::tests::run_tests();
        crate::wasm::tests::run_tests();
    }
}
