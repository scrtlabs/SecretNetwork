#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;

extern crate sgx_types;

pub mod r#const;
pub mod ecalls;

#[cfg(all(feature = "SGX_MODE_HW", feature = "production", not(feature = "test")))]
pub mod validator_whitelist;
pub mod storage;
mod cosmos;

use lazy_static::lazy_static;
use log::debug;

use tendermint_light_client_verifier::types::UntrustedBlockState;
use tendermint_light_client_verifier::{ProdVerifier, Verdict};

lazy_static! {
    static ref VERIFIER: ProdVerifier = ProdVerifier::default();
}

pub fn verify_block(untrusted_block: &UntrustedBlockState) -> bool {

    #[cfg(all(feature = "SGX_MODE_HW", feature = "production", not(feature = "test")))]
    if !whitelisted_validators_in_block(untrusted_block) {
        debug!("Error verifying validators in block");
        return false;
    }

    match VERIFIER.verify_commit(untrusted_block) {
        Verdict::Success => true,
        Verdict::NotEnoughTrust(_) => {
            debug!("Error verifying header - not enough trust");
            false
        },
        Verdict::Invalid(e) => {
            debug!("Error verifying header - invalid block header: {:?}", e);
            false
        },
    }
}


#[cfg(test)]
mod tests {
    #[test]
    fn it_works() {
        let result = 2 + 2;
        assert_eq!(result, 4);
    }
}
