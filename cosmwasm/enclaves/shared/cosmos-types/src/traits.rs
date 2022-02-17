use enclave_cosmwasm_types::types::CanonicalAddr;

// https://github.com/tendermint/tendermint/blob/v0.33.3/crypto/crypto.go#L22
pub trait CosmosAminoPubkey: PartialEq {
    /// derive the canonical address for this public key
    fn get_address(&self) -> CanonicalAddr;
    /// Serialize this public key to the legacy Amino format
    fn amino_bytes(&self) -> Vec<u8>;
}
