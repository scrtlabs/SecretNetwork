use cw_types_v1::ibc::IbcPacketReceiveMsg;
use cw_types_v1::results::REPLY_ENCRYPTION_MAGIC_BYTES;
use log::*;

use cw_types_generic::BaseEnv;

use cw_types_v010::types::{CanonicalAddr, Coin, HumanAddr};
use enclave_cosmos_types::traits::CosmosAminoPubkey;
use enclave_cosmos_types::types::{
    ContractCode, CosmosPubKey, DirectSdkMsg, HandleType, SigInfo, SignDoc, StdSignDoc,
    VerifyParamsType,
};
use enclave_crypto::traits::VerifyingKey;
use enclave_crypto::{sha_256, AESKey, Hmac, Kdf, HASH_SIZE, KEY_MANAGER};
use enclave_ffi_types::EnclaveError;

use crate::input_validation::contract_address_validation::verify_contract_address;
use crate::input_validation::msg_validation::verify_and_get_sdk_msg;
use crate::input_validation::send_funds_validations::verify_sent_funds;
use crate::input_validation::sender_validation::verify_sender;
use crate::io::create_callback_signature;
use crate::message::is_ibc_msg;
use crate::types::SecretMessage;

#[cfg(feature = "light-client-validation")]
use block_verifier::VERIFIED_MESSAGES;

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
pub fn check_msg_matches_state(msg: &SecretMessage) -> bool {
    let mut verified_wasm_msgs = VERIFIED_MESSAGES.lock().unwrap();
    let remaining_msgs = verified_wasm_msgs.remaining();

    if remaining_msgs == 0 {
        error!("Failed to validate message, error 0x3555");
        return false;
    }

    // Msgs might fail in the sdk before they reach the enclave. In this case we need to run through
    // all the messages available before we can determine that there has been a failure
    // this isn't an attack vector since this can happen anyway by manipulating the state between executions
    let raw_msg = msg.to_vec();
    while verified_wasm_msgs.remaining() > 0 {
        if let Some(expected_msg) = verified_wasm_msgs.get_next() {
            if is_subslice(&expected_msg, &raw_msg) {
                return true;
            }
        }
    }

    error!("Failed to validate message, error 0x3255");

    // if this message fails to verify we have to fail the rest of the TX, so we won't get any
    // other messages
    verified_wasm_msgs.clear();

    false
}

/// contract_key is a unique key for each contract
/// it's used in state encryption to prevent the same
/// encryption keys from being used for different contracts
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
        // painful at this point to change the validation protocol to remove it
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

