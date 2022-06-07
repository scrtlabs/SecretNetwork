use std::vec::Vec;

use crate::addresses::{CanonicalAddr, HumanAddr};
use crate::encoding::Binary;
use crate::errors::{RecoverPubkeyError, SigningError, StdError, StdResult, VerificationError};
#[cfg(feature = "iterator")]
use crate::iterator::{Order, KV, Pair};
use crate::memory::{alloc, build_region, consume_region, encode_sections, Region, get_optional_region_address};
use crate::serde::from_slice;
use crate::traits::{Api, Querier, QuerierResult, Storage};

/// An upper bound for typical canonical address lengths (e.g. 20 in Cosmos SDK/Ethereum or 32 in Nano/Substrate)
const CANONICAL_ADDRESS_BUFFER_LENGTH: usize = 64;
/// An upper bound for typical human readable address formats (e.g. 42 for Ethereum hex addresses or 90 for bech32)
const HUMAN_ADDRESS_BUFFER_LENGTH: usize = 90;

// This interface will compile into required Wasm imports.
// A complete documentation those functions is available in the VM that provides them:
// https://github.com/CosmWasm/cosmwasm/blob/v0.14.1/packages/vm/src/instance.rs#L84-L201
extern "C" {
    fn db_read(key: u32) -> u32;
    fn db_write(key: u32, value: u32);
    fn db_remove(key: u32);

    // scan creates an iterator, which can be read by consecutive next() calls
    #[cfg(feature = "iterator")]
    fn db_scan(start_ptr: u32, end_ptr: u32, order: i32) -> u32;
    #[cfg(feature = "iterator")]
    fn db_next(iterator_id: u32) -> u32;

    fn addr_validate(source_ptr: u32) -> u32;
    fn addr_canonicalize(source_ptr: u32, destination_ptr: u32) -> u32;
    fn addr_humanize(source_ptr: u32, destination_ptr: u32) -> u32;

    fn debug(source_ptr: u32);

    /// Executes a query on the chain (import). Not to be confused with the
    /// query export, which queries the state of the contract.
    fn query_chain(request: u32) -> u32;

    /// Verifies message hashes against a signature with a public key, using the
    /// secp256k1 ECDSA parametrization.
    /// Returns 0 on verification success, 1 on verification failure, and values
    /// greater than 1 in case of error.
    fn secp256k1_verify(message_hash_ptr: u32, signature_ptr: u32, public_key_ptr: u32) -> u32;

    fn secp256k1_recover_pubkey(
        message_hash_ptr: u32,
        signature_ptr: u32,
        recovery_param: u32,
    ) -> u64;

    /// Verifies a message against a signature with a public key, using the
    /// ed25519 EdDSA scheme.
    /// Returns 0 on verification success, 1 on verification failure, and values
    /// greater than 1 in case of error.
    fn ed25519_verify(message_ptr: u32, signature_ptr: u32, public_key_ptr: u32) -> u32;

    /// Verifies a batch of messages against a batch of signatures and public keys, using the
    /// ed25519 EdDSA scheme.
    /// Returns 0 on verification success, 1 on verification failure, and values
    /// greater than 1 in case of error.
    fn ed25519_batch_verify(messages_ptr: u32, signatures_ptr: u32, public_keys_ptr: u32) -> u32;

    fn secp256k1_sign(messages_ptr: u32, private_key_ptr: u32) -> u64;

    fn ed25519_sign(messages_ptr: u32, private_key_ptr: u32) -> u64;
}

/// A stateless convenience wrapper around database imports provided by the VM.
/// This cannot be cloned as it would not copy any data. If you need to clone this, it indicates a flaw in your logic.
pub struct ExternalStorage {}

impl ExternalStorage {
    pub fn new() -> ExternalStorage {
        ExternalStorage {}
    }
}

impl Storage for ExternalStorage {
    fn get(&self, key: &[u8]) -> Option<Vec<u8>> {
        let key = build_region(key);
        let key_ptr = &*key as *const Region as u32;

        let read = unsafe { db_read(key_ptr) };
        if read == 0 {
            // key does not exist in external storage
            return None;
        }

        let value_ptr = read as *mut Region;
        let data = unsafe { consume_region(value_ptr) };
        Some(data)
    }

