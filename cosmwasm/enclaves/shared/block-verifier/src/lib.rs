#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;

extern crate sgx_types;

pub mod r#const;
pub mod ecalls;

use lazy_static::lazy_static;

use tendermint_light_client_verifier::types::UntrustedBlockState;
use tendermint_light_client_verifier::{ProdVerifier, Verdict};

//
lazy_static! {
    static ref verifier: ProdVerifier = ProdVerifier::default();
}

pub fn verify_block(untrusted_block: &UntrustedBlockState) -> bool {
    match verifier.verify_commit(untrusted_block) {
        Verdict::Success => true,
        Verdict::NotEnoughTrust(_) => false,
        Verdict::Invalid(_) => false,
    }
}

// pub fn create_untrusted_block<'a>(
//     header: Header,
//     commit: Commit,
//     validator_set: &'a ValidatorSet,
//     next_validator_set: Option<&'a ValidatorSet>,
// ) -> UntrustedBlockState<'a> {
// }

#[cfg(test)]
mod tests {
    #[test]
    fn it_works() {
        let result = 2 + 2;
        assert_eq!(result, 4);
    }
}
