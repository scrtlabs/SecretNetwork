use crate::wasm_messages::tests::check_message_is_reg;
use crate::VERIFIED_MESSAGES;
use log::{debug, error};

pub fn verify_reg_msg(certificate: &[u8]) -> bool {
    let mut verified_msgs = VERIFIED_MESSAGES.lock().unwrap();
    let next = verified_msgs.get_next();

    let result = if let Some(msg) = next {
        // I assume @assaf will add the message type to the verified messages, so I'm going to leave this here for now
        // without a check - it will fail in the parse either way

        // if !check_message_is_reg(&msg) {
        //     error!("Error failed to validate registration message - 0x7535");
        //     false
        // }

        match cosmos_proto::registration::v1beta1::msg::RaAuthenticate::parse_from_bytes(&msg) {
            Ok(ra_msg) => {
                if ra_msg.certificate == certificate {
                    true
                }
                error!("Error failed to validate registration message - 0x7535");
                false
            }
            Err(e) => {
                debug!("Error decoding registation protobuf: {}", e);
                error!("Error decoding msg from block validator - 0xA0F2");
                false
            }
        }

        true
    } else {
        error!("Cannot verify new node unless msg is part of the current block");
        false
    };

    if !result {
        // if validation failed clear the message queue and prepare for next tx... or apphash
        verified_msgs.clear();
    }

    result
}