    fn set(&mut self, key: &[u8], value: &[u8]) {
        if value.is_empty() {
            panic!("TL;DR: Value must not be empty in Storage::set but in most cases you can use Storage::remove instead. Long story: Getting empty values from storage is not well supported at the moment. Some of our internal interfaces cannot differentiate between a non-existent key and an empty value. Right now, you cannot rely on the behaviour of empty values. To protect you from trouble later on, we stop here. Sorry for the inconvenience! We highly welcome you to contribute to CosmWasm, making this more solid one way or the other.");
        }

        // keep the boxes in scope, so we free it at the end (don't cast to pointers same line as build_region)
        let key = build_region(key);
        let key_ptr = &*key as *const Region as u32;
        let mut value = build_region(value);
        let value_ptr = &mut *value as *mut Region as u32;
        unsafe { db_write(key_ptr, value_ptr) };
    }

    fn remove(&mut self, key: &[u8]) {
        // keep the boxes in scope, so we free it at the end (don't cast to pointers same line as build_region)
        let key = build_region(key);
        let key_ptr = &*key as *const Region as u32;
        unsafe { db_remove(key_ptr) };
    }

    #[cfg(feature = "iterator")]
    fn range(
        &self,
        start: Option<&[u8]>,
        end: Option<&[u8]>,
        order: Order,
    ) -> Box<dyn Iterator<Item = Pair>> {
        // There is lots of gotchas on turning options into regions for FFI, thus this design
        // See: https://github.com/CosmWasm/cosmwasm/pull/509
        let start_region = start.map(build_region);
        let end_region = end.map(build_region);
        let start_region_addr = get_optional_region_address(&start_region.as_ref());
        let end_region_addr = get_optional_region_address(&end_region.as_ref());
        let iterator_id = unsafe { db_scan(start_region_addr, end_region_addr, order as i32) };
        let iter = ExternalIterator { iterator_id };
        Box::new(iter)
    }
}

#[cfg(feature = "iterator")]
/// ExternalIterator makes a call out to next.
/// We use the pointer to differentiate between multiple open iterators.
struct ExternalIterator {
    iterator_id: u32,
}

#[cfg(feature = "iterator")]
impl Iterator for ExternalIterator {
    type Item = Pair;

    fn next(&mut self) -> Option<Self::Item> {
        let next_result = unsafe { db_next(self.iterator_id) };
        let kv_region_ptr = next_result as *mut Region;
        let kv = unsafe { consume_region(kv_region_ptr) };
        let (key, value) = decode_sections2(kv);
        if key.len() == 0 {
            None
        } else {
            Some((key, value))
        }
    }
}

/// A stateless convenience wrapper around imports provided by the VM
#[derive(Copy, Clone)]
pub struct ExternalApi {}

impl ExternalApi {
    pub fn new() -> ExternalApi {
        ExternalApi {}
    }
}

impl Api for ExternalApi {
    fn addr_validate(&self, human: &str) -> StdResult<Addr> {
        let source = build_region(human.as_bytes());
        let source_ptr = &*source as *const Region as u32;

        let result = unsafe { addr_validate(source_ptr) };
        if result != 0 {
            let error = unsafe { consume_string_region_written_by_vm(result as *mut Region) };
            return Err(StdError::generic_err(format!(
                "addr_validate errored: {}",
                error
            )));
        }

        Ok(Addr::unchecked(human))
    }

    fn addr_canonicalize(&self, human: &str) -> StdResult<CanonicalAddr> {
        let send = build_region(human.as_bytes());
        let send_ptr = &*send as *const Region as u32;
        let canon = alloc(CANONICAL_ADDRESS_BUFFER_LENGTH);

        let result = unsafe { addr_canonicalize(send_ptr, canon as u32) };
        if result != 0 {
            let error = unsafe { consume_string_region_written_by_vm(result as *mut Region) };
            return Err(StdError::generic_err(format!(
                "addr_canonicalize errored: {}",
                error
            )));
        }

        let out = unsafe { consume_region(canon) };
        Ok(CanonicalAddr(Binary(out)))
    }

    fn addr_humanize(&self, canonical: &CanonicalAddr) -> StdResult<Addr> {
        let send = build_region(&canonical);
        let send_ptr = &*send as *const Region as u32;
        let human = alloc(HUMAN_ADDRESS_BUFFER_LENGTH);

        let result = unsafe { addr_humanize(send_ptr, human as u32) };
        if result != 0 {
            let error = unsafe { consume_string_region_written_by_vm(result as *mut Region) };
            return Err(StdError::generic_err(format!(
                "addr_humanize errored: {}",
                error
            )));
        }

        let address = unsafe { consume_string_region_written_by_vm(human) };
        Ok(Addr::unchecked(address))
    }

