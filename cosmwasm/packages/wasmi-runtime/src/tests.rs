#[cfg(not(feature = "test"))]
#[no_mangle]
pub extern "C" fn ecall_run_tests() {
    println!("This enclave was not built for running tests.");
}

#[cfg(feature = "test")]
#[no_mangle]
pub extern "C" fn ecall_run_tests() {
    println!("Running tests!");
}
