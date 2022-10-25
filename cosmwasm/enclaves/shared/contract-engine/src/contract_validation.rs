use cw_types_v1::ibc::IbcPacketReceiveMsg;
use cw_types_v1::results::REPLY_ENCRYPTION_MAGIC_BYTES;
use log::*;
use std::time::{Duration, Instant};

use cw_types_v010::types::{CanonicalAddr, Coin, HumanAddr};
use enclave_cosmos_types::traits::CosmosAminoPubkey;
use enclave_cosmos_types::types::{
    ContractCode, CosmWasmMsg, CosmosPubKey, HandleType, SigInfo, SignDoc, StdSignDoc,
};
use enclave_crypto::traits::VerifyingKey;
use enclave_crypto::{sha_256, AESKey, Hmac, Kdf, HASH_SIZE, KEY_MANAGER};
use enclave_ffi_types::EnclaveError;

use crate::io::create_callback_signature;
use crate::message::is_ibc_msg;
use crate::types::SecretMessage;

pub type ContractKey = [u8; CONTRACT_KEY_LENGTH];

pub const CONTRACT_KEY_LENGTH: usize = HASH_SIZE + HASH_SIZE;

const HEX_ENCODED_HASH_SIZE: usize = HASH_SIZE * 2;
const SIZE_OF_U64: usize = 8;

pub fn generate_encryption_key(
    sender: &CanonicalAddr,
    block_height: &u64,
    contract_hash: &[u8; HASH_SIZE],
    contract_address: &CanonicalAddr,
) -> Result<[u8; CONTRACT_KEY_LENGTH], EnclaveError> {
    let consensus_state_ikm = KEY_MANAGER.get_consensus_state_ikm().unwrap();

    let sender_id = generate_sender_id(&(sender.0).0, block_height);

    let mut encryption_key = [0u8; 64];

    let authenticated_contract_id = generate_contract_id(
        &consensus_state_ikm,
        &sender_id,
        contract_hash,
        &(contract_address.0).0,
    );

    encryption_key[0..32].copy_from_slice(&sender_id);
    encryption_key[32..].copy_from_slice(&authenticated_contract_id);

    trace!("contract key: {:?}", hex::encode(encryption_key));

    Ok(encryption_key)
}

