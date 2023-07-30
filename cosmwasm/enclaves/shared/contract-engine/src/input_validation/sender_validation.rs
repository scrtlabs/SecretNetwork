use cw_types_v010::types::CanonicalAddr;
use enclave_cosmos_types::types::CosmosSdkMsg;
use log::*;

pub fn verify_sender(msg: &CosmosSdkMsg, sender: &CanonicalAddr) -> Option<bool> {
    match msg {
        CosmosSdkMsg::MsgRecvPacket { .. }
        | CosmosSdkMsg::MsgAcknowledgement { .. }
        | CosmosSdkMsg::MsgTimeout { .. } => {
            // No sender to verify.
            // Going to pass null sender to the contract if all other checks pass.
        }
        CosmosSdkMsg::MsgExecuteContract { .. }
        | CosmosSdkMsg::MsgInstantiateContract { .. }
        | CosmosSdkMsg::Other => {
            if msg.sender() != Some(sender) {
                trace!(
                    "sender {:?} did not match sdk message sender: {:?}",
                    sender,
                    msg
                );
                return Some(false);
            }
        }
    }
    None
}
