use cw_types_v010::types::HumanAddr;
use enclave_cosmos_types::types::{
    DirectSdkMsg, FungibleTokenPacketData, IbcHooksIncomingTransferMsg,
    IbcHooksOutgoingTransferMemo, Packet,
};
use log::*;

/// Check that the contract listed in the cosmos sdk message matches the one in env
pub fn verify_contract_address(msg: &DirectSdkMsg, contract_address: &HumanAddr) -> bool {
    // Contract address is relevant only to execute, since during sending an instantiate message the contract address is not yet known
    match msg {
        DirectSdkMsg::MsgExecuteContract { contract, .. }
        | DirectSdkMsg::MsgMigrateContract { contract, .. } => {
            verify_msg_execute_or_migrate_contract_address(contract_address, contract)
        }
        // During sending an instantiate message the contract address is not yet known
        // so we cannot extract it from the message and compare it to the one in env
        DirectSdkMsg::MsgInstantiateContract { .. } => true,
        DirectSdkMsg::MsgRecvPacket {
            packet:
                Packet {
                    destination_port,
                    data,
                    ..
                },
            ..
        } => verify_contract_address_msg_recv_packet(destination_port, data, contract_address),
        DirectSdkMsg::MsgAcknowledgement {
            packet: Packet {
                source_port, data, ..
            },
            ..
        }
        | DirectSdkMsg::MsgTimeout {
            packet: Packet {
                source_port, data, ..
            },
            ..
        } => verify_contract_address_msg_ack_or_timeout(source_port, data, contract_address),
        DirectSdkMsg::Other => false,
    }
}

fn verify_msg_execute_or_migrate_contract_address(
    contract_address: &HumanAddr,
    contract: &HumanAddr,
) -> bool {
    info!("verifying contract address...");
    let is_verified = contract_address == contract;
    if !is_verified {
        trace!(
            "contract address sent to enclave {:?} is not the same as the signed one {:?}",
            contract_address,
            *contract
        );
    }
    is_verified
}

fn verify_contract_address_msg_ack_or_timeout(
    source_port: &String,
    data: &Vec<u8>,
    contract_address: &HumanAddr,
) -> bool {
    if source_port == "transfer" {
        // Packet was sent from a contract via the transfer port.
        verify_contract_address_ibc_wasm_hooks_outgoing_transfer(data, contract_address)
    } else {
        // Packet was sent from an IBC enabled contract
        verify_contract_address_ibc_contract(source_port, contract_address)
    }
}

fn verify_contract_address_ibc_wasm_hooks_outgoing_transfer(
    data: &Vec<u8>,
    contract_address: &HumanAddr,
) -> bool {
    // We're getting the ack (and timeout) here because the memo field contained `{"ibc_callback": "secret1contractAddr"}`,
    // and ibc-hooks routes the ack into `secret1contractAddr`.

    // Parse data as FungibleTokenPacketData JSON
    let packet_data: FungibleTokenPacketData = match serde_json::from_slice(data.as_slice()) {
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
    let ibc_hooks_outgoing_memo: IbcHooksOutgoingTransferMemo = match serde_json::from_slice(
        memo.as_bytes(),
    ) {
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

    let is_verified = *contract_address == ibc_hooks_outgoing_memo.ibc_callback
        && *contract_address == packet_data.sender;
    if !is_verified {
        trace!(
            "Contract address sent to enclave {:?} is not the same as in ibc-hooks outgoing transfer callback address packet {:?}",
            contract_address,
            ibc_hooks_outgoing_memo.ibc_callback
        );
    }
    is_verified
}

fn verify_contract_address_msg_recv_packet(
    destination_port: &String,
    data: &Vec<u8>,
    contract_address: &HumanAddr,
) -> bool {
    if destination_port == "transfer" {
        // Packet was routed here through ibc-hooks
        verify_contract_address_ibc_wasm_hooks_incoming_transfer(data, contract_address)
    } else {
        // Packet is for an IBC enabled contract
        verify_contract_address_ibc_contract(destination_port, contract_address)
    }
}

fn verify_contract_address_ibc_contract(port: &String, contract_address: &HumanAddr) -> bool {
    // port is of the form "wasm.{contract_address}"

    // Extract contract_address from port
    // This also checks that port starts with "wasm."
    let contract_address_from_port = match port.strip_prefix("wasm.") {
        Some(contract_address) => contract_address,
        None => {
            trace!(
                "IBC-enabled contract was called but port doesn't start with \"wasm.\": {:?}",
                port,
            );
            return false;
        }
    };

    let is_verified = *contract_address == HumanAddr::from(contract_address_from_port);
    if !is_verified {
        trace!(
            "IBC-enabled contract address sent to enclave {:?} is not the same as extracted from SDK message: {:?}",
            contract_address,
            contract_address_from_port,
        );
    }
    is_verified
}

fn verify_contract_address_ibc_wasm_hooks_incoming_transfer(
    data: &Vec<u8>,
    contract_address: &HumanAddr,
) -> bool {
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
    let is_verified =
        *contract_address == packet_data.receiver && *contract_address == wasm_msg.wasm.contract;
    if !is_verified {
        trace!(
            "Contract address sent to enclave {:?} is not the same as in ibc-hooks packet receiver={:?} memo={:?}",
            contract_address,
            packet_data.receiver,
            wasm_msg.wasm.contract
        );
    }
    is_verified
}