pub fn generate_sender_id(msg_sender: &[u8], block_height: &u64) -> [u8; HASH_SIZE] {
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
) -> Result<(), EnclaveError> {
    // parse contract key -> < signer_id || authentication_code >
    let mut signer_id: [u8; HASH_SIZE] = [0u8; HASH_SIZE];
    signer_id.copy_from_slice(&contract_key[0..HASH_SIZE]);

    let mut expected_authentication_id: [u8; HASH_SIZE] = [0u8; HASH_SIZE];
    expected_authentication_id.copy_from_slice(&contract_key[HASH_SIZE..]);

    // get the enclave key
    let enclave_key = KEY_MANAGER
        .get_consensus_state_ikm()
        .map_err(|_err| {
            error!("Error extracting consensus_state_key");
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

    if calculated_authentication_id == expected_authentication_id {
        trace!("Successfully authenticated the contract!");
        Ok(())
    } else {
        warn!("got an error while trying to deserialize output bytes");
        Err(EnclaveError::FailedContractAuthentication)
    }
}

pub struct ValidatedMessage {
    pub validated_msg: Vec<u8>,
    pub reply_params: Option<Vec<ReplyParams>>,
}

#[derive(Debug)]
pub struct ReplyParams {
    pub recipient_contract_hash: Vec<u8>,
    pub sub_msg_id: u64,
}

/// Validate that the message sent to the enclave (after decryption) was actually addressed to this contract.
pub fn validate_msg(
    msg: &[u8],
    contract_hash: &[u8; HASH_SIZE],
    data_for_validation: Option<Vec<u8>>,
    handle_type: Option<HandleType>,
) -> Result<ValidatedMessage, EnclaveError> {
    match handle_type {
        None => validate_basic_msg(msg, contract_hash, data_for_validation),
        Some(h) => match is_ibc_msg(h.clone()) {
            false => validate_basic_msg(msg, contract_hash, data_for_validation),
            true => validate_ibc_msg(msg, contract_hash, data_for_validation, h),
        },
    }
}

pub fn validate_ibc_msg(
    msg: &[u8],
    contract_hash: &[u8; HASH_SIZE],
    data_for_validation: Option<Vec<u8>>,
    handle_type: HandleType,
) -> Result<ValidatedMessage, EnclaveError> {
    match handle_type {
        HandleType::HANDLE_TYPE_IBC_PACKET_RECEIVE => {
            let mut parsed_ibc_packet: IbcPacketReceiveMsg =
                serde_json::from_slice(msg).map_err(|err| {
                    warn!(
                    "IbcPacketReceive msg got an error while trying to deserialize msg input bytes into json {:?}: {}",
                    String::from_utf8_lossy(msg),
                    err
                );
                    EnclaveError::FailedToDeserialize
                })?;

            let validated_msg = validate_basic_msg(
                parsed_ibc_packet.packet.data.as_slice(),
                contract_hash,
                data_for_validation,
            )?;
            parsed_ibc_packet.packet.data = validated_msg.validated_msg.as_slice().into();

            Ok(ValidatedMessage{
                validated_msg: serde_json::to_vec(&parsed_ibc_packet).map_err(|err| {
                    warn!(
                        "got an error while trying to serialize parsed_ibc_packet msg into bytes {:?}: {}",
                        parsed_ibc_packet, err
                    );
                    EnclaveError::FailedToSerialize
                })?,
                reply_params: validated_msg.reply_params,
            })
        }
        _ => {
            warn!("Malformed message - in IBC, only packet receive message can be encrypted");
            Err(EnclaveError::ValidationFailure)
        }
    }
}

pub fn validate_basic_msg(
    msg: &[u8],
    contract_hash: &[u8; HASH_SIZE],
    data_for_validation: Option<Vec<u8>>,
) -> Result<ValidatedMessage, EnclaveError> {
    if data_for_validation.is_none() && msg.len() < HEX_ENCODED_HASH_SIZE {
        warn!("Malformed message - expected contract code hash to be prepended to the msg");
        return Err(EnclaveError::ValidationFailure);
    }

    let mut received_contract_hash: [u8; HEX_ENCODED_HASH_SIZE] = [0u8; HEX_ENCODED_HASH_SIZE];
    let mut validated_msg: Vec<u8>;
    let mut reply_params: Option<Vec<ReplyParams>> = None;

    match data_for_validation {
        Some(c) => {
            received_contract_hash.copy_from_slice(&c.as_slice()[0..HEX_ENCODED_HASH_SIZE]);
            let mut partial_msg = c[HEX_ENCODED_HASH_SIZE..].to_vec();
            while partial_msg.len() >= REPLY_ENCRYPTION_MAGIC_BYTES.len()
                && partial_msg[0..(REPLY_ENCRYPTION_MAGIC_BYTES.len())]
                    == *REPLY_ENCRYPTION_MAGIC_BYTES
            {
                if reply_params.is_none() {
                    reply_params = Some(vec![]);
                }

                partial_msg = partial_msg[REPLY_ENCRYPTION_MAGIC_BYTES.len()..].to_vec();

                let mut sub_msg_deserialized: [u8; SIZE_OF_U64] = [0u8; SIZE_OF_U64];
                sub_msg_deserialized.copy_from_slice(&partial_msg[..SIZE_OF_U64]);

                let sub_msg_id: u64 = u64::from_be_bytes(sub_msg_deserialized);
                partial_msg = partial_msg[SIZE_OF_U64..].to_vec();

                let mut reply_recipient_contract_hash: [u8; HEX_ENCODED_HASH_SIZE] =
                    [0u8; HEX_ENCODED_HASH_SIZE];
                reply_recipient_contract_hash
                    .copy_from_slice(&partial_msg[0..HEX_ENCODED_HASH_SIZE]);

                reply_params.as_mut().unwrap().push(ReplyParams {
                    recipient_contract_hash: reply_recipient_contract_hash.to_vec(),
                    sub_msg_id,
                });

                partial_msg = partial_msg[HEX_ENCODED_HASH_SIZE..].to_vec();
            }

            validated_msg = msg.to_vec();
        }
        None => {
            received_contract_hash.copy_from_slice(&msg[0..HEX_ENCODED_HASH_SIZE]);
            validated_msg = msg[HEX_ENCODED_HASH_SIZE..].to_vec();
        }
    }

    let decoded_hash: Vec<u8> = hex::decode(&received_contract_hash[..]).map_err(|_| {
        warn!("Got message with malformed contract hash");

        EnclaveError::ValidationFailure
    })?;

    if decoded_hash != contract_hash {
        warn!("Message contains mismatched contract hash");
        return Err(EnclaveError::ValidationFailure);
    }

    while validated_msg.len() >= REPLY_ENCRYPTION_MAGIC_BYTES.len()
        && validated_msg[0..(REPLY_ENCRYPTION_MAGIC_BYTES.len())] == *REPLY_ENCRYPTION_MAGIC_BYTES
    {
        if reply_params.is_none() {
            reply_params = Some(vec![]);
        }

        validated_msg = validated_msg[REPLY_ENCRYPTION_MAGIC_BYTES.len()..].to_vec();

        let mut sub_msg_deserialized: [u8; SIZE_OF_U64] = [0u8; SIZE_OF_U64];
        sub_msg_deserialized.copy_from_slice(&validated_msg[..SIZE_OF_U64]);

        let sub_msg_id: u64 = u64::from_be_bytes(sub_msg_deserialized);
        validated_msg = validated_msg[SIZE_OF_U64..].to_vec();

        let mut reply_recipient_contract_hash: [u8; HEX_ENCODED_HASH_SIZE] =
            [0u8; HEX_ENCODED_HASH_SIZE];
        reply_recipient_contract_hash.copy_from_slice(&validated_msg[0..HEX_ENCODED_HASH_SIZE]);

        reply_params.as_mut().unwrap().push(ReplyParams {
            recipient_contract_hash: reply_recipient_contract_hash.to_vec(),
            sub_msg_id,
        });

        validated_msg = validated_msg[HEX_ENCODED_HASH_SIZE..].to_vec();
    }

    Ok(ValidatedMessage {
        validated_msg,
        reply_params,
    })
}

/// Verify all the parameters sent to the enclave match up, and were signed by the right account.
pub fn verify_params(
    sig_info: &SigInfo,
    sent_funds: &[Coin],
    sender: &CanonicalAddr,
    contract_address: &HumanAddr,
    msg: &SecretMessage,
) -> Result<(), EnclaveError> {
    debug!("Verifying message signatures for: {:?}", sig_info);

    let start = Instant::now();
    // If there's no callback signature - it's not a callback and there has to be a tx signer + signature
    if let Some(callback_sig) = &sig_info.callback_sig {
        return verify_callback_sig(callback_sig.as_slice(), sender, msg, sent_funds);
    }
    let duration = start.elapsed();
    trace!(
        "verify_params: Time elapsed in verify_callback_sig: {:?}",
        duration
    );

    trace!(
        "Sign bytes are: {:?}",
        String::from_utf8_lossy(sig_info.sign_bytes.as_slice())
    );

    let start = Instant::now();
    let (sender_public_key, messages) = get_signer_and_messages(sig_info, sender)?;
    let duration = start.elapsed();
    trace!(
        "verify_params: Time elapsed in get_signer_and_messages: {:?}",
        duration
    );

    trace!(
        "sender canonical address is: {:?}",
        sender_public_key.get_address().0 .0
    );
    trace!("sender signature is: {:?}", sig_info.signature);
    trace!("sign bytes are: {:?}", sig_info.sign_bytes);

    let start = Instant::now();
    sender_public_key
        .verify_bytes(
            sig_info.sign_bytes.as_slice(),
            sig_info.signature.as_slice(),
            sig_info.sign_mode,
        )
        .map_err(|err| {
            warn!("Signature verification failed: {:?}", err);
            EnclaveError::FailedTxVerification
        })?;
    let duration = start.elapsed();
    trace!(
        "verify_params: Time elapsed in verify_bytes: {:?}",
        duration
    );

    let start = Instant::now();
    if verify_message_params(
        &messages,
        sender,
        sent_funds,
        contract_address,
        &sender_public_key,
        msg,
    ) {
        info!("Parameters verified successfully");
        return Ok(());
    }
    let duration = start.elapsed();
    trace!(
        "verify_params: Time elapsed in verify_message_params: {:?}",
        duration
    );
    warn!("Parameter verification failed");

    Err(EnclaveError::FailedTxVerification)
}

fn get_signer_and_messages(
    sign_info: &SigInfo,
    sender: &CanonicalAddr,
) -> Result<(CosmosPubKey, Vec<CosmWasmMsg>), EnclaveError> {
    use cosmos_proto::tx::signing::SignMode::*;
    match sign_info.sign_mode {
        SIGN_MODE_DIRECT => {
            let sign_doc = SignDoc::from_bytes(sign_info.sign_bytes.as_slice())?;
            trace!("sign doc: {:?}", sign_doc);

            // This verifies that signatures and sign bytes are self consistent
            let sender_public_key =
                sign_doc
                    .auth_info
                    .sender_public_key(sender)
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
        SIGN_MODE_EIP_191 => {
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

            let sign_bytes_as_string = String::from_utf8_lossy(&sign_info.sign_bytes.0).to_string();

            trace!(
                "SIGN_MODE_EIP_191 sign_bytes_as_string: {:?}",
                sign_bytes_as_string
            );

            // Always starts with '\x19Ethereum Signed Message:\n\d+{'
            // So we need to find the first occurance of '{' and go from there until the end
            let start_index = match sign_bytes_as_string.find('{') {
                Some(start_index) => start_index,
                None => {
                    warn!(
                        "SIGN_MODE_EIP_191 failed to find first occurance of '{{' in '{}'",
                        sign_bytes_as_string
                    );
                    return Err(EnclaveError::FailedTxVerification);
                }
            };
            let sign_doc_str = &sign_bytes_as_string[start_index..];

            let sign_doc: StdSignDoc = serde_json::from_str(sign_doc_str).map_err(|err| {
                warn!(
                    "failed to parse SIGN_MODE_EIP_191 StdSignDoc as JSON from '{}': {:?}",
                    sign_doc_str, err
                );
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
    sender: &CanonicalAddr,
    msg: &SecretMessage,
    sent_funds: &[Coin],
) -> Result<(), EnclaveError> {
    if verify_callback_sig_impl(callback_signature, sender, msg, sent_funds) {
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
fn verify_contract(msg: &CosmWasmMsg, contract_address: &HumanAddr) -> bool {
    // Contract address is relevant only to execute, since during sending an instantiate message the contract address is not yet known
    match msg {
        CosmWasmMsg::Execute { contract, .. } => {
            info!("Verifying contract address..");
            let is_verified = contract_address == contract;
            if !is_verified {
                trace!(
                    "Contract address sent to enclave {:?} is not the same as the signed one {:?}",
                    contract_address,
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
fn verify_funds(msg: &CosmWasmMsg, sent_funds_msg: &[Coin]) -> bool {
    match msg {
        CosmWasmMsg::Execute { sent_funds, .. }
        | CosmWasmMsg::Instantiate {
            init_funds: sent_funds,
            ..
        } => sent_funds_msg == sent_funds,
        CosmWasmMsg::Other => false,
    }
}

fn verify_message_params(
    messages: &[CosmWasmMsg],
    sender: &CanonicalAddr,
    sent_funds: &[Coin],
    contract_address: &HumanAddr,
    signer_public_key: &CosmosPubKey,
    sent_msg: &SecretMessage,
) -> bool {
    debug!("Verifying sender..");

    // let msg_sender = match CanonicalAddr::from_human(&env.message.sender) {
    //     Ok(msg_sender) => msg_sender,
    //     _ => return false,
    // };

    let signer_addr = signer_public_key.get_address();
    if &signer_addr != sender {
        warn!("Sender verification failed!");
        trace!(
            "Message sender {:?} does not match with the message signer {:?}",
            sender,
            signer_addr
        );
        return false;
    }

    info!("Verifying message..");
    // If msg is not found (is None) then it means message verification failed,
    // since it didn't find a matching signed message
    let msg = get_verified_msg(messages, sender, sent_msg);
    if msg.is_none() {
        debug!("Message verification failed!");
        trace!(
            "Message sent to contract {:?} by {:?} does not match any signed messages {:?}",
            sent_msg.to_vec(),
            sender,
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

    if !verify_contract(msg, contract_address) {
        warn!("Contract address verification failed!");
        return false;
    }

    info!("Verifying funds..");
    if !verify_funds(msg, sent_funds) {
        warn!("Funds verification failed!");
        return false;
    }

    true
}
