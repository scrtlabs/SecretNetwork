use cw_types_v010::types::CanonicalAddr;
use enclave_cosmos_types::types::DirectSdkMsg;
use log::*;

pub fn verify_sender(sdk_msg: &DirectSdkMsg, sender: &CanonicalAddr) -> Option<bool> {
    match sdk_msg {
        DirectSdkMsg::MsgRecvPacket { .. }
        | DirectSdkMsg::MsgAcknowledgement { .. }
        | DirectSdkMsg::MsgTimeout { .. } => {
            // No sender to verify.
            // Going to pass null sender to the contract if all other checks pass.
        }
        DirectSdkMsg::MsgExecuteContract { .. }
        | DirectSdkMsg::MsgInstantiateContract { .. }
        | DirectSdkMsg::MsgMigrateContract { .. }
        | DirectSdkMsg::Other => {
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
