use cw_types_v010::{
    encoding::Binary,
    types::{CanonicalAddr, HumanAddr},
};
use cw_types_v1::ibc::IbcPacketReceiveMsg;
use enclave_cosmos_types::types::{
    is_transfer_ack_error, DirectSdkMsg, FungibleTokenPacketData, HandleType, IBCLifecycleComplete,
    IBCLifecycleCompleteOptions, IBCPacketAckMsg, IBCPacketTimeoutMsg, IbcHooksIncomingTransferMsg,
    IncentivizedAcknowledgement, Packet, VerifyParamsType,
};

use log::*;

use crate::types::SecretMessage;

/// Get the cosmwasm message that contains the encrypted message
pub fn verify_and_get_sdk_msg<'sd>(
    sdk_messages: &'sd [DirectSdkMsg],
    sent_sender: &CanonicalAddr,
    sent_contract_address: &HumanAddr,
    sent_wasm_input: &SecretMessage,
    verify_params_types: VerifyParamsType,
    sent_current_admin: Option<&CanonicalAddr>,
    sent_new_admin: Option<&CanonicalAddr>,
) -> Option<&'sd DirectSdkMsg> {
    trace!("verify_and_get_sdk_msg: {:?}", sdk_messages);

    sdk_messages.iter().find(|&m| match m {
        DirectSdkMsg::Other => false,
        DirectSdkMsg::MsgInstantiateContract {
            init_msg: msg,
            sender,
            admin,
            ..
        } => {
            let empty_canon = &CanonicalAddr(Binary(vec![]));
            let empty_human = HumanAddr("".to_string());

            let sent_current_admin = sent_current_admin.unwrap_or(empty_canon);
            let sent_current_admin =
                &HumanAddr::from_canonical(sent_current_admin).unwrap_or(empty_human);

            sent_current_admin == admin && sent_sender == sender && &sent_wasm_input.to_vec() == msg
        }
        DirectSdkMsg::MsgExecuteContract {
            msg,
            sender,
            contract,
            ..
        } => {
            sent_sender == sender
                && sent_contract_address == contract
                && &sent_wasm_input.to_vec() == msg
        }
        DirectSdkMsg::MsgMigrateContract {
            msg,
            sender,
            contract,
            ..
        } => {
            sent_sender == sender
                && sent_current_admin.is_some()
                && sent_current_admin.unwrap() == sender
                && sent_contract_address == contract
                && &sent_wasm_input.to_vec() == msg
        }
        DirectSdkMsg::MsgUpdateAdmin {
            sender,
            contract,
            new_admin,
        } => {
            let empty_canon = &CanonicalAddr(Binary(vec![]));
            let empty_human = HumanAddr("".to_string());

            let sent_new_admin = sent_new_admin.unwrap_or(empty_canon);
            let sent_new_admin = &HumanAddr::from_canonical(sent_new_admin).unwrap_or(empty_human);

            sent_sender == sender
                && sent_current_admin.is_some()
                && sent_current_admin.unwrap() == sender
                && sent_contract_address == contract
                && sent_new_admin == new_admin
        }
        DirectSdkMsg::MsgClearAdmin {
            sender, contract, ..
        } => {
            let empty_canon = &CanonicalAddr(Binary(vec![]));

            sent_sender == sender
                && sent_current_admin.is_some()
                && sent_current_admin.unwrap() == sender
                && sent_contract_address == contract
                && sent_new_admin == Some(empty_canon)
        }
        DirectSdkMsg::MsgRecvPacket { packet, .. } => match verify_params_types {
            VerifyParamsType::HandleType(HandleType::HANDLE_TYPE_IBC_PACKET_RECEIVE) => {
                verify_ibc_packet_recv(sent_wasm_input, packet)
            }
            VerifyParamsType::HandleType(
                HandleType::HANDLE_TYPE_IBC_WASM_HOOKS_INCOMING_TRANSFER,
            ) => verify_ibc_wasm_hooks_incoming_transfer(sent_wasm_input, packet),
            _ => false,
        },
        DirectSdkMsg::MsgAcknowledgement {
            packet,
            acknowledgement,
            signer,
            ..
        } => match verify_params_types {
            VerifyParamsType::HandleType(HandleType::HANDLE_TYPE_IBC_PACKET_ACK) => {
                verify_ibc_packet_ack(sent_wasm_input, packet, acknowledgement, signer)
            }
            VerifyParamsType::HandleType(
                HandleType::HANDLE_TYPE_IBC_WASM_HOOKS_OUTGOING_TRANSFER_ACK,
            ) => verify_ibc_wasm_hooks_outgoing_transfer_ack(
                sent_wasm_input,
                packet,
                acknowledgement,
            ),
            _ => false,
        },
        DirectSdkMsg::MsgTimeout { packet, signer, .. } => match verify_params_types {
            VerifyParamsType::HandleType(HandleType::HANDLE_TYPE_IBC_PACKET_TIMEOUT) => {
                verify_ibc_packet_timeout(sent_wasm_input, packet, signer)
            }
            VerifyParamsType::HandleType(
                HandleType::HANDLE_TYPE_IBC_WASM_HOOKS_OUTGOING_TRANSFER_TIMEOUT,
            ) => verify_ibc_wasm_hooks_outgoing_transfer_timeout(sent_wasm_input, packet),
            _ => false,
        },
    })
}

