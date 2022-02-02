use log::*;

use enclave_ffi_types::EnclaveError;

use enclave_cosmos_types::traits::CosmosAminoPubkey;
use enclave_cosmos_types::types::{
    ContractCode, CosmWasmMsg, CosmosPubKey, SigInfo, SignDoc, StdSignDoc,
};
use enclave_cosmwasm_types::types::{CanonicalAddr, Coin, Env, HumanAddr};
use enclave_crypto::traits::VerifyingKey;
use enclave_crypto::{sha_256, AESKey, Hmac, Kdf, HASH_SIZE, KEY_MANAGER};

use crate::io::create_callback_signature;
use crate::types::SecretMessage;

pub type ContractKey = [u8; CONTRACT_KEY_LENGTH];

pub const CONTRACT_KEY_LENGTH: usize = HASH_SIZE + HASH_SIZE;

const HEX_ENCODED_HASH_SIZE: usize = HASH_SIZE * 2;

pub fn generate_encryption_key(
    env: &Env,
    contract_hash: [u8; HASH_SIZE],
    contract_address: &[u8],
) -> Result<[u8; CONTRACT_KEY_LENGTH], EnclaveError> {
    let consensus_state_ikm = KEY_MANAGER.get_consensus_state_ikm().unwrap();

    let (_, sender_address_u5) = bech32::decode(env.message.sender.as_str()).map_err(|err| {
        warn!(
            "got an error while trying to deserialize env.message.sender from bech32 string to bytes {:?}: {}",
            env.message.sender, err
        );
        EnclaveError::FailedToDeserialize
    })?;
    let snder_address: Vec<u8> = sender_address_u5.iter().map(|x| x.to_u8()).collect();

    let sender_id = generate_sender_id(&snder_address, env.block.height as u64);

    let mut encryption_key = [0u8; 64];

    let authenticated_contract_id = generate_contract_id(
        &consensus_state_ikm,
        &sender_id,
        &contract_hash,
        contract_address,
    );

    encryption_key[0..32].copy_from_slice(&sender_id);
    encryption_key[32..].copy_from_slice(&authenticated_contract_id);

    Ok(encryption_key)
}

