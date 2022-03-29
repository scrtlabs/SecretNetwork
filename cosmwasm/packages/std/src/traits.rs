use serde::{de::DeserializeOwned, Serialize};

use crate::addresses::{CanonicalAddr, HumanAddr};
use crate::coins::Coin;
use crate::encoding::Binary;
use crate::errors::{
    RecoverPubkeyError, SigningError, StdError, StdResult, SystemResult, VerificationError,
};
#[cfg(feature = "iterator")]
use crate::iterator::{Order, KV};
use crate::query::{AllBalanceResponse, BalanceResponse, BankQuery, QueryRequest};
#[cfg(feature = "staking")]
use crate::query::{
    AllDelegationsResponse, BondedDenomResponse, Delegation, DelegationResponse, FullDelegation,
    StakingQuery, Validator, ValidatorsResponse,
};
use crate::serde::{from_binary, to_vec};
use crate::types::Empty;

/// Holds all external dependencies of the contract.
/// Designed to allow easy dependency injection at runtime.
/// This cannot be copied or cloned since it would behave differently
/// for mock storages and a bridge storage in the VM.
pub struct Extern<S: Storage, A: Api, Q: Querier> {
    pub storage: S,
    pub api: A,
    pub querier: Q,
}

impl<S: Storage, A: Api, Q: Querier> Extern<S, A, Q> {
    /// change_querier is a helper mainly for test code when swapping out the Querier
    /// from the auto-generated one from mock_dependencies. This changes the type of
    /// Extern so replaces requires some boilerplate.
    pub fn change_querier<T: Querier, F: Fn(Q) -> T>(self, transform: F) -> Extern<S, A, T> {
        Extern {
            storage: self.storage,
            api: self.api,
            querier: transform(self.querier),
        }
    }
}

/// ReadonlyStorage is access to the contracts persistent data store
pub trait ReadonlyStorage {
    /// Returns None when key does not exist.
    /// Returns Some(Vec<u8>) when key exists.
    ///
    /// Note: Support for differentiating between a non-existent key and a key with empty value
    /// is not great yet and might not be possible in all backends. But we're trying to get there.
    fn get(&self, key: &[u8]) -> Option<Vec<u8>>;

    #[cfg(feature = "iterator")]
    /// Allows iteration over a set of key/value pairs, either forwards or backwards.
    ///
    /// The bound `start` is inclusive and `end` is exclusive.
    ///
    /// If `start` is lexicographically greater than or equal to `end`, an empty range is described, mo matter of the order.
    fn range<'a>(
        &'a self,
        start: Option<&[u8]>,
        end: Option<&[u8]>,
        order: Order,
    ) -> Box<dyn Iterator<Item = KV> + 'a>;
}

// Storage extends ReadonlyStorage to give mutable access
pub trait Storage: ReadonlyStorage {
    fn set(&mut self, key: &[u8], value: &[u8]);
    /// Removes a database entry at `key`.
    ///
    /// The current interface does not allow to differentiate between a key that existed
    /// before and one that didn't exist. See https://github.com/CosmWasm/cosmwasm/issues/290
    fn remove(&mut self, key: &[u8]);
}

/// Api are callbacks to system functions defined outside of the wasm modules.
/// This is a trait to allow Mocks in the test code.
///
/// Currently it just supports address conversion, we could add eg. crypto functions here.
/// These should all be pure (stateless) functions. If you need state, you probably want
/// to use the Querier.
///
/// We can use feature flags to opt-in to non-essential methods
/// for backwards compatibility in systems that don't have them all.
pub trait Api: Copy + Clone + Send {
    /// Takes a human readable address and returns a canonical binary representation of it.
    /// This can be used when a compact fixed length representation is needed.
    fn canonical_address(&self, human: &HumanAddr) -> StdResult<CanonicalAddr>;

    /// Takes a canonical address and returns a human readble address.
    /// This is the inverse of [`canonical_address`].
    ///
    /// [`canonical_address`]: Api::canonical_address
    fn human_address(&self, canonical: &CanonicalAddr) -> StdResult<HumanAddr>;