pub fn verify_ibc_packet_recv(sent_msg: &SecretMessage, packet: &Packet) -> bool {
    let Packet {
        sequence,
        source_port,
        source_channel,
        destination_port,
        destination_channel,
        data,
    } = packet;

    let parsed_sent_msg = serde_json::from_slice::<IbcPacketReceiveMsg>(&sent_msg.msg);
    if parsed_sent_msg.is_err() {
        trace!("get_verified_msg HANDLE_TYPE_IBC_PACKET_RECEIVE: sent_msg.msg cannot be parsed as IbcPacketReceiveMsg: {:?} Error: {:?}", String::from_utf8_lossy(&sent_msg.msg), parsed_sent_msg.err());

        trace!("Checking if sent_msg & data are encrypted");
        return &sent_msg.to_vec() == data;
    }
    let parsed = parsed_sent_msg.unwrap();

    // First, verify all the packet metadata - this should always match
    let metadata_valid = parsed.packet.sequence == *sequence
        && parsed.packet.src.port_id == *source_port
        && parsed.packet.src.channel_id == *source_channel
        && parsed.packet.dest.port_id == *destination_port
        && parsed.packet.dest.channel_id == *destination_channel;

    info!("IBC packet metadata valid: {}", metadata_valid);

    if !metadata_valid {
        warn!("IBC packet metadata verification failed");
        return false;
    }

    // Now verify the data - with hooks awareness
    let data_valid = verify_ibc_packet_data_with_hooks(&parsed.packet.data.as_slice(), data);

    info!("IBC packet data valid: {}", data_valid);

    metadata_valid && data_valid
}

const ZERO_SENDER: &str = "secret1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq3x5k6p";

fn verify_ibc_packet_data_with_hooks(actual_data: &[u8], expected_data: &[u8]) -> bool {
    // Parse both packets using the proper struct
    let actual_packet = match serde_json::from_slice::<FungibleTokenPacketData>(actual_data) {
        Ok(packet) => packet,
        Err(e) => {
            warn!("Failed to parse actual packet: {}", e);
            return false;
        }
    };

    let expected_packet = match serde_json::from_slice::<FungibleTokenPacketData>(expected_data) {
        Ok(packet) => packet,
        Err(e) => {
            warn!("Failed to parse expected packet: {}", e);
            return false;
        }
    };

    // Verify core fields are unchanged
    let core_valid = expected_packet.denom == actual_packet.denom
        && expected_packet.amount == actual_packet.amount
        && expected_packet.sender == actual_packet.sender
        && expected_packet.memo == actual_packet.memo;

    if !core_valid {
        warn!("Core IBC packet fields were modified");
        return false;
    }

    // Handle receiver changes
    if expected_packet.receiver == actual_packet.receiver {
        info!("Standard IBC transfer - receiver unchanged");
        return true;
    }

    // Allow receiver change to ZeroSender for IBC hooks
    if actual_packet.receiver.as_str() == ZERO_SENDER {
        info!("IBC hooks detected: receiver changed to ZeroSender");

        // Verify there's a valid wasm memo
        if let Some(memo_str) = &actual_packet.memo {
            if let Ok(memo_json) = serde_json::from_str::<serde_json::Value>(memo_str) {
                if let Some(contract_addr) = memo_json
                    .get("wasm")
                    .and_then(|w| w.get("contract"))
                    .and_then(|c| c.as_str())
                {
                    // Verify original receiver matches contract in memo
                    let valid = expected_packet.receiver.as_str() == contract_addr;
                    info!(
                        "IBC hooks validation: original='{}', contract='{}', valid={}",
                        expected_packet.receiver, contract_addr, valid
                    );
                    return valid;
                }
            }
        }
        warn!("Invalid IBC hooks: ZeroSender change without valid wasm memo");
        return false;
    }

    warn!(
        "Unauthorized receiver change: {} -> {}",
        expected_packet.receiver, actual_packet.receiver
    );
    false
}

