use lazy_static::lazy_static;
use log::debug;

use tendermint_light_client_verifier::types::UntrustedBlockState;
use tendermint_light_client_verifier::{ProdVerifier, Verdict};

#[cfg(feature = "verify-validator-whitelist")]
use crate::validator_whitelist;

lazy_static! {
    static ref VERIFIER: ProdVerifier = ProdVerifier::default();
}

pub fn verify_block(untrusted_block: &UntrustedBlockState) -> bool {
    // #[cfg(feature = "verify-validator-whitelist")]
    // if !validator_whitelist::whitelisted_validators_in_block(untrusted_block) {
        // debug!("Error verifying validators in block");
        // return false;
    // }

    match VERIFIER.verify_commit(untrusted_block) {
        Verdict::Success => true,
        Verdict::NotEnoughTrust(_) => {
            debug!("Error verifying header - not enough trust");
            false
        }
        Verdict::Invalid(e) => {
            debug!("Error verifying header - invalid block header: {:?}", e);
            false
        }
    }
}
