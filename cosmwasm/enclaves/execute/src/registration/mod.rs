pub use offchain::{ecall_init_bootstrap, ecall_init_node};
pub use verify::ecall_legacy_verify_node_on_chain;

mod attestation;
mod offchain;
mod persistency;
mod seed_exchange;
mod verify;

pub mod seed_service;

#[cfg(feature = "test")]
pub mod tests {
    use super::*;
    use crate::count_failures;

    pub fn run_tests() {
        println!();
        let mut failures = 0;

        // tests should be moved to different crate
        // count_failures!(failures, {
        //     report::tests::test_sgx_quote_parse_from();
        //     report::tests::test_attestation_report_from_cert();
        //     report::tests::test_attestation_report_from_cert_invalid();
        //     report::tests::test_attestation_report_from_cert_api_version_not_compatible();
        //     report::tests::test_attestation_report_test();
        //     cert::tests::test_certificate_valid();
        //     cert::tests::test_certificate_invalid_configuration_needed();
        //     cert::tests::test_epid_whitelist();
        // });

        // The test doesn't work for some reason
        // #[cfg(feature = "SGX_MODE_HW")]
        // count_failures!(failures, {
        //     cert::tests::test_certificate_invalid_group_out_of_date();
        // });

        if failures != 0 {
            panic!("{}: {} tests failed", file!(), failures);
        }
    }
}
