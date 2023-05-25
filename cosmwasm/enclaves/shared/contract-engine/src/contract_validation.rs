use cw_types_v1::ibc::{IbcPacketReceiveMsg};
use cw_types_v1::results::REPLY_ENCRYPTION_MAGIC_BYTES;
use log::*;

#[cfg(feature = "light-client-validation")]
use cw_types_generic::BaseEnv;

use cw_types_v010::types::{CanonicalAddr, Coin, HumanAddr};
use enclave_cosmos_types::traits::CosmosAminoPubkey;
use enclave_cosmos_types::types::{
    ContractCode, CosmosSdkMsg, CosmosPubKey, FungibleTokenPacketData, HandleType,
    SigInfo, SignDoc, StdSignDoc, IbcHooksIncomingTransferMsg, Packet, IbcHooksOutgoingTransferMemo, IBCLifecycleComplete, is_transfer_ack_error, IBCPacketAckMsg, IBCLifecycleCompleteOptions,
};
use enclave_crypto::traits::VerifyingKey;
use enclave_crypto::{sha_256, AESKey, Hmac, Kdf, HASH_SIZE, KEY_MANAGER};
use enclave_ffi_types::EnclaveError;

use crate::io::create_callback_signature;
use crate::message::is_ibc_msg;
use crate::types::SecretMessage;

#[cfg(feature = "light-client-validation")]
use block_verifier::VERIFIED_MESSAGES;

use sha2::{Digest, Sha256};
extern crate hex;

pub type ContractKey = [u8; CONTRACT_KEY_LENGTH];

pub const CONTRACT_KEY_LENGTH: usize = HASH_SIZE + HASH_SIZE;

const HEX_ENCODED_HASH_SIZE: usize = HASH_SIZE * 2;
const SIZE_OF_U64: usize = 8;

#[cfg(feature = "light-client-validation")]
fn is_subslice(larger: &[u8], smaller: &[u8]) -> bool {
    if smaller.is_empty() {
        return true;
    }
    if larger.len() < smaller.len() {
        return false;
    }
    for window in larger.windows(smaller.len()) {
        if window == smaller {
            return true;
        }
    }
    false
}

#[cfg(feature = "light-client-validation")]
pub fn verify_block_info(base_env: &BaseEnv) -> Result<(), EnclaveError> {
    let verified_msgs = VERIFIED_MESSAGES.lock().unwrap();
    if verified_msgs.height() != base_env.0.block.height {
        error!("wrong height for this block - 0xF6AC");
        return Err(EnclaveError::ValidationFailure);
    }

    if verified_msgs.time() != base_env.0.block.time as i128 {
        error!("wrong height for this block - 0xF6AF");
        return Err(EnclaveError::ValidationFailure);
    }

    Ok(())
}

#[cfg(feature = "light-client-validation")]
pub fn check_msg_matches_state(msg: &[u8]) -> bool {
    let mut verified_msgs = VERIFIED_MESSAGES.lock().unwrap();
    let remaining_msgs = verified_msgs.remaining();

    if remaining_msgs == 0 {
        error!("Failed to validate message, error 0x3555");
        return false;
    }

    // Msgs might fail in the sdk before they reach the enclave. In this case we need to run through
    // all the messages available before we can determine that there has been a failure
    // this isn't an attack vector since this can happen anyway by manipulating the state between executions
    while verified_msgs.remaining() > 0 {
        if let Some(expected_msg) = verified_msgs.get_next() {
            if is_subslice(&expected_msg, msg) {
                return true;
            }
        }
    }

    error!("Failed to validate message, error 0x3255");

    // if this message fails to verify we have to fail the rest of the TX, so we won't get any
    // other messages
    verified_msgs.clear();

    false
}