    fn secp256k1_verify(
        &self,
        message_hash: &[u8],
        signature: &[u8],
        public_key: &[u8],
    ) -> Result<bool, VerificationError> {
        let hash_send = build_region(message_hash);
        let hash_send_ptr = &*hash_send as *const Region as u32;
        let sig_send = build_region(signature);
        let sig_send_ptr = &*sig_send as *const Region as u32;
        let pubkey_send = build_region(public_key);
        let pubkey_send_ptr = &*pubkey_send as *const Region as u32;

        let result = unsafe { secp256k1_verify(hash_send_ptr, sig_send_ptr, pubkey_send_ptr) };
        match result {
            0 => Ok(true),
            1 => Ok(false),
            2 => panic!("MessageTooLong must not happen. This is a bug in the VM."),
            3 => Err(VerificationError::InvalidHashFormat),
            4 => Err(VerificationError::InvalidSignatureFormat),
            5 => Err(VerificationError::InvalidPubkeyFormat),
            10 => Err(VerificationError::GenericErr),
            error_code => Err(VerificationError::unknown_err(error_code)),
        }
    }

    fn secp256k1_recover_pubkey(
        &self,
        message_hash: &[u8],
        signature: &[u8],
        recover_param: u8,
    ) -> Result<Vec<u8>, RecoverPubkeyError> {
        let hash_send = build_region(message_hash);
        let hash_send_ptr = &*hash_send as *const Region as u32;
        let sig_send = build_region(signature);
        let sig_send_ptr = &*sig_send as *const Region as u32;

        let result =
            unsafe { secp256k1_recover_pubkey(hash_send_ptr, sig_send_ptr, recover_param.into()) };
        let error_code = from_high_half(result);
        let pubkey_ptr = from_low_half(result);
        match error_code {
            0 => {
                let pubkey = unsafe { consume_region(pubkey_ptr as *mut Region) };
                Ok(pubkey)
            }
            2 => panic!("MessageTooLong must not happen. This is a bug in the VM."),
            3 => Err(RecoverPubkeyError::InvalidHashFormat),
            4 => Err(RecoverPubkeyError::InvalidSignatureFormat),
            6 => Err(RecoverPubkeyError::InvalidRecoveryParam),
            error_code => Err(RecoverPubkeyError::unknown_err(error_code)),
        }
    }

    fn ed25519_verify(
        &self,
        message: &[u8],
        signature: &[u8],
        public_key: &[u8],
    ) -> Result<bool, VerificationError> {
        let msg_send = build_region(message);
        let msg_send_ptr = &*msg_send as *const Region as u32;
        let sig_send = build_region(signature);
        let sig_send_ptr = &*sig_send as *const Region as u32;
        let pubkey_send = build_region(public_key);
        let pubkey_send_ptr = &*pubkey_send as *const Region as u32;

        let result = unsafe { ed25519_verify(msg_send_ptr, sig_send_ptr, pubkey_send_ptr) };
        match result {
            0 => Ok(true),
            1 => Ok(false),
            2 => panic!("Error code 2 unused since CosmWasm 0.15. This is a bug in the VM."),
            3 => panic!("InvalidHashFormat must not happen. This is a bug in the VM."),
            4 => Err(VerificationError::InvalidSignatureFormat),
            5 => Err(VerificationError::InvalidPubkeyFormat),
            10 => Err(VerificationError::GenericErr),
            error_code => Err(VerificationError::unknown_err(error_code)),
        }
    }

    fn ed25519_batch_verify(
        &self,
        messages: &[&[u8]],
        signatures: &[&[u8]],
        public_keys: &[&[u8]],
    ) -> Result<bool, VerificationError> {
        let msgs_encoded = encode_sections(messages);
        let msgs_send = build_region(&msgs_encoded);
        let msgs_send_ptr = &*msgs_send as *const Region as u32;

        let sigs_encoded = encode_sections(signatures);
        let sig_sends = build_region(&sigs_encoded);
        let sigs_send_ptr = &*sig_sends as *const Region as u32;

        let pubkeys_encoded = encode_sections(public_keys);
        let pubkeys_send = build_region(&pubkeys_encoded);
        let pubkeys_send_ptr = &*pubkeys_send as *const Region as u32;

        let result =
            unsafe { ed25519_batch_verify(msgs_send_ptr, sigs_send_ptr, pubkeys_send_ptr) };
        match result {
            0 => Ok(true),
            1 => Ok(false),
            2 => panic!("Error code 2 unused since CosmWasm 0.15. This is a bug in the VM."),
            3 => panic!("InvalidHashFormat must not happen. This is a bug in the VM."),
            4 => Err(VerificationError::InvalidSignatureFormat),
            5 => Err(VerificationError::InvalidPubkeyFormat),
            10 => Err(VerificationError::GenericErr),
            error_code => Err(VerificationError::unknown_err(error_code)),
        }
    }

