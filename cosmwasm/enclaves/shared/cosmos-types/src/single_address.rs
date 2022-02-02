use enclave_crypto::secp256k1::{Secp256k1PubKey, SECP256K1_PREFIX};
use log::warn;

use super::traits::CosmosAminoPubkey;
use enclave_cosmwasm_types::types::CanonicalAddr;
use enclave_crypto::hash::ripemd::ripemd160;
use enclave_crypto::hash::sha::sha_256;

impl CosmosAminoPubkey for Secp256k1PubKey {
    fn get_address(&self) -> CanonicalAddr {
        // This reference describes how this should be derived:
        // https://github.com/tendermint/spec/blob/743a65861396e36022b2704e4383198b42c9cfbe/spec/blockchain/encoding.md#secp256k1
        // https://docs.tendermint.com/v0.32/spec/blockchain/encoding.html#secp256k1
        // This was updated in a later version of tendermint:
        // https://github.com/tendermint/spec/blob/32b811a1fb6e8b40bae270339e31a8bc5e8dea31/spec/core/encoding.md#secp256k1
        // https://docs.tendermint.com/v0.33/spec/core/encoding.html#secp256k1
        // but Cosmos kept the old algorithm

        let hash1 = sha_256(&self.0);
        let hash2 = ripemd160(&hash1);

        CanonicalAddr::from_vec(hash2.to_vec())
    }

    fn amino_bytes(&self) -> Vec<u8> {
        // Amino encoding here is basically: prefix | leb128 encoded length | ..bytes..
        let mut encoded = Vec::new();
        encoded.extend_from_slice(&SECP256K1_PREFIX);

        // Length may be more than 1 byte and it is protobuf encoded
        let mut length = Vec::new();

        // This line can't fail since it could only fail if `length` does not have sufficient capacity to encode
        if prost::encode_length_delimiter(self.0.len(), &mut length).is_err() {
            warn!(
                "Could not encode length delimiter: {:?}. This should not happen",
                self.0.len()
            );
            return vec![];
        }

        encoded.extend_from_slice(&length);
        encoded.extend_from_slice(&self.0);

        encoded
    }
}