pub fn generate_contract_key(
    sender: &CanonicalAddr,
    block_height: &u64,
    contract_hash: &[u8; HASH_SIZE],
    contract_address: &CanonicalAddr,
) -> Result<[u8; CONTRACT_KEY_LENGTH], EnclaveError> {
    let consensus_state_ikm = KEY_MANAGER.get_consensus_state_ikm().unwrap();

    let sender_id = generate_sender_id(&(sender.0).0, block_height);

    let mut contract_key = [0u8; 64];

    let authenticated_contract_id = generate_contract_id(
        // contract_key is public and used as a salt to differentiate state of different contracts
        // there's no reason not to use consensus_state_ikm.genesis
        // otherwise we'd have to migrate all the contract_keys every time we rotate the seed
        // which is doable but requires one more ecall & just unnecessary
        // actually using consensus_state_ikm might be entirely unnecessary here but it's too
        // painful at this point to change the protocol to remove it
        &consensus_state_ikm.genesis,
        &sender_id,
        contract_hash,
        &(contract_address.0).0,
    );

    contract_key[0..32].copy_from_slice(&sender_id);
    contract_key[32..].copy_from_slice(&authenticated_contract_id);

    trace!("contract key: {:?}", hex::encode(contract_key));

    Ok(contract_key)
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
        .unwrap()
        .genesis;

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

                let mut sub_msg_id_serialized: [u8; SIZE_OF_U64] = [0u8; SIZE_OF_U64];
                sub_msg_id_serialized.copy_from_slice(&partial_msg[..SIZE_OF_U64]);

                let sub_msg_id: u64 = u64::from_be_bytes(sub_msg_id_serialized);
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

        let mut sub_msg_id_serialized: [u8; SIZE_OF_U64] = [0u8; SIZE_OF_U64];
        sub_msg_id_serialized.copy_from_slice(&validated_msg[..SIZE_OF_U64]);

        let sub_msg_id: u64 = u64::from_be_bytes(sub_msg_id_serialized);
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
#[allow(clippy::too_many_arguments)]
pub fn verify_params(
    sig_info: &SigInfo,
    sent_funds: &[Coin],
    sender: &CanonicalAddr,
    contract_address: &HumanAddr,
    msg: &SecretMessage,
    #[cfg(feature = "light-client-validation")]
    og_msg: &[u8],
    should_validate_sig_info: bool,
    should_validate_input: bool,
    handle_type: HandleType,
) -> Result<(), EnclaveError> {
    if should_validate_sig_info {
        debug!("Verifying message signatures for: {:?}", sig_info);

        //let start = Instant::now();
        // If there's no callback signature - it's not a callback and there has to be a tx signer + signature
        if let Some(callback_sig) = &sig_info.callback_sig {
            return verify_callback_sig(callback_sig.as_slice(), sender, msg, sent_funds);
        }
        // let duration = start.elapsed();
        // trace!(
        //     "verify_params: Time elapsed in verify_callback_sig: {:?}",
        //     duration
        // );
        #[cfg(feature = "light-client-validation")]
        if !check_msg_matches_state(og_msg) {
            return Err(EnclaveError::ValidationFailure);
        }
        // check if sign_bytes are in approved tx list

        trace!(
            "Sign bytes are: {:?}",
            String::from_utf8_lossy(sig_info.sign_bytes.as_slice())
        );

        //let start = Instant::now();
        let sender_public_key = get_signer(sig_info, sender, handle_type)?;
        // let duration = start.elapsed();
        // trace!(
        //     "verify_params: Time elapsed in get_signer_and_messages: {:?}",
        //     duration
        // );

        trace!(
            "sender canonical address is: {:?}",
            sender_public_key.get_address().0 .0
        );
        trace!("sender signature is: {:?}", sig_info.signature);
        trace!("sign bytes are: {:?}", sig_info.sign_bytes);

        //let start = Instant::now();
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
        // let duration = start.elapsed();
        // trace!(
        //     "verify_params: Time elapsed in verify_bytes: {:?}",
        //     duration
        // );

        let signer_addr = sender_public_key.get_address();
        if &signer_addr != sender {
            warn!("Sender verification failed!");
            trace!(
                "Message sender {:?} does not match with the message signer {:?}",
                sender,
                signer_addr
            );
            return Err(EnclaveError::FailedTxVerification);
        }
    }

    if should_validate_input {
        let messages = get_messages(sig_info, handle_type)?;

        // let start = Instant::now();
        let is_verified = verify_message_params(
            &messages,
            sender,
            sent_funds,
            contract_address,
            msg,
            handle_type,
        );
        // let duration = start.elapsed();
        // trace!(
        //     "verify_params: Time elapsed in verify_message_params: {:?}",
        //     duration
        // );

        if !is_verified {
            warn!("Parameter verification failed");
            return Err(EnclaveError::FailedTxVerification);
        }
    }

    info!("Parameters verified successfully");
    Ok(())
}

fn get_signer(sign_info: &SigInfo, sender: &CanonicalAddr, handle_type: HandleType) -> Result<CosmosPubKey, EnclaveError> {
    use cosmos_proto::tx::signing::SignMode::*;
    match sign_info.sign_mode {
        SIGN_MODE_DIRECT => {
            let sign_doc = SignDoc::from_bytes(sign_info.sign_bytes.as_slice(), handle_type)?;
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

            Ok(sender_public_key.clone())
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

            Ok(public_key)
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

            Ok(public_key)
        }
        _ => {
            warn!(
                "get_signer(): unsupported signature mode: {:?}",
                sign_info.sign_mode
            );
            Err(EnclaveError::FailedTxVerification)
        }
    }
}

fn get_messages(sign_info: &SigInfo, handle_type: HandleType) -> Result<Vec<CosmosSdkMsg>, EnclaveError> {
    use cosmos_proto::tx::signing::SignMode::*;
    match sign_info.sign_mode {
        SIGN_MODE_DIRECT => {
            let sign_doc = SignDoc::from_bytes(sign_info.sign_bytes.as_slice(), handle_type)?;
            trace!("sign doc: {:?}", sign_doc);

            Ok(sign_doc.body.messages)
        }
        SIGN_MODE_LEGACY_AMINO_JSON => {
            let sign_doc: StdSignDoc = serde_json::from_slice(sign_info.sign_bytes.as_slice())
                .map_err(|err| {
                    warn!("failure to parse StdSignDoc: {:?}", err);
                    EnclaveError::FailedTxVerification
                })?;
            let messages: Result<Vec<CosmosSdkMsg>, _> = sign_doc
                .msgs
                .iter()
                .map(|x| x.clone().into_cosmwasm_msg())
                .collect();
            Ok(messages?)
        }
        SIGN_MODE_EIP_191 => {
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
            let messages: Result<Vec<CosmosSdkMsg>, _> = sign_doc
                .msgs
                .iter()
                .map(|x| x.clone().into_cosmwasm_msg())
                .collect();
            Ok(messages?)
        }
        _ => {
            warn!(
                "get_messages(): unsupported signature mode: {:?}",
                sign_info.sign_mode
            );
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

    let callback_sig = create_callback_signature(sender, &msg.msg, sent_funds);

    if callback_signature != callback_sig {
        trace!(
            "Contract signature does not match with the one sent: {:?}. Expected message to be signed: {:?}",
            callback_signature,
            String::from_utf8_lossy(msg.msg.as_slice())
        );

        return false;
    }

    true
}

/// Get the cosmwasm message that contains the encrypted message
fn get_verified_msg<'sd>(
    messages: &'sd [CosmosSdkMsg],
    msg_sender: &CanonicalAddr,
    sent_msg: &SecretMessage,
    handle_type: HandleType,
) -> Option<&'sd CosmosSdkMsg> {
    trace!("get_verified_msg: {:?}", messages);

    messages.iter().find(|&m| match m {
        CosmosSdkMsg::MsgExecuteContract { msg, sender, .. }
        | CosmosSdkMsg::MsgInstantiateContract {
            init_msg: msg,
            sender,
            ..
        } => msg_sender == sender && &sent_msg.to_vec() == msg,
        CosmosSdkMsg::MsgRecvPacket {
           packet: Packet {
            sequence,
            source_port,
            source_channel,
            destination_port,
            destination_channel,
            data,
           }, ..
        } => match handle_type {
            HandleType::HANDLE_TYPE_IBC_PACKET_RECEIVE => {
                let parsed_sent_msg = serde_json::from_slice::<IbcPacketReceiveMsg>(&sent_msg.msg);
                if parsed_sent_msg.is_err() {
                    trace!("get_verified_msg HANDLE_TYPE_IBC_PACKET_RECEIVE: sent_msg.msg cannot be parsed as IbcPacketReceiveMsg: {:?} Error: {:?}", String::from_utf8_lossy(&sent_msg.msg), parsed_sent_msg.err());

                    trace!("Checking if sent_msg & data are encrypted");
                    return &sent_msg.to_vec() == data;
                }
                let parsed = parsed_sent_msg.unwrap();
                
                parsed.packet.data.as_slice() == data.as_slice()
                    && parsed.packet.sequence == *sequence
                    && parsed.packet.src.port_id == *source_port
                    && parsed.packet.src.channel_id == *source_channel
                    && parsed.packet.dest.port_id == *destination_port
                    && parsed.packet.dest.channel_id == *destination_channel
                    // TODO check timeout too? sequence + destination_channel + data should be enough
            }
            HandleType::HANDLE_TYPE_IBC_WASM_HOOKS_INCOMING_TRANSFER => {
                let fungible_token_packet_data = serde_json::from_slice::<FungibleTokenPacketData>(data);
                if fungible_token_packet_data.is_err(){
                    trace!("get_verified_msg HANDLE_TYPE_IBC_WASM_HOOKS_INCOMING_TRANSFER: data cannot be parsed as FungibleTokenPacketData: {:?} Error: {:?}", String::from_utf8_lossy(data), fungible_token_packet_data.err());
                    return false;
                }
                let fungible_token_packet_data= fungible_token_packet_data.unwrap();


                let ibc_hooks_incoming_transfer_msg = serde_json::from_slice::<IbcHooksIncomingTransferMsg>(fungible_token_packet_data.memo.clone().unwrap_or_else(|| "".to_string()).as_bytes());
                if ibc_hooks_incoming_transfer_msg.is_err(){
                    trace!("get_verified_msg HANDLE_TYPE_IBC_WASM_HOOKS_INCOMING_TRANSFER: fungible_token_packet_data.memo cannot be parsed as IbcHooksIncomingTransferMsg: {:?} Error: {:?}", fungible_token_packet_data.memo, ibc_hooks_incoming_transfer_msg.err());
                    return false;
                }
                let ibc_hooks_incoming_transfer_msg = ibc_hooks_incoming_transfer_msg.unwrap();
                
                let sent_msg_value = serde_json::from_slice::<serde_json::Value>(&sent_msg.msg);
                if sent_msg_value.is_err(){
                    trace!("get_verified_msg HANDLE_TYPE_IBC_WASM_HOOKS_INCOMING_TRANSFER: sent_msg.msg cannot be parsed as serde_json::Value: {:?} Error: {:?}", String::from_utf8_lossy(&sent_msg.msg), sent_msg_value.err());
                    return false;
                }

                ibc_hooks_incoming_transfer_msg.wasm.msg == sent_msg_value.unwrap()
            }
            _ => false,
        },
        CosmosSdkMsg::Other => false,
        CosmosSdkMsg::MsgAcknowledgement { packet, acknowledgement, signer,.. } => {
            match handle_type {
                HandleType::HANDLE_TYPE_IBC_PACKET_ACK => {
                    let ibc_packet_ack_msg = serde_json::from_slice::<IBCPacketAckMsg>(&sent_msg.msg);
                    if ibc_packet_ack_msg.is_err(){
                        trace!("get_verified_msg HANDLE_TYPE_IBC_PACKET_ACK: sent_msg.msg cannot be parsed as IBCPacketAckMsg: {:?} Error: {:?}", String::from_utf8_lossy(&sent_msg.msg), ibc_packet_ack_msg.err());
                        return false;
                    }
                    let ibc_packet_ack_msg= ibc_packet_ack_msg.unwrap();

                    ibc_packet_ack_msg.original_packet.src.channel_id == packet.source_channel &&
                    ibc_packet_ack_msg.original_packet.src.port_id == packet.source_port &&
                    ibc_packet_ack_msg.original_packet.dest.channel_id == packet.destination_channel &&
                    ibc_packet_ack_msg.original_packet.dest.port_id == packet.destination_port &&
                    ibc_packet_ack_msg.original_packet.sequence == packet.sequence &&
                    ibc_packet_ack_msg.original_packet.data == packet.data &&
                    ibc_packet_ack_msg.relayer == *signer &&
                    ibc_packet_ack_msg.acknowledgement.data == *acknowledgement
                },
                HandleType::HANDLE_TYPE_IBC_WASM_HOOKS_OUTGOING_TRANSFER_ACK => {
                    trace!("ASSAF 1");
                    let ibc_lifecycle_complete = serde_json::from_slice::<IBCLifecycleComplete>(&sent_msg.msg);
                if ibc_lifecycle_complete.is_err(){
                    trace!("get_verified_msg HANDLE_TYPE_IBC_WASM_HOOKS_OUTGOING_TRANSFER_ACK: sent_msg.msg cannot be parsed as IBCLifecycleComplete: {:?} Error: {:?}", String::from_utf8_lossy(&sent_msg.msg), ibc_lifecycle_complete.err());
                    return false;
                }
                let ibc_lifecycle_complete= ibc_lifecycle_complete.unwrap();

                trace!("ASSAF 2 {:?}", ibc_lifecycle_complete);


              match  ibc_lifecycle_complete {
                IBCLifecycleComplete::IBCLifecycleComplete(IBCLifecycleCompleteOptions::IBCAck { channel, sequence, ack, success }) => 
                    channel == packet.source_channel
                    && sequence == packet.sequence
                    && ack == String::from_utf8_lossy( acknowledgement)
                    && success == !is_transfer_ack_error(acknowledgement)
                ,
                IBCLifecycleComplete::IBCLifecycleComplete(IBCLifecycleCompleteOptions::IBCTimeout { .. }) => false,
            }
                },
            _ => false,

            }
        },
    })
}

/// Check that the contract listed in the cosmwasm message matches the one in env
fn verify_contract(msg: &CosmosSdkMsg, contract_address: &HumanAddr) -> bool {
    // Contract address is relevant only to execute, since during sending an instantiate message the contract address is not yet known
    match msg {
        CosmosSdkMsg::MsgExecuteContract { contract, .. } => {
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
        CosmosSdkMsg::MsgInstantiateContract { .. } => true,
        CosmosSdkMsg::Other => false,
        CosmosSdkMsg::MsgRecvPacket {
          packet:Packet{  destination_port,
            data,..},
            ..
        } => {
            if destination_port == "transfer" {
                // Packet was routed here through ibc-hooks

                // Parse data as FungibleTokenPacketData JSON
                let packet_data: FungibleTokenPacketData = match serde_json::from_slice(
                    data.as_slice(),
                ) {
                    Ok(packet_data) => packet_data,
                    Err(err) => {
                        trace!(
                            "Contract was called via ibc-hooks but packet_data cannot be parsed as FungibleTokenPacketData: {:?} Error: {:?}",
                            String::from_utf8_lossy(data.as_slice()),
                            err,
                        );
                        return false;
                    }
                };

                // memo must be set in ibc-hooks
                let memo = match packet_data.memo {
                    Some(memo) => memo,
                    None => {
                        trace!("Contract was called via ibc-hooks but packet_data.memo is empty");
                        return false;
                    }
                };

                // Parse data.memo as IbcHooksWasmMsg JSON
                let wasm_msg: IbcHooksIncomingTransferMsg = match serde_json::from_slice(memo.as_bytes()) {
                    Ok(wasm_msg) => wasm_msg,
                    Err(err) => {
                        trace!(
                            "Contract was called via ibc-hooks but packet_data.memo cannot be parsed as IbcHooksWasmMsg: {:?} Error: {:?}",
                            memo,
                            err,
                        );
                        return false;
                    }
                };

                // In ibc-hooks contract_address == packet_data.memo.wasm.contract == packet_data.receiver
                let is_verified = *contract_address == packet_data.receiver
                    && *contract_address == wasm_msg.wasm.contract;
                if !is_verified {
                    trace!(
                        "Contract address sent to enclave {:?} is not the same as in ibc-hooks packet receiver={:?} memo={:?}",
                        contract_address,
                        packet_data.receiver,
                        wasm_msg.wasm.contract
                    );
                }
                is_verified
            } else {
                // Packet is for an IBC enabled contract
                // destination_port is of the form "wasm.{contract_address}"

                // Extract contract_address from destination_port
                // This also checks that destination_port starts with "wasm."
                let contract_address_from_port = match destination_port.strip_prefix("wasm.") {
                    Some(contract_address) => contract_address,
                    None => {
                        trace!(
                            "IBC-enabled Contract was called via MsgRecvPacket but destination_port doesn't start with \"wasm.\": {:?}",
                            destination_port,
                        );
                        return false;
                    }
                };

                let is_verified = *contract_address == HumanAddr::from(contract_address_from_port);
                if !is_verified {
                    trace!(
                        "IBC-enabled Contract address sent to enclave {:?} is not the same as extracted from MsgRecvPacket but destination_port: {:?}",
                        contract_address,
                        contract_address_from_port,
                    );
                }
                is_verified
            }
        },
        CosmosSdkMsg::MsgAcknowledgement{
            packet:Packet{ 
                source_port,
                data,..  },
            ..
        } => {
            if source_port == "transfer" {
// Packet was sent from a contract via the transfer port.
// We're getting the ack here because the memo field contained `{"ibc_callback": "secret1contractAddr"}`,
// and ibc-hooks routes the ack into `secret1contractAddr`.

// Parse data as FungibleTokenPacketData JSON
let packet_data: FungibleTokenPacketData = match serde_json::from_slice(
    data.as_slice(),
) {
    Ok(packet_data) => packet_data,
    Err(err) => {
        trace!(
            "Contract was called via ibc-hooks ack callback but packet_data cannot be parsed as FungibleTokenPacketData: {:?} Error: {:?}",
            String::from_utf8_lossy(data.as_slice()),
            err,
        );
        return false;
    }
};

// memo must be set in ibc-hooks
let memo = match packet_data.memo {
    Some(memo) => memo,
    None => {
        trace!("Contract was called via ibc-hooks ack callback but packet_data.memo is empty");
        return false;
    }
};

// Parse data.memo as `{"ibc_callback": "secret1contractAddr"}` JSON
let ibc_hooks_outgoing_memo: IbcHooksOutgoingTransferMemo = match serde_json::from_slice(memo.as_bytes()) {
    Ok(wasm_msg) => wasm_msg,
    Err(err) => {
        trace!(
            "Contract was called via ibc-hooks but packet_data.memo cannot be parsed as IbcHooksWasmMsg: {:?} Error: {:?}",
            memo,
            err,
        );
        return false;
    }
};

let is_verified =  *contract_address == ibc_hooks_outgoing_memo.ibc_callback && *contract_address == packet_data.sender;
if !is_verified {
    trace!(
        "Contract address sent to enclave {:?} is not the same as in ibc-hooks outgoing transfer callback address packet {:?}",
        contract_address,
        ibc_hooks_outgoing_memo.ibc_callback
    );
}
is_verified
            } else {
                  // Packet was sent from an IBC enabled contract
                // source_port is of the form "wasm.{contract_address}"

                // Extract contract_address from source_port
                // This also checks that source_port starts with "wasm."
                let contract_address_from_port = match source_port.strip_prefix("wasm.") {
                    Some(contract_address) => contract_address,
                    None => {
                        trace!(
                            "IBC-enabled Contract was called via MsgAcknowledgement but source_port doesn't start with \"wasm.\": {:?}",
                            source_port,
                        );
                        return false;
                    }
                };

                let is_verified = *contract_address == HumanAddr::from(contract_address_from_port);
                if !is_verified {
                    trace!(
                        "Contract address sent to enclave {:?} is not the same as extracted from MsgAcknowledgement but source_port: {:?}",
                        contract_address,
                        contract_address_from_port,
                    );
                }
                is_verified
            }

        }
    }
}

/// Check that the funds listed in the cosmwasm message matches the ones in env
fn verify_funds(msg: &CosmosSdkMsg, sent_funds_msg: &[Coin]) -> bool {
    match msg {
        CosmosSdkMsg::MsgExecuteContract { sent_funds, .. }
        | CosmosSdkMsg::MsgInstantiateContract {
            init_funds: sent_funds,
            ..
        } => sent_funds_msg == sent_funds,
        CosmosSdkMsg::Other => false,
        CosmosSdkMsg::MsgRecvPacket {
            packet: Packet {
            data,
            source_port,
            source_channel,
            destination_port,
            destination_channel,
            ..
            },
            ..
        } => {
            if destination_port == "transfer" {
                // Packet was routed here through ibc-hooks

                // Should be just one coin
                if sent_funds_msg.len() != 1 {
                    trace!(
                        "Contract was called via ibc-hooks but sent_funds_msg.len() != 1: {:?}",
                        sent_funds_msg,
                    );
                    return false;
                }

                let sent_funds_msg_coin = &sent_funds_msg[0];

                // Parse data as FungibleTokenPacketData JSON
                let packet_data: FungibleTokenPacketData = match serde_json::from_slice(
                    data.as_slice(),
                ) {
                    Ok(packet_data) => packet_data,
                    Err(err) => {
                        trace!(
                            "Contract was called via ibc-hooks but packet_data cannot be parsed as FungibleTokenPacketData: {:?} Error: {:?}",
                            String::from_utf8_lossy(data.as_slice()),
                            err,
                        );
                        return false;
                    }
                };

                // Check amount
                if sent_funds_msg_coin.amount != packet_data.amount {
                    trace!(
                        "Contract was called via ibc-hooks but sent_funds_msg_coin.amount != packet_data.amount: {:?} != {:?}",
                        sent_funds_msg_coin.amount,
                        packet_data.amount,
                    );
                    return false;
                }

                // The packet's denom is the denom in the sender chain.
                // It needs to be converted to the local denom.
                // Logic source: https://github.com/scrtlabs/SecretNetwork/blob/96b0ba7d6/x/ibc-hooks/wasm_hook.go#L483-L513
                let denom: String = if receiver_chain_is_source(
                    source_port,
                    source_channel,
                    &packet_data.denom,
                ) {
                    // remove prefix added by sender chain
                    let voucher_prefix = get_denom_prefix(source_port, source_channel);

                    let unprefixed_denom: String = match packet_data
                        .denom
                        .strip_prefix(&voucher_prefix)
                    {
                        Some(unprefixed_denom) => unprefixed_denom.to_string(),
                        None => {
                            trace!(
                                "Contract was called via ibc-hooks but packet_data.denom doesn't start with voucher_prefix: {:?} != {:?}",
                                packet_data.denom,
                                voucher_prefix,
                            );
                            return false;
                        }
                    };

                    // The denomination used to send the coins is either the native denom or the hash of the path
                    // if the denomination is not native.
                    let denom_trace = parse_denom_trace(&unprefixed_denom);
                    if !denom_trace.path.is_empty() {
                        denom_trace.ibc_denom()
                    } else {
                        unprefixed_denom
                    }
                } else {
                    let prefixed_denom = get_denom_prefix(destination_port, destination_channel)
                        + &packet_data.denom;
                    parse_denom_trace(&prefixed_denom).ibc_denom()
                };

                // Check denom
                if sent_funds_msg_coin.denom.to_lowercase() != denom.to_lowercase() {
                    trace!(
                        "Contract was called via ibc-hooks but sent_funds_msg_coin.denom != denom: {:?} != {:?}",
                        sent_funds_msg_coin.denom,
                        denom,
                    );
                    return false;
                }

                true
            } else {
                // Packet is for an IBC enabled contract
                // No funds should be sent
                sent_funds_msg.is_empty()
            }
        }
        CosmosSdkMsg::MsgAcknowledgement { .. } => {
            
                // No funds should be sent with a MsgAcknowledgement
                sent_funds_msg.is_empty()
        },
    }
}

fn verify_message_params(
    messages: &[CosmosSdkMsg],
    sender: &CanonicalAddr,
    sent_funds: &[Coin],
    contract_address: &HumanAddr,
    sent_msg: &SecretMessage,
    handle_type: HandleType,
) -> bool {
    info!("Verifying message...");
    // If msg is not found (is None) then it means message verification failed,
    // since it didn't find a matching signed message
    let msg = get_verified_msg(messages, sender, sent_msg, handle_type);
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

    match msg {
        CosmosSdkMsg::MsgRecvPacket{..} |
        CosmosSdkMsg::MsgAcknowledgement{..}   => {
            // No sender to verify.
            // Going to pass null sender to the contract if all other checks pass.
        }
        CosmosSdkMsg::MsgExecuteContract { .. } | CosmosSdkMsg::MsgInstantiateContract { .. } | CosmosSdkMsg::Other => {
            if msg.sender() != Some(sender) {
                warn!(
                    "message sender did not match cosmwasm message sender: {:?} {:?}",
                    sender, msg
                );
                return false;
            }
        }
    }

    info!("Verifying contract address..");
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

/// ReceiverChainIsSource returns true if the denomination originally came
/// from the receiving chain and false otherwise.
fn receiver_chain_is_source(source_port: &str, source_channel: &str, denom: &str) -> bool {
    // The prefix passed in should contain the SourcePort and SourceChannel.
    // If the receiver chain originally sent the token to the sender chain
    // the denom will have the sender's SourcePort and SourceChannel as the
    // prefix.
    let voucher_prefix = get_denom_prefix(source_port, source_channel);
    denom.starts_with(&voucher_prefix)
}

/// GetDenomPrefix returns the receiving denomination prefix
fn get_denom_prefix(port_id: &str, channel_id: &str) -> String {
    format!("{}/{}/", port_id, channel_id)
}

/// DenomTrace contains the base denomination for ICS20 fungible tokens and the
/// source tracing information path.
struct DenomTrace {
    /// path defines the chain of port/channel identifiers used for tracing the
    /// source of the fungible token.
    path: String,
    /// base denomination of the relayed fungible token.
    base_denom: String,
}

impl DenomTrace {
    /// Hash returns the hex bytes of the SHA256 hash of the DenomTrace fields using the following formula:
    /// hash = sha256(tracePath + "/" + baseDenom)
    pub fn hash(&self) -> Vec<u8> {
        let hash = Sha256::digest(self.get_full_denom_path().as_bytes());
        hash.to_vec()
    }

    /// IBCDenom a coin denomination for an ICS20 fungible token in the format
    /// 'ibc/{hash(tracePath + baseDenom)}'. If the trace is empty, it will return the base denomination.
    pub fn ibc_denom(&self) -> String {
        if !self.path.is_empty() {
            format!("ibc/{}", hex::encode(self.hash()))
        } else {
            self.base_denom.clone()
        }
    }

    /// GetFullDenomPath returns the full denomination according to the ICS20 specification:
    /// tracePath + "/" + baseDenom
    /// If there exists no trace then the base denomination is returned.
    pub fn get_full_denom_path(&self) -> String {
        if self.path.is_empty() {
            self.base_denom.clone()
        } else {
            self.get_prefix() + &self.base_denom
        }
    }

    // GetPrefix returns the receiving denomination prefix composed by the trace info and a separator.
    fn get_prefix(&self) -> String {
        return format!("{}/", self.path);
    }
}

/// ParseDenomTrace parses a string with the ibc prefix (denom trace) and the base denomination
/// into a DenomTrace type.
///
/// Examples:
///
/// - "portidone/channel-0/uatom" => DenomTrace{Path: "portidone/channel-0", BaseDenom: "uatom"}
/// - "portidone/channel-0/portidtwo/channel-1/uatom" => DenomTrace{Path: "portidone/channel-0/portidtwo/channel-1", BaseDenom: "uatom"}
/// - "portidone/channel-0/gamm/pool/1" => DenomTrace{Path: "portidone/channel-0", BaseDenom: "gamm/pool/1"}
/// - "gamm/pool/1" => DenomTrace{Path: "", BaseDenom: "gamm/pool/1"}
/// - "uatom" => DenomTrace{Path: "", BaseDenom: "uatom"}
fn parse_denom_trace(raw_denom: &str) -> DenomTrace {
    let denom_split: Vec<&str> = raw_denom.split('/').collect();

    if denom_split.len() == 1 {
        return DenomTrace {
            path: "".to_string(),
            base_denom: raw_denom.to_string(),
        };
    }

    let (path, base_denom) = extract_path_and_base_from_full_denom(&denom_split);

    DenomTrace {
        path,
        base_denom,
    }
}

/// extract_path_and_base_from_full_denom returns the trace path and the base denom from
/// the elements that constitute the complete denom.
fn extract_path_and_base_from_full_denom(full_denom_items: &[&str]) -> (String, String) {
    let mut path = vec![];
    let mut base_denom = vec![];

    let length = full_denom_items.len();
    let mut i = 0;

    while i < length {
        // The IBC specification does not guarantee the expected format of the
        // destination port or destination channel identifier. A short term solution
        // to determine base denomination is to expect the channel identifier to be the
        // one ibc-go specifies. A longer term solution is to separate the path and base
        // denomination in the ICS20 packet. If an intermediate hop prefixes the full denom
        // with a channel identifier format different from our own, the base denomination
        // will be incorrectly parsed, but the token will continue to be treated correctly
        // as an IBC denomination. The hash used to store the token internally on our chain
        // will be the same value as the base denomination being correctly parsed.
        if i < length - 1 && length > 2 && is_valid_channel_id(full_denom_items[i + 1]) {
            path.push(full_denom_items[i].to_owned());
            path.push(full_denom_items[i + 1].to_owned());
            i += 2;
        } else {
            base_denom = full_denom_items[i..].to_vec();
            break;
        }
    }

    (path.join("/"), base_denom.join("/"))
}

/// IsValidChannelID checks if a channelID is valid and can be parsed to the channel
/// identifier format.
fn is_valid_channel_id(channel_id: &str) -> bool {
    parse_channel_sequence(channel_id).is_some()
}

/// ParseChannelSequence parses the channel sequence from the channel identifier.
fn parse_channel_sequence(channel_id: &str) -> Option<&str> {
    channel_id.strip_prefix("channel-")
}