pub fn verify_ibc_wasm_hooks_incoming_transfer(sent_msg: &SecretMessage, packet: &Packet) -> bool {
    let Packet { data, .. } = packet;

    let fungible_token_packet_data = serde_json::from_slice::<FungibleTokenPacketData>(data);
    if fungible_token_packet_data.is_err() {
        trace!("get_verified_msg HANDLE_TYPE_IBC_WASM_HOOKS_INCOMING_TRANSFER: data cannot be parsed as FungibleTokenPacketData: {:?} Error: {:?}", String::from_utf8_lossy(data), fungible_token_packet_data.err());
        return false;
    }
    let fungible_token_packet_data = fungible_token_packet_data.unwrap();

    let ibc_hooks_incoming_transfer_msg = serde_json::from_slice::<IbcHooksIncomingTransferMsg>(
        fungible_token_packet_data
            .memo
            .clone()
            .unwrap_or_default()
            .as_bytes(),
    );
    if ibc_hooks_incoming_transfer_msg.is_err() {
        trace!("get_verified_msg HANDLE_TYPE_IBC_WASM_HOOKS_INCOMING_TRANSFER: fungible_token_packet_data.memo cannot be parsed as IbcHooksIncomingTransferMsg: {:?} Error: {:?}", fungible_token_packet_data.memo, ibc_hooks_incoming_transfer_msg.err());
        return false;
    }
    let ibc_hooks_incoming_transfer_msg = ibc_hooks_incoming_transfer_msg.unwrap();
    let sent_msg_value = serde_json::from_slice::<serde_json::Value>(&sent_msg.msg);
    if sent_msg_value.is_err() {
        trace!("get_verified_msg HANDLE_TYPE_IBC_WASM_HOOKS_INCOMING_TRANSFER: sent_msg.msg cannot be parsed as serde_json::Value: {:?} Error: {:?}", String::from_utf8_lossy(&sent_msg.msg), sent_msg_value.err());
        return false;
    }

    ibc_hooks_incoming_transfer_msg.wasm.msg == sent_msg_value.unwrap()
}

pub fn verify_ibc_packet_ack(
    sent_msg: &SecretMessage,
    packet: &Packet,
    acknowledgement: &Vec<u8>,
    signer: &String,
) -> bool {
    let send_msg_ack_msg = serde_json::from_slice::<IBCPacketAckMsg>(&sent_msg.msg);
    if send_msg_ack_msg.is_err() {
        trace!("get_verified_msg HANDLE_TYPE_IBC_PACKET_ACK: sent_msg.msg cannot be parsed as IBCPacketAckMsg: {:?} Error: {:?}", String::from_utf8_lossy(&sent_msg.msg), send_msg_ack_msg.err());
        return false;
    }
    let sent_msg_ack_msg = send_msg_ack_msg.unwrap();

    let incentivized_acknowledgement =
        serde_json::from_slice::<IncentivizedAcknowledgement>(acknowledgement);
    let is_ack_verified = match incentivized_acknowledgement {
        Ok(incentivized_acknowledgement) => {
            trace!("get_verified_msg HANDLE_TYPE_IBC_PACKET_ACK is an IncentivizedAcknowledgement, using app_acknowledgement");

            sent_msg_ack_msg.acknowledgement.data
                == incentivized_acknowledgement.app_acknowledgement
        }
        Err(_) => {
            trace!(
                "get_verified_msg HANDLE_TYPE_IBC_PACKET_ACK is not an IncentivizedAcknowledgement, continuing with acknowledgement"
            );

            sent_msg_ack_msg.acknowledgement.data.0 == *acknowledgement
        }
    };

    is_ack_verified
        && sent_msg_ack_msg.original_packet.src.channel_id == packet.source_channel
        && sent_msg_ack_msg.original_packet.src.port_id == packet.source_port
        && sent_msg_ack_msg.original_packet.dest.channel_id == packet.destination_channel
        && sent_msg_ack_msg.original_packet.dest.port_id == packet.destination_port
        && sent_msg_ack_msg.original_packet.sequence == packet.sequence
        && sent_msg_ack_msg.original_packet.data.0 == packet.data
        && sent_msg_ack_msg.relayer == *signer
}