    /// ECDSA secp256k1 signature verification.
    ///
    /// This function verifies message hashes (hashed unsing SHA-256) against a signature,
    /// with the public key of the signer, using the secp256k1 elliptic curve digital signature
    /// parametrization / algorithm.
    ///
    /// The signature and public key are in "Cosmos" format:
    /// - signature:  Serialized "compact" signature (64 bytes).
    /// - public key: [Serialized according to SEC 2](https://www.oreilly.com/library/view/programming-bitcoin/9781492031482/ch04.html)
    fn secp256k1_verify(
        &self,
        message_hash: &[u8],
        signature: &[u8],
        public_key: &[u8],
    ) -> Result<bool, VerificationError>;

    /// Recovers a public key from a message hash and a signature.
    ///
    /// This is required when working with Ethereum where public keys
    /// are not stored on chain directly.
    ///
    /// `recovery_param` must be 0 or 1. The values 2 and 3 are unsupported by this implementation,
    /// which is the same restriction as Ethereum has (https://github.com/ethereum/go-ethereum/blob/v1.9.25/internal/ethapi/api.go#L466-L469).
    /// All other values are invalid.
    ///
    /// Returns the recovered pubkey in compressed form, which can be used
    /// in secp256k1_verify directly.
    fn secp256k1_recover_pubkey(
        &self,
        message_hash: &[u8],
        signature: &[u8],
        recovery_param: u8,
    ) -> Result<Vec<u8>, RecoverPubkeyError>;

    /// EdDSA ed25519 signature verification.
    ///
    /// This function verifies messages against a signature, with the public key of the signer,
    /// using the ed25519 elliptic curve digital signature parametrization / algorithm.
    ///
    /// The maximum currently supported message length is 4096 bytes.
    /// The signature and public key are in [Tendermint](https://docs.tendermint.com/v0.32/spec/blockchain/encoding.html#public-key-cryptography)
    /// format:
    /// - signature: raw ED25519 signature (64 bytes).
    /// - public key: raw ED25519 public key (32 bytes).
    fn ed25519_verify(
        &self,
        message: &[u8],
        signature: &[u8],
        public_key: &[u8],
    ) -> Result<bool, VerificationError>;

    /// Performs batch Ed25519 signature verification.
    ///
    /// Batch verification asks whether all signatures in some set are valid, rather than asking whether
    /// each of them is valid. This allows sharing computations among all signature verifications,
    /// performing less work overall, at the cost of higher latency (the entire batch must complete),
    /// complexity of caller code (which must assemble a batch of signatures across work-items),
    /// and loss of the ability to easily pinpoint failing signatures.
    ///
    /// This batch verification implementation is adaptive, in the sense that it detects multiple
    /// signatures created with the same verification key, and automatically coalesces terms
    /// in the final verification equation.
    ///
    /// In the limiting case where all signatures in the batch are made with the same verification key,
    /// coalesced batch verification runs twice as fast as ordinary batch verification.
    ///
    /// Three Variants are suppported in the input for convenience:
    ///  - Equal number of messages, signatures, and public keys: Standard, generic functionality.
    ///  - One message, and an equal number of signatures and public keys: Multiple digital signature
    /// (multisig) verification of a single message.
    ///  - One public key, and an equal number of messages and signatures: Verification of multiple
    /// messages, all signed with the same private key.
    ///
    /// Any other variants of input vectors result in an error.
    ///
    /// Notes:
    ///  - The "one-message, with zero signatures and zero public keys" case, is considered the empty
    /// case.
    ///  - The "one-public key, with zero messages and zero signatures" case, is considered the empty
    /// case.
    ///  - The empty case (no messages, no signatures and no public keys) returns true.
    fn ed25519_batch_verify(
        &self,
        messages: &[&[u8]],
        signatures: &[&[u8]],
        public_keys: &[&[u8]],
    ) -> Result<bool, VerificationError>;

    /// ECDSA secp256k1 signing.
    ///
    /// This function signs a message with a private key using the secp256k1 elliptic curve digital signature parametrization / algorithm.
    ///
    /// - message: Arbitrary message.
    /// - private key: Raw secp256k1 private key (32 bytes)
    fn secp256k1_sign(&self, message: &[u8], private_key: &[u8]) -> Result<Vec<u8>, SigningError>;

