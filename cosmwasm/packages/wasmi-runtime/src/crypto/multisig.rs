use log::*;

use crate::cosmwasm::types::{CanonicalAddr, PubKeyKind};
use crate::crypto::traits::PubKey;
use crate::crypto::CryptoError;
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct MultisigThresholdPubKey {
    threshold: usize,
    pubkeys: Vec<PubKeyKind>,
}

impl PubKey for MultisigThresholdPubKey {
    fn get_address(&self) -> CanonicalAddr {
        unimplemented!()
    }

    fn as_bytes(&self) -> Vec<u8> {
        unimplemented!()
    }

    fn verify_bytes(&self, bytes: &[u8], sig: &[u8]) -> Result<(), CryptoError> {
        trace!("verifying multisig");
        let signatures = decode_multisig_signature(sig)?;

        if signatures.len() < self.threshold || signatures.len() > self.pubkeys.len() {
            error!(
                "Wrong number of signatures! min expected: {:?}, max expected: {:?}, provided: {:?}",
                self.threshold,
                self.pubkeys.len(),
                signatures.len()
            );
            return Err(CryptoError::VerificationError);
        }

        let mut verified_counter = 0;

        for current_sig in &signatures {
            trace!("Checking sig: {:?}", current_sig);
            // TODO: can we somehow easily skip already verified signatures?
            for current_pubkey in &self.pubkeys {
                // This technically support that one of the multisig signers is a multisig itself
                let result = current_pubkey.verify_bytes(bytes, &current_sig);

                if result.is_ok() {
                    verified_counter += 1;
                    break;
                }
            }
        }

        if verified_counter < signatures.len() {
            error!("Failed to verify some or all signatures");
            Err(CryptoError::VerificationError)
        } else {
            Ok(())
        }
    }
}

type MultisigSignature = Vec<Vec<u8>>;

fn decode_multisig_signature(raw_blob: &[u8]) -> Result<MultisigSignature, CryptoError> {
    trace!("decoding blob: {:?}", raw_blob);
    let blob_size = raw_blob.len();
    if blob_size < 8 {
        error!("Multisig signature too short. decoding failed!");
        return Err(CryptoError::ParsingError);
    }

    let mut signatures: MultisigSignature = vec![];

    let mut idx: usize = 7;
    while let Some(curr_blob_window) = raw_blob.get(idx..) {
        if curr_blob_window.is_empty() {
            break;
        }

        trace!("while letting with {:?}", curr_blob_window);
        trace!("blob len is {:?} idx is: {:?}", raw_blob.len(), idx);
        let current_sig_prefix = curr_blob_window[0];

        if current_sig_prefix != 0x12 {
            error!("Multisig signature wrong prefix. decoding failed!");
            return Err(CryptoError::ParsingError);
        } else if let Some(current_sig_len) = curr_blob_window.get(1) {
            trace!("sig len is: {:?}", current_sig_len);
            if let Some(current_sig) = curr_blob_window.get(2..(*current_sig_len as usize) + 2) {
                signatures.push((&current_sig).to_vec());
                idx += 2 + (*current_sig_len as usize); // prefix_byte + length_byte + len(sig)
            } else {
                error!("Multisig signature malformed. decoding failed!");
                return Err(CryptoError::ParsingError);
            }
        } else {
            error!("Multisig signature malformed. decoding failed!");
            return Err(CryptoError::ParsingError);
        }
    }

    if signatures.is_empty() {
        error!("Multisig signature empty. decoding failed!");
        return Err(CryptoError::ParsingError);
    }

    Ok(signatures)
}
