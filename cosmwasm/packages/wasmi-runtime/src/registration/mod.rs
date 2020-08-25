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
            report::tests::test_sgx_quote_parse_from();
            report::tests::test_attestation_report_from_cert();
            report::tests::test_attestation_report_from_cert_api_version_not_compatible();
            cert::tests::test_certificate_valid();
            cert::tests::test_certificate_invalid_configuration_needed();
        });

        if failures != 0 {
            panic!("{}: {} tests failed", file!(), failures);
        }
    }
}