    /// EdDSA Ed25519 signing.
    ///
    /// This function signs a message with a private key using the ed25519 elliptic curve digital signature parametrization / algorithm.
    ///
    /// - message: Arbitrary message.
    /// - private key: Raw ED25519 private key (32 bytes)
    fn ed25519_sign(&self, message: &[u8], private_key: &[u8]) -> Result<Vec<u8>, SigningError>;
}

/// A short-hand alias for the two-level query result (1. accessing the contract, 2. executing query in the contract)
pub type QuerierResult = SystemResult<StdResult<Binary>>;

pub trait Querier {
    /// raw_query is all that must be implemented for the Querier.
    /// This allows us to pass through binary queries from one level to another without
    /// knowing the custom format, or we can decode it, with the knowledge of the allowed
    /// types. People using the querier probably want one of the simpler auto-generated
    /// helper methods
    fn raw_query(&self, bin_request: &[u8]) -> QuerierResult;

    /// query is a shorthand for custom_query when we are not using a custom type,
    /// this allows us to avoid specifying "Empty" in all the type definitions.
    fn query<T: DeserializeOwned>(&self, request: &QueryRequest<Empty>) -> StdResult<T> {
        self.custom_query(request)
    }

    /// Makes the query and parses the response. Also handles custom queries,
    /// so you need to specify the custom query type in the function parameters.
    /// If you are no using a custom query, just use `query` for easier interface.
    ///
    /// Any error (System Error, Error or called contract, or Parse Error) are flattened into
    /// one level. Only use this if you don't need to check the SystemError
    /// eg. If you don't differentiate between contract missing and contract returned error
    fn custom_query<T: Serialize, U: DeserializeOwned>(
        &self,
        request: &QueryRequest<T>,
    ) -> StdResult<U> {
        let raw = match to_vec(request) {
            Ok(raw) => raw,
            Err(e) => {
                return Err(StdError::generic_err(format!(
                    "Serializing QueryRequest: {}",
                    e
                )))
            }
        };
        match self.raw_query(&raw) {
            Err(sys) => Err(StdError::generic_err(format!(
                "Querier system error: {}",
                sys
            ))),
            Ok(Err(err)) => Err(err),
            // in theory we would process the response, but here it is the same type, so just pass through
            Ok(Ok(res)) => from_binary(&res),
        }
    }

    fn query_balance<U: Into<HumanAddr>>(&self, address: U, denom: &str) -> StdResult<Coin> {
        let request = BankQuery::Balance {
            address: address.into(),
            denom: denom.to_string(),
        }
        .into();
        let res: BalanceResponse = self.query(&request)?;
        Ok(res.amount)
    }

    fn query_all_balances<U: Into<HumanAddr>>(&self, address: U) -> StdResult<Vec<Coin>> {
        let request = BankQuery::AllBalances {
            address: address.into(),
        }
        .into();
        let res: AllBalanceResponse = self.query(&request)?;
        Ok(res.amount)
    }

    #[cfg(feature = "staking")]
    fn query_validators(&self) -> StdResult<Vec<Validator>> {
        let request = StakingQuery::Validators {}.into();
        let res: ValidatorsResponse = self.query(&request)?;
        Ok(res.validators)
    }

    #[cfg(feature = "staking")]
    fn query_bonded_denom(&self) -> StdResult<String> {
        let request = StakingQuery::BondedDenom {}.into();
        let res: BondedDenomResponse = self.query(&request)?;
        Ok(res.denom)
    }

    #[cfg(feature = "staking")]
    fn query_all_delegations<U: Into<HumanAddr>>(
        &self,
        delegator: U,
    ) -> StdResult<Vec<Delegation>> {
        let request = StakingQuery::AllDelegations {
            delegator: delegator.into(),
        }
        .into();
        let res: AllDelegationsResponse = self.query(&request)?;
        Ok(res.delegations)
    }

    #[cfg(feature = "staking")]
    fn query_delegation<U: Into<HumanAddr>>(
        &self,
        delegator: U,
        validator: U,
    ) -> StdResult<Option<FullDelegation>> {
        let request = StakingQuery::Delegation {
            delegator: delegator.into(),
            validator: validator.into(),
        }
        .into();
        let res: DelegationResponse = self.query(&request)?;
        Ok(res.delegation)
    }
}