    fn debug(&self, message: &str) {
        // keep the boxes in scope, so we free it at the end (don't cast to pointers same line as build_region)
        let region = build_region(message.as_bytes());
        let region_ptr = region.as_ref() as *const Region as u32;
        unsafe { debug(region_ptr) };
    }

    /// ECDSA secp256k1 implementation.
    ///
    /// This function verifies message hashes (typically, hashed unsing SHA-256) against a signature,
    /// with the public key of the signer, using the secp256k1 elliptic curve digital signature
    /// parametrization / algorithm.
    ///
    /// The signature and public key are in "Cosmos" format:
    /// - signature:  Serialized "compact" signature (64 bytes).
    /// - public key: [Serialized according to SEC 2](https://www.oreilly.com/library/view/programming-bitcoin/9781492031482/ch04.html)
    /// (33 or 65 bytes).
    fn secp256k1_verify(
        &self,
        message_hash: &[u8],
        signature: &[u8],
        public_key: &[u8],
    ) -> Result<bool, VerificationError> {
        let hash_send = build_region(message_hash);
        let hash_send_ptr = &*hash_send as *const Region as u32;
        let sig_send = build_region(signature);
        let sig_send_ptr = &*sig_send as *const Region as u32;
        let pubkey_send = build_region(public_key);
        let pubkey_send_ptr = &*pubkey_send as *const Region as u32;

        let result = unsafe { secp256k1_verify(hash_send_ptr, sig_send_ptr, pubkey_send_ptr) };
        match result {
            0 => Ok(true),
            1 => Ok(false),
            3 => Err(VerificationError::InvalidHashFormat),
            4 => Err(VerificationError::InvalidSignatureFormat),
            5 => Err(VerificationError::InvalidPubkeyFormat),
            10 => Err(VerificationError::GenericErr),
            error_code => Err(VerificationError::unknown_err(error_code)),
        }
    }

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
        recover_param: u8,
    ) -> Result<Vec<u8>, RecoverPubkeyError> {
        let hash_send = build_region(message_hash);
        let hash_send_ptr = &*hash_send as *const Region as u32;
        let sig_send = build_region(signature);
        let sig_send_ptr = &*sig_send as *const Region as u32;

        let result =
            unsafe { secp256k1_recover_pubkey(hash_send_ptr, sig_send_ptr, recover_param.into()) };
        let error_code = from_high_half(result);
        let pubkey_ptr = from_low_half(result);
        match error_code {
            0 => {
                let pubkey = unsafe { consume_region(pubkey_ptr as *mut Region) };
                Ok(pubkey)
            }
            3 => Err(RecoverPubkeyError::InvalidHashFormat),
            4 => Err(RecoverPubkeyError::InvalidSignatureFormat),
            6 => Err(RecoverPubkeyError::InvalidRecoveryParam),
            error_code => Err(RecoverPubkeyError::unknown_err(error_code)),
        }
    }

    /// EdDSA ed25519 implementation.
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
    ) -> Result<bool, VerificationError> {
        let msg_send = build_region(message);
        let msg_send_ptr = &*msg_send as *const Region as u32;
        let sig_send = build_region(signature);
        let sig_send_ptr = &*sig_send as *const Region as u32;
        let pubkey_send = build_region(public_key);
        let pubkey_send_ptr = &*pubkey_send as *const Region as u32;

        let result = unsafe { ed25519_verify(msg_send_ptr, sig_send_ptr, pubkey_send_ptr) };
        match result {
            0 => Ok(true),
            1 => Ok(false),
            4 => Err(VerificationError::InvalidSignatureFormat),
            5 => Err(VerificationError::InvalidPubkeyFormat),
            10 => Err(VerificationError::GenericErr),
            error_code => Err(VerificationError::unknown_err(error_code)),
        }
    }

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
    ) -> Result<bool, VerificationError> {
        let msgs_encoded = encode_sections(messages);
        let msgs_send = build_region(&msgs_encoded);
        let msgs_send_ptr = &*msgs_send as *const Region as u32;

        let sigs_encoded = encode_sections(signatures);
        let sig_sends = build_region(&sigs_encoded);
        let sigs_send_ptr = &*sig_sends as *const Region as u32;

        let pubkeys_encoded = encode_sections(public_keys);
        let pubkeys_send = build_region(&pubkeys_encoded);
        let pubkeys_send_ptr = &*pubkeys_send as *const Region as u32;

        let result =
            unsafe { ed25519_batch_verify(msgs_send_ptr, sigs_send_ptr, pubkeys_send_ptr) };
        match result {
            0 => Ok(true),
            1 => Ok(false),
            4 => Err(VerificationError::InvalidSignatureFormat),
            5 => Err(VerificationError::InvalidPubkeyFormat),
            10 => Err(VerificationError::GenericErr),
            error_code => Err(VerificationError::unknown_err(error_code)),
        }
    }

    fn secp256k1_sign(&self, message: &[u8], private_key: &[u8]) -> Result<Vec<u8>, SigningError> {
        let msg_send = build_region(message);
        let msg_send_ptr = &*msg_send as *const Region as u32;
        let pk_send = build_region(private_key);
        let pk_send_ptr = &*pk_send as *const Region as u32;

        let result = unsafe { secp256k1_sign(msg_send_ptr, pk_send_ptr) };
        let error_code = from_high_half(result);
        let signature_ptr = from_low_half(result);
        match error_code {
            0 => {
                let signature = unsafe { consume_region(signature_ptr as *mut Region) };
                Ok(signature)
            }
            1000 => Err(SigningError::InvalidPrivateKeyFormat),
            error_code => Err(SigningError::unknown_err(error_code)),
        }
    }

    fn ed25519_sign(&self, message: &[u8], private_key: &[u8]) -> Result<Vec<u8>, SigningError> {
        let msg_send = build_region(message);
        let msg_send_ptr = &*msg_send as *const Region as u32;
        let pk_send = build_region(private_key);
        let pk_send_ptr = &*pk_send as *const Region as u32;

        let result = unsafe { ed25519_sign(msg_send_ptr, pk_send_ptr) };
        let error_code = from_high_half(result);
        let signature_ptr = from_low_half(result);
        match error_code {
            0 => {
                let signature = unsafe { consume_region(signature_ptr as *mut Region) };
                Ok(signature)
            }
            1000 => Err(SigningError::InvalidPrivateKeyFormat),
            error_code => Err(SigningError::unknown_err(error_code)),
        }
    }
}