pub fn validate_current_contract_key(
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

/// validate_contract_key validates the contract key against the contract address and code hash. If the contract was previously migrated, it also validates the contract key proof against the original contract key.
pub fn validate_contract_key(
    base_env: &BaseEnv,
    canonical_contract_address: &CanonicalAddr,
    contract_code: &ContractCode,
) -> Result<[u8; CONTRACT_KEY_LENGTH], EnclaveError> {
    let current_contract_key: [u8; CONTRACT_KEY_LENGTH] = base_env.get_current_contract_key()?;

    validate_current_contract_key(
        &current_contract_key,
        canonical_contract_address,
        contract_code,
    )?;

    if base_env.was_migrated() {
        println!("Contract was migrated, validating proof");

        let og_contract_key: [u8; CONTRACT_KEY_LENGTH] = base_env.get_og_contract_key()?;
        let sent_contract_key_proof = base_env.get_current_contract_key_proof()?;

        let contract_key_proof = generate_contract_key_proof(
            &canonical_contract_address.0 .0,
            &contract_code.hash(),
            &og_contract_key,
            &current_contract_key, // this is already validated
        );

        if sent_contract_key_proof != contract_key_proof {
            error!("Failed to validate contract key proof for a migrated contract");
            return Err(EnclaveError::ValidationFailure);
        }

        // og_contract_key is always used in engine for state encryption
        Ok(og_contract_key)
    } else {
        // og_contract_key is always used in engine for state encryption
        // contract wasn't previously migrated, thus og_contract_key == current_contract_key
        Ok(current_contract_key)
    }
}

pub fn generate_contract_key_proof(
    contract_address: &[u8],
    code_hash: &[u8],
    prev_contract_key: &[u8],
    new_contract_key: &[u8],
) -> [u8; enclave_crypto::HASH_SIZE] {
    let mut data_to_hash = vec![];
    data_to_hash.extend_from_slice(contract_address);
    data_to_hash.extend_from_slice(code_hash);
    data_to_hash.extend_from_slice(prev_contract_key);
    data_to_hash.extend_from_slice(new_contract_key);

    let contract_key_proof_secret = KEY_MANAGER.get_contract_key_proof_secret().unwrap();

    data_to_hash.extend_from_slice(contract_key_proof_secret.get());

    sha_256(&data_to_hash)
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
        Some(h) => match is_ibc_msg(h) {
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

#[allow(clippy::too_many_arguments)]
pub fn verify_params(
    sig_info: &SigInfo,
    sent_funds: &[Coin],
    sender: &CanonicalAddr,
    contract_address: &HumanAddr,
    msg: &SecretMessage,
    should_verify_sig_info: bool,
    should_verify_input: bool,
    verify_params_type: VerifyParamsType,
    admin: Option<&CanonicalAddr>,
) -> Result<(), EnclaveError> {
    if should_verify_sig_info {
        debug!("Verifying message signatures for: {:?}", sig_info);

        if let Some(callback_sig) = &sig_info.callback_sig {
            return verify_callback_sig(callback_sig.as_slice(), sender, msg, sent_funds);
        }

        #[cfg(feature = "light-client-validation")]
        if !check_msg_matches_state(msg) {
            return Err(EnclaveError::ValidationFailure);
        }

        verify_signature(sig_info, sender, verify_params_type)?;
    }

    if should_verify_input {
        verify_input(
            sig_info,
            sent_funds,
            sender,
            contract_address,
            msg,
            verify_params_type,
            admin,
        )?;
    }

    info!("Parameters verified successfully");

    Ok(())
}

fn verify_signature(
    sig_info: &SigInfo,
    sender: &CanonicalAddr,
    verify_params_type: VerifyParamsType,
) -> Result<(), EnclaveError> {
    let sender_public_key = get_signer(sig_info, sender, verify_params_type)?;

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

    Ok(())
}

fn verify_input(
    sig_info: &SigInfo,
    sent_funds: &[Coin],
    sender: &CanonicalAddr,
    contract_address: &HumanAddr,
    msg: &SecretMessage,
    verify_params_types: VerifyParamsType,
    admin: Option<&CanonicalAddr>,
) -> Result<(), EnclaveError> {
    let sdk_messages = get_sdk_messages(sig_info, verify_params_types)?;

    let is_verified = verify_input_params(
        &sdk_messages,
        sender,
        sent_funds,
        contract_address,
        msg,
        verify_params_types,
        admin,
    );

    if !is_verified {
        warn!("Parameter verification failed");
        return Err(EnclaveError::FailedTxVerification);
    }

    Ok(())
}

fn get_signer(
    sign_info: &SigInfo,
    sender: &CanonicalAddr,
    verify_params_type: VerifyParamsType,
) -> Result<CosmosPubKey, EnclaveError> {
    use cosmos_proto::tx::signing::SignMode::*;
    match sign_info.sign_mode {
        SIGN_MODE_DIRECT => {
            let sign_doc =
                SignDoc::from_bytes(sign_info.sign_bytes.as_slice(), verify_params_type)?;
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

fn get_sdk_messages(
    sign_info: &SigInfo,
    verify_params_types: VerifyParamsType,
) -> Result<Vec<DirectSdkMsg>, EnclaveError> {
    use cosmos_proto::tx::signing::SignMode::*;
    match sign_info.sign_mode {
        SIGN_MODE_DIRECT => {
            let sign_doc =
                SignDoc::from_bytes(sign_info.sign_bytes.as_slice(), verify_params_types)?;
            trace!("sign doc: {:?}", sign_doc);

            Ok(sign_doc.body.messages)
        }
        SIGN_MODE_LEGACY_AMINO_JSON => {
            let sign_doc: StdSignDoc = serde_json::from_slice(sign_info.sign_bytes.as_slice())
                .map_err(|err| {
                    warn!("failure to parse StdSignDoc: {:?}", err);
                    EnclaveError::FailedTxVerification
                })?;
            let messages: Result<Vec<DirectSdkMsg>, _> = sign_doc
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
            let messages: Result<Vec<DirectSdkMsg>, _> = sign_doc
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

fn verify_input_params(
    sdk_messages: &[DirectSdkMsg],
    sender: &CanonicalAddr,
    sent_funds: &[Coin],
    contract_address: &HumanAddr,
    sent_wasm_input: &SecretMessage,
    verify_params_types: VerifyParamsType,
    admin: Option<&CanonicalAddr>,
) -> bool {
    info!("Verifying sdk message against wasm input...");
    // If msg is not found (is None) then it means message verification failed,
    // since it didn't find a matching signed message
    let sdk_msg = verify_and_get_sdk_msg(
        sdk_messages,
        sender,
        contract_address,
        sent_wasm_input,
        verify_params_types,
        admin,
    );

    let sdk_msg = match sdk_msg {
        Some(sdk_msg) => sdk_msg,
        None => {
            debug!("Message verification failed!");
            trace!(
                "Message sent to contract {:?} by {:?} does not match any signed messages {:?}",
                sent_wasm_input.to_vec(),
                sender,
                sdk_messages
            );
            return false;
        }
    };

    info!("Verifying message sender...");
    if let Some(value) = verify_sender(sdk_msg, sender) {
        return value;
    }

    info!("Verifying contract address...");
    if !verify_contract_address(sdk_msg, contract_address) {
        warn!("Contract address verification failed!");
        return false;
    }

    info!("Verifying sent funds...");
    if !verify_sent_funds(sdk_msg, sent_funds) {
        warn!("Funds verification failed!");
        return false;
    }

    true
}