pub fn extract_contract_key(env: &Env) -> Result<[u8; CONTRACT_KEY_LENGTH], EnclaveError> {
    if env.contract_key.is_none() {
        warn!("Contract execute with empty contract key");
        return Err(EnclaveError::FailedContractAuthentication);
    }

    let contract_key =
        base64::decode(env.contract_key.as_ref().unwrap().as_bytes()).map_err(|err| {
            warn!(
                "got an error while trying to deserialize output bytes into json {:?}: {}",
                env, err
            );
            EnclaveError::FailedContractAuthentication
        })?;

    if contract_key.len() != CONTRACT_KEY_LENGTH {
        warn!("Contract execute with empty contract key");
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
    contract_address: &[u8],
) -> [u8; HASH_SIZE] {
    let authentication_key = consensus_state_ikm.derive_key_from_this(sender_id.as_ref());

    let mut input_data = sender_id.to_vec();
    input_data.extend_from_slice(code_hash);
    input_data.extend_from_slice(contract_address);
    authentication_key.sign_sha_256(&input_data)
}

pub fn validate_contract_key(
    contract_key: &[u8; CONTRACT_KEY_LENGTH],
    contract_address: &CanonicalAddr,
    contract_code: &ContractCode,
) -> bool {
    // parse contract key -> < signer_id || authentication_code >
    let mut signer_id: [u8; HASH_SIZE] = [0u8; HASH_SIZE];
    signer_id.copy_from_slice(&contract_key[0..HASH_SIZE]);

    let mut expected_authentication_id: [u8; HASH_SIZE] = [0u8; HASH_SIZE];
    expected_authentication_id.copy_from_slice(&contract_key[HASH_SIZE..]);

    // get the enclave key
    let enclave_key = KEY_MANAGER
        .get_consensus_state_ikm()
        .map_err(|_err| {
            warn!("Error extracting consensus_state_key");
            false
        })
        .unwrap();

    // calculate the authentication_id
    let calculated_authentication_id = generate_contract_id(
        &enclave_key,
        &signer_id,
        &contract_code.hash(),
        contract_address.as_slice(),
    );

    calculated_authentication_id == expected_authentication_id
}

/// Validate that the message sent to the enclave (after decryption) was actually addressed to this contract.
pub fn validate_msg(msg: &[u8], contract_hash: [u8; HASH_SIZE]) -> Result<Vec<u8>, EnclaveError> {
    if msg.len() < HEX_ENCODED_HASH_SIZE {
        warn!("Malformed message - expected contract code hash to be prepended to the msg");
        return Err(EnclaveError::ValidationFailure);
    }

    let mut received_contract_hash: [u8; HEX_ENCODED_HASH_SIZE] = [0u8; HEX_ENCODED_HASH_SIZE];
    received_contract_hash.copy_from_slice(&msg[0..HEX_ENCODED_HASH_SIZE]);

    let decoded_hash: Vec<u8> = hex::decode(&received_contract_hash[..]).map_err(|_| {
        warn!("Got message with malformed contract hash");
        EnclaveError::ValidationFailure
    })?;

    if decoded_hash != contract_hash {
        warn!("Message contains mismatched contract hash");
        return Err(EnclaveError::ValidationFailure);
    }

    Ok(msg[HEX_ENCODED_HASH_SIZE..].to_vec())
}

/// Verify all the parameters sent to the enclave match up, and were signed by the right account.
pub fn verify_params(
    sig_info: &SigInfo,
    env: &Env,
    msg: &SecretMessage,
) -> Result<(), EnclaveError> {
    info!("Verifying message signatures for: {:?}", sig_info);

    // If there's no callback signature - it's not a callback and there has to be a tx signer + signature
    if let Some(callback_sig) = &sig_info.callback_sig {
        return verify_callback_sig(
            callback_sig.as_slice(),
            &env.message.sender,
            msg,
            &env.message.sent_funds,
        );
    }

    trace!(
        "Sign bytes are: {:?}",
        String::from_utf8_lossy(sig_info.sign_bytes.as_slice())
    );

    let (sender_public_key, messages) = get_signer_and_messages(sig_info, env)?;

    trace!(
        "sender canonical address is: {:?}",
        sender_public_key.get_address().0.0
    );
    trace!("sender signature is: {:?}", sig_info.signature);
    trace!("sign bytes are: {:?}", sig_info.sign_bytes);

    sender_public_key
        .verify_bytes(
            sig_info.sign_bytes.as_slice(),
            sig_info.signature.as_slice(),
        )
        .map_err(|err| {
            warn!("Signature verification failed: {:?}", err);
            EnclaveError::FailedTxVerification
        })?;

    if verify_message_params(&messages, env, &sender_public_key, msg) {
        info!("Parameters verified successfully");
        return Ok(());
    }

    warn!("Parameter verification failed");

    Err(EnclaveError::FailedTxVerification)
}

fn get_signer_and_messages(
    sign_info: &SigInfo,
    env: &Env,
) -> Result<(CosmosPubKey, Vec<CosmWasmMsg>), EnclaveError> {
    use cosmos_proto::tx::signing::SignMode::*;
    match sign_info.sign_mode {
        SIGN_MODE_DIRECT => {
            let sign_doc = SignDoc::from_bytes(sign_info.sign_bytes.as_slice())?;
            trace!("sign doc: {:?}", sign_doc);

            let sender = CanonicalAddr::from_human(&env.message.sender).map_err(|err| {
                warn!(
                    "failed to canonicalize message sender: {} {}",
                    env.message.sender, err
                );
                EnclaveError::FailedTxVerification
            })?;
            trace!("sender canonical address is: {:?}", sender.0.0);

            // This verifies that signatures and sign bytes are self consistent
            let sender_public_key =
                sign_doc
                    .auth_info
                    .sender_public_key(&sender)
                    .ok_or_else(|| {
                        warn!("Couldn't find message sender in auth_info.signer_infos");
                        EnclaveError::FailedTxVerification
                    })?;

            Ok((sender_public_key.clone(), sign_doc.body.messages))
        }
        SIGN_MODE_LEGACY_AMINO_JSON => {
            use protobuf::well_known_types::Any as AnyProto;
            use protobuf::Message;

            let any_pub_key =
                AnyProto::parse_from_bytes(&sign_info.public_key.0).map_err(|err| {
                    warn!("failed to parse public key as Any: {:?}", err);
                    EnclaveError::FailedTxVerification
                })?;
            let public_key = CosmosPubKey::from_proto(&any_pub_key).map_err(|err| {
                warn!("failure to parse pubkey: {:?}", err);
                EnclaveError::FailedTxVerification
            })?;
            let sign_doc: StdSignDoc = serde_json::from_slice(sign_info.sign_bytes.as_slice())
                .map_err(|err| {
                    warn!("failure to parse StdSignDoc: {:?}", err);
                    EnclaveError::FailedTxVerification
                })?;
            let messages: Result<Vec<CosmWasmMsg>, _> = sign_doc
                .msgs
                .iter()
                .map(|x| x.clone().into_cosmwasm_msg())
                .collect();
            Ok((public_key, messages?))
        }
        _ => {
            warn!("unsupported signature mode: {:?}", sign_info.sign_mode);
            Err(EnclaveError::FailedTxVerification)
        }
    }
}

/// Verify that the callback sig is appropriate.
///
///This is used when contracts send callbacks to each other.
fn verify_callback_sig(
    callback_signature: &[u8],
    sender: &HumanAddr,
    msg: &SecretMessage,
    sent_funds: &[Coin],
) -> Result<(), EnclaveError> {
    if verify_callback_sig_impl(
        callback_signature,
        &CanonicalAddr::from_human(sender).or(Err(EnclaveError::FailedToSerialize))?,
        msg,
        sent_funds,
    ) {
        info!("Message verified! msg.sender is the calling contract");
        return Ok(());
    }

    warn!("Callback signature verification failed");
    Err(EnclaveError::FailedTxVerification)
}

fn verify_callback_sig_impl(
    callback_signature: &[u8],
    sender: &CanonicalAddr,
    msg: &SecretMessage,
    sent_funds: &[Coin],
) -> bool {
    if callback_signature.is_empty() {
        return false;
    }

    let callback_sig = create_callback_signature(sender, msg, sent_funds);

    if callback_signature != callback_sig {
        trace!(
            "Contract signature does not match with the one sent: {:?}",
            callback_signature
        );
        return false;
    }

    true
}

/// Get the cosmwasm message that contains the encrypted message
fn get_verified_msg<'sd>(
    messages: &'sd [CosmWasmMsg],
    msg_sender: &CanonicalAddr,
    sent_msg: &SecretMessage,
) -> Option<&'sd CosmWasmMsg> {
    messages.iter().find(|&m| match m {
        CosmWasmMsg::Execute { msg, sender, .. }
        | CosmWasmMsg::Instantiate {
            init_msg: msg,
            sender,
            ..
        } => msg_sender == sender && &sent_msg.to_vec() == msg,
        CosmWasmMsg::Other => false,
    })
}