use std::convert::TryInto;

/// Returns the four most significant bytes
#[allow(dead_code)] // only used in Wasm builds
#[inline]
pub fn from_high_half(data: u64) -> u32 {
    (data >> 32).try_into().unwrap()
}

/// Returns the four least significant bytes
#[allow(dead_code)] // only used in Wasm builds
#[inline]
pub fn from_low_half(data: u64) -> u32 {
    (data & 0xFFFFFFFF).try_into().unwrap()
}

/// Takes a pointer to a Region and reads the data into a String.
/// This is for trusted string sources only.
unsafe fn consume_string_region_written_by_vm(from: *mut Region) -> String {
    let data = consume_region(from);
    // We trust the VM/chain to return correct UTF-8, so let's save some gas
    String::from_utf8_unchecked(data)
}

/// A stateless convenience wrapper around imports provided by the VM
pub struct ExternalQuerier {}

impl ExternalQuerier {
    pub fn new() -> ExternalQuerier {
        ExternalQuerier {}
    }
}

impl Querier for ExternalQuerier {
    fn raw_query(&self, bin_request: &[u8]) -> QuerierResult {
        let req = build_region(bin_request);
        let request_ptr = &*req as *const Region as u32;

        let response_ptr = unsafe { query_chain(request_ptr) };
        let response = unsafe { consume_region(response_ptr as *mut Region) };

        from_slice(&response).unwrap_or_else(|parsing_err| {
            SystemResult::Err(SystemError::InvalidResponse {
                error: parsing_err.to_string(),
                response: response.into(),
            })
        })
    }
}
