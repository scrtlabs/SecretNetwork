#[cfg(not(feature = "test"))]
#[no_mangle]
pub extern "C" fn ecall_run_tests() -> u32 {
    println!("This enclave was not built for running tests.");
    0
}

#[cfg(feature = "test")]
mod test {
    /// Catch failures like the standard test runner, and print similar information per test.
    /// Tests can only fail by panicking, not by returning a `Result` type.
    #[macro_export]
    macro_rules! count_failures {
        ( $counter: ident, { $($test: expr;)* } ) => {
            $(
                print!("test {} ... ", std::stringify!($test));
                match std::panic::catch_unwind(|| $test) {
                    Ok(_) => println!("ok"),
                    Err(_) => {
                        $counter += 1;
                        println!("FAILED");
                    }
                }
            )*
        }
    }

    #[no_mangle]
    pub extern "C" fn ecall_run_tests() -> u32 {
        println!("Running tests!");

        let mut failures = 0;

        count_failures!(failures, {
            enclave_contract_engine::tests::run_tests();
            enclave_cosmos_types::tests::run_tests();
            crate::registration::tests::run_tests();
            crate::logging_tests::run_tests();

            // example failing tests:
            // panic!("AAAAA");
            // panic!("BBBBB");
        });

        failures
    }
}
