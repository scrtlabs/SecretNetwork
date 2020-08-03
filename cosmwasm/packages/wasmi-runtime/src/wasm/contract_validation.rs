use log::*;

use enclave_ffi_types::EnclaveError;

use crate::cosmwasm::encoding::Binary;
use crate::cosmwasm::types::{CanonicalAddr, CosmosSignature, Env};
use crate::crypto::{secp256k1, sha_256, AESKey, Hmac, Kdf, HASH_SIZE, KEY_MANAGER};
use crate::wasm::io;
use crate::wasm::types::SecretMessage;

pub type ContractKey = [u8; CONTRACT_KEY_LENGTH];

pub const CONTRACT_KEY_LENGTH: usize = HASH_SIZE + HASH_SIZE;

pub fn generate_encryption_key(
    env: &Env,
    contract: &[u8],
) -> Result<[u8; CONTRACT_KEY_LENGTH], EnclaveError> {
    let consensus_state_ikm = KEY_MANAGER.get_consensus_state_ikm().unwrap();

    let contract_hash = calc_contract_hash(contract);

    let sender_id = generate_sender_id(env.message.sender.as_slice(), env.block.height as u64);

    let mut encryption_key = [0u8; 64];

    let authenticated_contract_id =
        generate_contract_id(&consensus_state_ikm, &sender_id, &contract_hash);

    encryption_key[0..32].copy_from_slice(&sender_id);
    encryption_key[32..].copy_from_slice(&authenticated_contract_id);

    Ok(encryption_key)
}

pub fn extract_contract_key(env: &Env) -> Result<[u8; CONTRACT_KEY_LENGTH], EnclaveError> {
    if env.contract_key.is_none() {
        error!("Contract execute with empty contract key");
        return Err(EnclaveError::FailedContractAuthentication);
    }

    let contract_key =
        base64::decode(env.contract_key.as_ref().unwrap().as_bytes()).map_err(|err| {
            error!(
                "got an error while trying to deserialize output bytes into json {:?}: {}",
                env, err
            );
            EnclaveError::FailedContractAuthentication
        })?;

    if contract_key.len() != CONTRACT_KEY_LENGTH {
        error!("Contract execute with empty contract key");
        return Err(EnclaveError::FailedContractAuthentication);
    }

    let mut key_as_bytes = [0u8; CONTRACT_KEY_LENGTH];

    key_as_bytes.copy_from_slice(&contract_key);

    Ok(key_as_bytes)
}

pub fn generate_sender_id(msg_sender: &[u8], block_height: u64) -> [u8; HASH_SIZE] {
    let mut input_data = msg_sender.to_vec();
    input_data.extend_from_slice(&block_height.to_be_bytes());
    sha_256(&input_data)
}

pub fn generate_contract_id(
    consensus_state_ikm: &AESKey,
    sender_id: &[u8; HASH_SIZE],
    code_hash: &[u8; HASH_SIZE],
) -> [u8; HASH_SIZE] {
    let authentication_key = consensus_state_ikm.derive_key_from_this(sender_id.as_ref());

    let mut input_data = sender_id.to_vec();
    input_data.extend_from_slice(code_hash);

    authentication_key.sign_sha_256(&input_data)
}

pub fn calc_contract_hash(contract_bytes: &[u8]) -> [u8; HASH_SIZE] {
    sha_256(&contract_bytes)
}

pub fn validate_contract_key(
    contract_key: &[u8; CONTRACT_KEY_LENGTH],
    contract_code: &[u8],
) -> bool {
    // parse contract key -> < signer_id || authentication_code >
    let mut signer_id: [u8; HASH_SIZE] = [0u8; HASH_SIZE];
    signer_id.copy_from_slice(&contract_key[0..HASH_SIZE]);

    let mut expected_authentication_id: [u8; HASH_SIZE] = [0u8; HASH_SIZE];
    expected_authentication_id.copy_from_slice(&contract_key[HASH_SIZE..]);

    // calculate contract hash
    let contract_hash = calc_contract_hash(contract_code);

    // get the enclave key
    let enclave_key = KEY_MANAGER
        .get_consensus_state_ikm()
        .map_err(|_err| {
            error!("Error extractling consensus_state_key");
            false
        })
        .unwrap();

    // calculate the authentication_id
    let calculated_authentication_id =
        generate_contract_id(&enclave_key, &signer_id, &contract_hash);

    calculated_authentication_id == expected_authentication_id
}

pub fn verify_params(
    signatures: Vec<CosmosSignature>,
    sign_bytes: Vec<Binary>,
    msg_sender: &CanonicalAddr,
    callback_signature: Option<Vec<u8>>,
    contract_address: &CanonicalAddr,
    msg: &SecretMessage,
) -> Result<(), EnclaveError> {
    trace!("Verifying tx signatures..");

    // Verify each signature
    // We currently support only secp256k1 signatures
    for (sig, sb) in signatures.iter().zip(sign_bytes.iter()) {
        secp256k1::verify_signature(sb, sig)?;
    }

    trace!("signatures verified");

    // Check if sender does match the sender's signed pubkey
    if verify_sender(&signatures[0], msg_sender) {
        info!("msg.sender is the tx signer");

        return Ok(());
    }

    info!(
        "Message sender {:?} does not match with the signed pubkey's addres: {:?}",
        msg_sender,
        signatures[0].get_public_key()
    );

    // Check if there's a callback signature and if it is valid
    if let Some(cb_sig) = callback_signature {
        if verify_callback_sig(cb_sig, msg_sender, contract_address, msg) {
            info!("msg.sender is the calling contract");

            return Ok(());
        }
    } else {
        error!("Did not provide callback signature");
    }

    error!("Signature verification failed");

    Err(EnclaveError::FailedTxVerification)
}

fn verify_sender(signature: &CosmosSignature, msg_sender: &CanonicalAddr) -> bool {
    // Taking only the first signature, as it is the fee payer
    // Ref: https://github.com/cosmos/cosmos-sdk/blob/v0.38.3/x/auth/types/stdtx.go#L24
    let sender_pubkey = signature.get_public_key();
    let address = secp256k1::pubkey_to_tm_address(&sender_pubkey);

    address.eq(&msg_sender.as_slice())
}

fn verify_callback_sig(
    callback_signature: Vec<u8>,
    sender: &CanonicalAddr,
    receiver: &CanonicalAddr,
    msg: &SecretMessage,
) -> bool {
    // No signature
    if callback_signature.is_empty() {
        return false;
    }

    let callback_sig = io::create_callback_signature(sender, &receiver, msg);

    if !callback_signature.eq(&callback_sig) {
        info!(
            "Contract signature does not match with the one sent: {:?}",
            callback_signature
        );
        return false;
    }

    true
}
