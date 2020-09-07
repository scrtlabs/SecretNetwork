use log::*;

use crate::cosmwasm::encoding::Binary;
use crate::cosmwasm::types::{CanonicalAddr, PubKeyKind};
use crate::crypto::traits::PubKey;
use crate::crypto::CryptoError;

use serde::{Deserialize, Serialize};
use sha2::Digest;

const THRESHOLD_PREFIX: [u8; 5] = [34, 193, 247, 226, 8];
const GENERIC_PREFIX: u8 = 18;

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq)]
pub struct MultisigThresholdPubKey {
    threshold: u8,
    pubkeys: Vec<PubKeyKind>,
}

impl PubKey for MultisigThresholdPubKey {
    fn get_address(&self) -> CanonicalAddr {
        // Spec: https://docs.tendermint.com/master/spec/core/encoding.html#key-types
        // Multisig is undocumented, but we figured out it's the same as ed25519
        let address_bytes = &sha2::Sha256::digest(self.bytes().as_slice())[..20];

        CanonicalAddr(Binary::from(address_bytes))
    }

    fn bytes(&self) -> Vec<u8> {
        // Encoding for multisig is basically:
        // threshold_prefix | threshold | generic_prefix | encoded_pubkey_length | ...encoded_pubkey... | generic_prefix | encoded_pubkey_length | ...encoded_pubkey...
        let mut encoded: Vec<u8> = vec![];

        encoded.extend_from_slice(&THRESHOLD_PREFIX);
        encoded.push(self.threshold);

        for pubkey in &self.pubkeys {
            encoded.push(GENERIC_PREFIX);

            // Length may be more than 1 byte and it is protobuf encoded
            let mut length = Vec::<u8>::new();

            let pubkey_bytes = pubkey.bytes();
            // This line should never fail since it could only fail if `length` does not have sufficient capacity to encode
            if prost::encode_length_delimiter(pubkey_bytes.len(), &mut length).is_err() {
                warn!(
                    "Could not encode length delimiter: {:?}. This should not happen",
                    pubkey_bytes.len()
                );
                return vec![];
            }
            encoded.extend_from_slice(&length);
            encoded.extend_from_slice(&pubkey_bytes);
        }

        trace!("pubkey bytes are: {:?}", encoded);
        encoded
    }

    fn verify_bytes(&self, bytes: &[u8], sig: &[u8]) -> Result<(), CryptoError> {
        debug!("verifying multisig");
        trace!("Sign bytes are: {:?}", bytes);
        let signatures = decode_multisig_signature(sig)?;

        if signatures.len() < (self.threshold as usize) || signatures.len() > self.pubkeys.len() {
            warn!(
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
                trace!("Checking pubkey: {:?}", current_pubkey);
                // This technically support that one of the multisig signers is a multisig itself
                let result = current_pubkey.verify_bytes(bytes, &current_sig);

                if result.is_ok() {
                    verified_counter += 1;
                    break;
                }
            }
        }

        if verified_counter < signatures.len() {
            warn!("Failed to verify some or all signatures");
            Err(CryptoError::VerificationError)
        } else {
            debug!("Miltusig verified successfully");
            Ok(())
        }
    }
}

type MultisigSignature = Vec<Vec<u8>>;

