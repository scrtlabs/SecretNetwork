use cw_types_v010::types::CanonicalAddr;
use enclave_cosmos_types::types::CosmosSdkMsg;
use log::*;

pub fn verify_sender(sdk_msg: &CosmosSdkMsg, sender: &CanonicalAddr) -> Option<bool> {
    match sdk_msg {
        CosmosSdkMsg::MsgRecvPacket { .. }
        | CosmosSdkMsg::MsgAcknowledgement { .. }
        | CosmosSdkMsg::MsgTimeout { .. } => {
            // No sender to verify.
            // Going to pass null sender to the contract if all other checks pass.
        }
        CosmosSdkMsg::MsgExecuteContract { .. }
        | CosmosSdkMsg::MsgInstantiateContract { .. }
        | CosmosSdkMsg::MsgMigrateContract { .. }
        | CosmosSdkMsg::Other => {
            if sdk_msg.sender() != Some(sender) {
                trace!(
                    "sender {:?} did not match sdk message sender: {:?}",
                    sender,
                    sdk_msg
                );
                return Some(false);
            }
        }
    }
    None
}