pub fn verify_ibc_wasm_hooks_outgoing_transfer_ack(
    sent_msg: &SecretMessage,
    packet: &Packet,
    acknowledgement: &[u8],
) -> bool {
    let ibc_lifecycle_complete = serde_json::from_slice::<IBCLifecycleComplete>(&sent_msg.msg);
    if ibc_lifecycle_complete.is_err() {
        trace!("get_verified_msg HANDLE_TYPE_IBC_WASM_HOOKS_OUTGOING_TRANSFER_ACK: sent_msg.msg cannot be parsed as IBCLifecycleComplete: {:?} Error: {:?}", String::from_utf8_lossy(&sent_msg.msg), ibc_lifecycle_complete.err());
        return false;
    }
    let ibc_lifecycle_complete = ibc_lifecycle_complete.unwrap();

    match ibc_lifecycle_complete {
        IBCLifecycleComplete::IBCLifecycleComplete(IBCLifecycleCompleteOptions::IBCAck {
            channel,
            sequence,
            ack,
            success,
        }) => {
            let ack_as_string = serde_json::from_slice::<String>(ack.as_bytes());
            if ack_as_string.is_err() {
                trace!("get_verified_msg HANDLE_TYPE_IBC_WASM_HOOKS_OUTGOING_TRANSFER_ACK: ack cannot be parsed as String: {:?} Error: {:?}", ack, ack_as_string.err());
                return false;
            }
            let ack_as_string = ack_as_string.unwrap();
            let ack_as_binary = Binary::from_base64(&ack_as_string);
            if ack_as_binary.is_err() {
                trace!("get_verified_msg HANDLE_TYPE_IBC_WASM_HOOKS_OUTGOING_TRANSFER_ACK: ack_as_string cannot be parsed as Binary: {:?} Error: {:?}", ack_as_string, ack_as_binary.err());
                return false;
            }
            let ack_as_binary = ack_as_binary.unwrap();

            channel == packet.source_channel
                && sequence == packet.sequence
                && ack_as_binary.as_slice() == acknowledgement
                && success != is_transfer_ack_error(acknowledgement)
        }
        _ => false,
    }
}

pub fn verify_ibc_packet_timeout(
    sent_msg: &SecretMessage,
    packet: &Packet,
    signer: &String,
) -> bool {
    let send_msg_timeout_msg = serde_json::from_slice::<IBCPacketTimeoutMsg>(&sent_msg.msg);
    if send_msg_timeout_msg.is_err() {
        trace!("get_verified_msg HANDLE_TYPE_IBC_PACKET_TIMEOUT: sent_msg.msg cannot be parsed as IBCPacketTimeoutMsg: {:?} Error: {:?}", String::from_utf8_lossy(&sent_msg.msg), send_msg_timeout_msg.err());
        return false;
    }
    let sent_msg_timeout_msg = send_msg_timeout_msg.unwrap();

    sent_msg_timeout_msg.packet.src.channel_id == packet.source_channel
        && sent_msg_timeout_msg.packet.src.port_id == packet.source_port
        && sent_msg_timeout_msg.packet.dest.channel_id == packet.destination_channel
        && sent_msg_timeout_msg.packet.dest.port_id == packet.destination_port
        && sent_msg_timeout_msg.packet.sequence == packet.sequence
        && sent_msg_timeout_msg.packet.data.0 == packet.data
        && sent_msg_timeout_msg.relayer == *signer
}

pub fn verify_ibc_wasm_hooks_outgoing_transfer_timeout(
    sent_msg: &SecretMessage,
    packet: &Packet,
) -> bool {
    let ibc_lifecycle_complete = serde_json::from_slice::<IBCLifecycleComplete>(&sent_msg.msg);
    if ibc_lifecycle_complete.is_err() {
        trace!("get_verified_msg HANDLE_TYPE_IBC_WASM_HOOKS_OUTGOING_TRANSFER_TIMEOUT: sent_msg.msg cannot be parsed as IBCLifecycleComplete: {:?} Error: {:?}", String::from_utf8_lossy(&sent_msg.msg), ibc_lifecycle_complete.err());
        return false;
    }
    let ibc_lifecycle_complete = ibc_lifecycle_complete.unwrap();

    match ibc_lifecycle_complete {
        IBCLifecycleComplete::IBCLifecycleComplete(IBCLifecycleCompleteOptions::IBCTimeout {
            channel,
            sequence,
        }) => channel == packet.source_channel && sequence == packet.sequence,
        _ => false,
    }
}