fn decode_multisig_signature(raw_blob: &[u8]) -> Result<MultisigSignature, CryptoError> {
    trace!("decoding blob: {:?}", raw_blob);
    let blob_size = raw_blob.len();
    if blob_size < 8 {
        warn!("Multisig signature too short. decoding failed!");
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
            warn!("Multisig signature wrong prefix. decoding failed!");
            return Err(CryptoError::ParsingError);
        // The condition below can't fail because:
        // (1) curr_blob_window.get(1..) will return a Some(empty_slice) if curr_blob_window.len()=1
        // (2) At the beginning of the while loop we make sure curr_blob_window isn't empty, thus curr_blob_window.len() > 0
        // Therefore, no need for an else clause
        } else if let Some(sig_including_len) = curr_blob_window.get(1..) {
            // The condition below will take care of a case where `sig_including_len` is empty due
            // to curr_blob_window.get(), so no explicit check is needed here
            if let Ok(current_sig_len) = prost::decode_length_delimiter(sig_including_len) {
                let len_size = prost::length_delimiter_len(current_sig_len);

                trace!("sig len is: {:?}", current_sig_len);
                if let Some(raw_signature) =
                    sig_including_len.get(len_size..current_sig_len + len_size)
                {
                    signatures.push((&raw_signature).to_vec());
                    idx += 1 + len_size + current_sig_len; // prefix_byte + length_byte + len(sig)
                } else {
                    warn!("Multisig signature malformed. decoding failed!");
                    return Err(CryptoError::ParsingError);
                }
            } else {
                warn!("Multisig signature malformed. decoding failed!");
                return Err(CryptoError::ParsingError);
            }
        }
    }

    if signatures.is_empty() {
        warn!("Multisig signature empty. decoding failed!");
        return Err(CryptoError::ParsingError);
    }

    Ok(signatures)
}

#[cfg(feature = "test")]
pub mod tests_decode_multisig_signature {
    use crate::crypto::multisig::{decode_multisig_signature, MultisigSignature};

    pub fn test_decode_sig_sanity() {
        let sig: Vec<u8> = vec![
            0, 0, 0, 0, 0, 0, 0, 0x12, 10, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 0x12, 4, 1, 2, 3, 4,
        ];

        let result = decode_multisig_signature(sig.as_slice()).unwrap();
        assert_eq!(
            result,
            vec![vec![1, 2, 3, 4, 5, 6, 7, 8, 9, 10], vec![1, 2, 3, 4]],
            "Signature is: {:?} and result is: {:?}",
            sig,
            result
        )
    }

    pub fn test_decode_long_leb128() {
        let sig: Vec<u8> = vec![
            0, 0, 0, 0, 0, 0, 0, 0x12, 200, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
            0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
            0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
            0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
            0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
            0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
            0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
            0, 0, 0, 0, 0, 0, 0, 0, 0,
        ];

        let result = decode_multisig_signature(sig.as_slice()).unwrap();
        assert_eq!(
            result,
            vec![vec![
                0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
                0, 0, 0, 0,
            ]],
            "Signature is: {:?} and result is: {:?}",
            sig,
            result
        )
    }

    pub fn test_decode_wrong_long_leb128() {
        let malformed_sig: Vec<u8> = vec![
            0, 0, 0, 0, 0, 0, 0, 0x12, 205, 1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
        ];

        let result = decode_multisig_signature(malformed_sig.as_slice());
        assert!(
            result.is_err(),
            "Signature is: {:?} and result is: {:?}",
            malformed_sig,
            result
        );
    }

    pub fn test_decode_malformed_sig_only_prefix() {
        let malformed_sig: Vec<u8> = vec![0, 0, 0, 0, 0, 0, 0, 0x12];

        let result = decode_multisig_signature(malformed_sig.as_slice());
        assert!(
            result.is_err(),
            "Signature is: {:?} and result is: {:?}",
            malformed_sig,
            result
        );
    }

    pub fn test_decode_sig_length_zero() {
        let sig: Vec<u8> = vec![0, 0, 0, 0, 0, 0, 0, 0x12, 0];

        let result = decode_multisig_signature(sig.as_slice()).unwrap();
        let expected: Vec<Vec<u8>> = vec![vec![]];
        assert_eq!(
            result, expected,
            "Signature is: {:?} and result is: {:?}",
            sig, result
        )
    }

    pub fn test_decode_malformed_sig_wrong_length() {
        let malformed_sig: Vec<u8> = vec![0, 0, 0, 0, 0, 0, 0, 0x12, 10, 0, 0];

        let result = decode_multisig_signature(malformed_sig.as_slice());
        assert!(
            result.is_err(),
            "Signature is: {:?} and result is: {:?}",
            malformed_sig,
            result
        );
    }
}