/// Check that the contract listed in the cosmwasm message matches the one in env
fn verify_contract(msg: &CosmWasmMsg, env: &Env) -> bool {
    // Contract address is relevant only to execute, since during sending an instantiate message the contract address is not yet known
    match msg {
        CosmWasmMsg::Execute { contract, .. } => {
            info!("Verifying contract address..");
            let is_verified = env.contract.address == *contract;
            if !is_verified {
                trace!(
                    "Contract address sent to enclave {:?} is not the same as the signed one {:?}",
                    env.contract.address,
                    *contract
                );
            }
            is_verified
        }
        CosmWasmMsg::Instantiate { .. } => true,
        CosmWasmMsg::Other => false,
    }
}

/// Check that the funds listed in the cosmwasm message matches the ones in env
fn verify_funds(msg: &CosmWasmMsg, env: &Env) -> bool {
    match msg {
        CosmWasmMsg::Execute { sent_funds, .. }
        | CosmWasmMsg::Instantiate {
            init_funds: sent_funds,
            ..
        } => &env.message.sent_funds == sent_funds,
        CosmWasmMsg::Other => false,
    }
}

fn verify_message_params(
    messages: &[CosmWasmMsg],
    env: &Env,
    signer_public_key: &CosmosPubKey,
    sent_msg: &SecretMessage,
) -> bool {
    info!("Verifying sender..");

    let msg_sender = match CanonicalAddr::from_human(&env.message.sender) {
        Ok(msg_sender) => msg_sender,
        _ => return false,
    };

    let signer_addr = signer_public_key.get_address();
    if signer_addr != msg_sender {
        warn!("Sender verification failed!");
        trace!(
            "Message sender {:?} does not match with the message signer {:?}",
            msg_sender,
            signer_addr
        );
        return false;
    }

    info!("Verifying message..");
    // If msg is not found (is None) then it means message verification failed,
    // since it didn't find a matching signed message
    let msg = get_verified_msg(messages, &msg_sender, sent_msg);
    if msg.is_none() {
        warn!("Message verification failed!");
        trace!(
            "Message sent to contract {:?} by {:?} does not match any signed messages {:?}",
            sent_msg.to_vec(),
            msg_sender,
            messages
        );
        return false;
    }
    let msg = msg.unwrap();

    if msg.sender() != Some(&signer_addr) {
        warn!(
            "message signer did not match cosmwasm message sender: {:?} {:?}",
            signer_addr, msg
        );
        return false;
    }

    if !verify_contract(msg, env) {
        warn!("Contract address verification failed!");
        return false;
    }

    info!("Verifying funds..");
    if !verify_funds(msg, env) {
        warn!("Funds verification failed!");
        return false;
    }

    true
}
