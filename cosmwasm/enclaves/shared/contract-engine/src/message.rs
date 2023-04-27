use log::trace;

use enclave_cosmos_types::types::HandleType;
use enclave_ffi_types::EnclaveError;

use crate::execute_message::parse_execute_message;
use crate::ibc_message::{parse_ibc_receive_message, parse_plaintext_ibc_protocol_message};
use crate::reply_message::parse_reply_message;
use crate::types::ParsedMessage;

// Parse the message that was passed to handle (Based on the assumption that it might be a reply or IBC as well)
pub fn parse_message(
    message: &[u8],
    handle_type: &HandleType,
) -> Result<ParsedMessage, EnclaveError> {
    return match handle_type {
        HandleType::HANDLE_TYPE_EXECUTE => parse_execute_message(message),
        HandleType::HANDLE_TYPE_REPLY => parse_reply_message(message),
        HandleType::HANDLE_TYPE_IBC_CHANNEL_OPEN
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_CONNECT
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_CLOSE
        | HandleType::HANDLE_TYPE_IBC_PACKET_ACK
        | HandleType::HANDLE_TYPE_IBC_PACKET_TIMEOUT
        | HandleType::HANDLE_TYPE_IBC_WASM_HOOKS_INCOMING_TRANSFER
        | HandleType::HANDLE_TYPE_IBC_WASM_HOOKS_OUTGOING_TRANSFER_ACK
        | HandleType::HANDLE_TYPE_IBC_WASM_HOOKS_OUTGOING_TRANSFER_TIMEOUT => {
            trace!(
                "parsing {} msg (Should always be plaintext): {:?}",
                HandleType::get_export_name(handle_type),
                base64::encode(&message)
            );

            parse_plaintext_ibc_protocol_message(message)
        }
        HandleType::HANDLE_TYPE_IBC_PACKET_RECEIVE => parse_ibc_receive_message(message),
    };
}

pub fn is_ibc_msg(handle_type: HandleType) -> bool {
    matches!(
        handle_type,
        HandleType::HANDLE_TYPE_IBC_CHANNEL_OPEN
            | HandleType::HANDLE_TYPE_IBC_CHANNEL_CONNECT
            | HandleType::HANDLE_TYPE_IBC_CHANNEL_CLOSE
            | HandleType::HANDLE_TYPE_IBC_PACKET_RECEIVE
            | HandleType::HANDLE_TYPE_IBC_PACKET_ACK
            | HandleType::HANDLE_TYPE_IBC_PACKET_TIMEOUT
    )
}
