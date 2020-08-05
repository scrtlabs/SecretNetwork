pub use attestation::create_attestation_certificate;
pub use offchain::{ecall_get_attestation_report, ecall_init_bootstrap, ecall_init_node};
pub use onchain::ecall_authenticate_new_node;

mod attestation;
mod cert;
mod hex;
mod offchain;
mod onchain;
mod report;

mod seed_exchange;

#[cfg(feature = "test")]
pub mod tests {
    use super::*;
    use crate::count_failures;

    pub fn run_tests() {
        println!();
        let mut failures = 0;

        count_failures!(failures, {
            // attestation::tests::test_create_attestation_certificate();
        });

        if failures != 0 {
            panic!("{}: {} tests failed", file!(), failures);
        }
    }
}
