use crate::ibc_denom_utils::{get_denom_prefix, parse_denom_trace, receiver_chain_is_source};
use cw_types_v010::types::Coin;
use enclave_cosmos_types::types::{CosmosSdkMsg, FungibleTokenPacketData, Packet};
use log::*;

/// Check that the funds listed in the cosmwasm message matches the ones in env
pub fn verify_sent_funds(msg: &CosmosSdkMsg, sent_funds_msg: &[Coin]) -> bool {
    match msg {
        CosmosSdkMsg::MsgExecuteContract { sent_funds, .. }
        | CosmosSdkMsg::MsgInstantiateContract {
            init_funds: sent_funds,
            ..
        } => sent_funds_msg == sent_funds,
        CosmosSdkMsg::Other => false,
        CosmosSdkMsg::MsgRecvPacket {
            packet:
                Packet {
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
                verify_sent_funds_ibc_wasm_hooks_incoming_transfer(
                    sent_funds_msg,
                    data,
                    source_port,
                    source_channel,
                    destination_port,
                    destination_channel,
                )
            } else {
                // Packet is for an IBC enabled contract
                // No funds should be sent
                sent_funds_msg.is_empty()
            }
        }
        CosmosSdkMsg::MsgAcknowledgement { .. } | CosmosSdkMsg::MsgTimeout { .. } => {
            sent_funds_msg.is_empty()
        }
    }
}

fn verify_sent_funds_ibc_wasm_hooks_incoming_transfer(
    sent_funds_msg: &[Coin],
    data: &Vec<u8>,
    source_port: &String,
    source_channel: &String,
    destination_port: &String,
    destination_channel: &String,
) -> bool {
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
    let packet_data: FungibleTokenPacketData = match serde_json::from_slice(data.as_slice()) {
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
    let denom: String = if receiver_chain_is_source(source_port, source_channel, &packet_data.denom)
    {
        // remove prefix added by sender chain
        let voucher_prefix = get_denom_prefix(source_port, source_channel);

        let unprefixed_denom: String = match packet_data.denom.strip_prefix(&voucher_prefix) {
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
        let prefixed_denom =
            get_denom_prefix(destination_port, destination_channel) + &packet_data.denom;
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
}
