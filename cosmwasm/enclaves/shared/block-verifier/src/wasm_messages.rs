use cosmos_proto::tx::tx::Tx;
use lazy_static::lazy_static;

use std::sync::SgxMutex;
// use cosmrs::{tx as cosmtx, Tx};
// use enclave_utils::tx_bytes::TxBytesForHeight;
// use log::error;
// use sgx_types::{sgx_status_t, SgxResult};
//
// pub fn parse_tx(raw_txs: &TxBytesForHeight) -> SgxResult<Vec<cosmrs::Tx>> {
//     let result: Result<Vec<cosmrs::Tx>, _> = raw_txs
//         .txs
//         .iter()
//         .map_ok(|tx| {
//             Tx::from_bytes(tx.tx.as_slice()).map_err(|e| {
//                 error!("Failed to parse tx");
//                 sgx_status_t::SGX_ERROR_INVALID_SIGNATURE
//             })
//         })
//         .collect();
//
//     result
// }
//
// pub struct TxForValidation {
//     current: cosmrs::Tx,
//     remaining: Vec<Tx>,
// }
//
// impl TxForValidation {
//     fn get_next_sign_bytes(&mut self) {
//         self.current.body.into_bytes()
//     }
// }
//
// lazy_static! {
//   static ref CURRENT_TX: SgxMutex = SgxMutex::new(MsgCounter::default());
// }

pub fn message_is_wasm(msg: &protobuf::well_known_types::Any) -> bool {
    matches!(
        msg.type_url.as_str(),
        "/secret.compute.v1beta1.MsgExecuteContract"
            | "/secret.compute.v1beta1.MsgInstantiateContract"
    )
}

#[derive(Debug, Clone, Default)]
pub struct VerifiedWasmMessages {
    messages: Vec<Vec<u8>>,
}

impl VerifiedWasmMessages {
    pub fn get_next(&mut self) -> Option<Vec<u8>> {
        self.messages.pop()
    }

    pub fn remaining(&self) -> usize {
        self.messages.len()
    }

    pub fn append_wasm_from_tx(&mut self, mut tx: Tx) {
        for msg in tx.take_body().messages {
            if message_is_wasm(&msg) {
                self.messages.push(msg.value);
            }
        }
    }
}

lazy_static! {
    pub static ref VERIFIED_MESSAGES: SgxMutex<VerifiedWasmMessages> =
        SgxMutex::new(VerifiedWasmMessages::default());
}

#[cfg(feature = "test")]
pub mod tests {

    use cosmos_proto::tx as protoTx;
    use hex;
    use protobuf::Message;

    const TX_RAW_SINGLE_WASM_MSG: &str = "0abe020abb020a2a2f7365637265742e636f6d707574652e763162657461312e4d736745786563757465436f6e7472616374128c020a14def8f4c5de676431f1bac48c892b5e4593f3b4f312143b1a7485c6162c5883ee45fb2d7477a87d8a4ce51add01a715462d5ca8feb6dceb58e21d8794e8f0257361871006e5e51a7d5e9e136e1d908024e1ea59a1008ef28c998b60a47b1836dbf70e3e8454078db37bbd0c7e3d1d5b939622fc5989ffc119684acae9a9750d910105d5aaaa6e4e008b44765802351814f3b25626dd4a545c494d174f312453ccbc88ead428d97426cedcd080b4a88d95f929bcf66693cfc497918ba94861f877df5a280a8424bfda118afd01a581dcff7bb1e983370a047275ca03a15fd2c4582e10f141a2228137c75206a975f1934050a96abe3a97d2a21aafe7845a15003d8a8cd01c0e5560d0ac2c12520a4e0a460a1f2f636f736d6f732e63727970746f2e736563703235366b312e5075624b657912230a2103fdeea779e2da196817e46ed6566eed937a00b31b3b26351fc86d7519a6ffac7f12040a02080112001a409b0cf1103b1b578fd1d4c0ea37a7ea258c39ae32918df1334c68ad18674cc1450a6433f0f693f2aee8b57e55eb184f8149e738d68cf4d3a954c43b9fab4b1f5b";
    const TX_RAW_2_WASM_MSG: &str = "0ab4040a97020a2a2f7365637265742e636f6d707574652e763162657461312e4d736745786563757465436f6e747261637412e8010a14a082110ac6b058019d436d718a4e79f70d27357212143b1a7485c6162c5883ee45fb2d7477a87d8a4ce51a9e01c7f0e5d6f46bc1fa66b4e0e439e8b7c5d89cb20f0261b3dc21e4ed31e7752ca05f652f4faed2125bccac7851d95f906c6cb36c4132b65d6c86adf76466e3543aa5e1f66fc0d9ff4780dcadebff66c163af0b93747f3d4239a9881a2295cd425b689b023a4ecbe411cd41d28826ec4c396d8faadcf6f1fdd9077b4ea24b3f4fb6f931a046d23207bafa940de07d54c009cced68545f05e1dad766d706255e2a0c0a05617373616612033133302a0b0a0564656e6f6d120231350a97020a2a2f7365637265742e636f6d707574652e763162657461312e4d736745786563757465436f6e747261637412e8010a14a082110ac6b058019d436d718a4e79f70d27357212143b1a7485c6162c5883ee45fb2d7477a87d8a4ce51a9e01c7f0e5d6f46bc1fa66b4e0e439e8b7c5d89cb20f0261b3dc21e4ed31e7752ca05f652f4faed2125bccac7851d95f906c6cb36c4132b65d6c86adf76466e3543aa5e1f66fc0d9ff4780dcadebff66c163af0b93747f3d4239a9881a2295cd425b689b023a4ecbe411cd41d28826ec4c396d8faadcf6f1fdd9077b4ea24b3f4fb6f931a046d23207bafa940de07d54c009cced68545f05e1dad766d706255e2a0c0a05617373616612033133302a0b0a0564656e6f6d1202313512a2010a4e0a460a1f2f636f736d6f732e63727970746f2e736563703235366b312e5075624b657912230a2103b1aaf60dba87c43e1dc3e1b1b4f9c39f41fd9f97f9073106329d676517a482eb12040a0208010a4e0a460a1f2f636f736d6f732e63727970746f2e736563703235366b312e5075624b657912230a2103b1aaf60dba87c43e1dc3e1b1b4f9c39f41fd9f97f9073106329d676517a482eb12040a02080112001a40d980b90c0b67c34568872db73ef335b8445099b433c7211f0adde85179fa74da5d655ff4f4c478b38ee2fe9da49c5f321a92268b6ebd24e81bd19deb64befa461a40d980b90c0b67c34568872db73ef335b8445099b433c7211f0adde85179fa74da5d655ff4f4c478b38ee2fe9da49c5f321a92268b6ebd24e81bd19deb64befa46";
    const TX_RAW_2_WASM_1_BANK_MSG: &str = "0ad0050a97020a2a2f7365637265742e636f6d707574652e763162657461312e4d736745786563757465436f6e747261637412e8010a1406a918f7c66a8f4182f4a6304f8600c98261484712143b1a7485c6162c5883ee45fb2d7477a87d8a4ce51a9e01f1f5895860fbfc3f849e6349b801d19c6d430c64edd37c660b18f1a82f08d3ee5f652f4faed2125bccac7851d95f906c6cb36c4132b65d6c86adf76466e3543a9ac4ec5f75c7af3318a3fd66fa1a2e8747344bf02dc0e128b05ccdbac74a8b19e22957ecf787a40928091bd39e5cd2267ec477d0e5280ae6351497601b97ec79dacf22250cd79d991d9026c17258f517cc1b864d15dc510a1bf70c8022e82a0c0a05617373616612033133302a0b0a0564656e6f6d120231350a97020a2a2f7365637265742e636f6d707574652e763162657461312e4d736745786563757465436f6e747261637412e8010a1406a918f7c66a8f4182f4a6304f8600c98261484712143b1a7485c6162c5883ee45fb2d7477a87d8a4ce51a9e01f1f5895860fbfc3f849e6349b801d19c6d430c64edd37c660b18f1a82f08d3ee5f652f4faed2125bccac7851d95f906c6cb36c4132b65d6c86adf76466e3543a9ac4ec5f75c7af3318a3fd66fa1a2e8747344bf02dc0e128b05ccdbac74a8b19e22957ecf787a40928091bd39e5cd2267ec477d0e5280ae6351497601b97ec79dacf22250cd79d991d9026c17258f517cc1b864d15dc510a1bf70c8022e82a0c0a05617373616612033133302a0b0a0564656e6f6d120231350a99010a1c2f636f736d6f732e62616e6b2e763162657461312e4d736753656e6412790a2d7365637265743171363533336137786432383572716835356363796c707371657870787a6a7a38717a6e386e38122d7365637265743171363533336137786432383572716835356363796c707371657870787a6a7a38717a6e386e381a0c0a05617373616612033133301a0b0a0564656e6f6d1202313512f2010a4e0a460a1f2f636f736d6f732e63727970746f2e736563703235366b312e5075624b657912230a2103d3c85ce8007e9a745e5dc986aa721289da524c08adee427cbc58d0a0b015eaf012040a0208010a4e0a460a1f2f636f736d6f732e63727970746f2e736563703235366b312e5075624b657912230a2103d3c85ce8007e9a745e5dc986aa721289da524c08adee427cbc58d0a0b015eaf012040a0208010a4e0a460a1f2f636f736d6f732e63727970746f2e736563703235366b312e5075624b657912230a2103d3c85ce8007e9a745e5dc986aa721289da524c08adee427cbc58d0a0b015eaf012040a02080112001a40d95940e2c70ed12f223dce90e1f89f357cabdc16e234467cc2534e4554c804004b8457db52af71228726b170fa7c897352eec906e9a948ef327182725f01192a1a40d95940e2c70ed12f223dce90e1f89f357cabdc16e234467cc2534e4554c804004b8457db52af71228726b170fa7c897352eec906e9a948ef327182725f01192a1a40d95940e2c70ed12f223dce90e1f89f357cabdc16e234467cc2534e4554c804004b8457db52af71228726b170fa7c897352eec906e9a948ef327182725f01192a";

    pub fn parse_tx_basic() {
        let tx_bytes_hex = TX_RAW_SINGLE_WASM_MSG;

        let tx_bytes = hex::decode(tx_bytes_hex).unwrap();

        let tx = protoTx::tx::Tx::parse_from_bytes(tx_bytes.as_slice()).unwrap();

        assert_eq!(tx.body.unwrap().messages.len(), 1 as usize)
    }

    pub fn parse_tx_multiple_msg() {
        let tx_bytes_hex = TX_RAW_2_WASM_MSG;

        let tx_bytes = hex::decode(tx_bytes_hex).unwrap();

        let tx = protoTx::tx::Tx::parse_from_bytes(tx_bytes.as_slice()).unwrap();

        assert_eq!(tx.body.unwrap().messages.len(), 2 as usize)
    }

    pub fn parse_tx_multiple_msg_non_wasm() {
        let tx_bytes_hex = TX_RAW_2_WASM_1_BANK_MSG;

        let tx_bytes = hex::decode(tx_bytes_hex).unwrap();

        let tx = protoTx::tx::Tx::parse_from_bytes(tx_bytes.as_slice()).unwrap();

        assert_eq!(tx.body.unwrap().messages.len(), 3 as usize)
    }

    pub fn test_check_message_not_wasm() {
        let tx_bytes_hex = TX_RAW_2_WASM_1_BANK_MSG;

        let tx_bytes = hex::decode(tx_bytes_hex).unwrap();

        let tx = protoTx::tx::Tx::parse_from_bytes(tx_bytes.as_slice()).unwrap();

        let msg = tx.body.unwrap().messages[2].clone();

        assert_eq!(super::message_is_wasm(&msg), false)
    }

    pub fn check_message_is_wasm() {
        let tx_bytes_hex = TX_RAW_SINGLE_WASM_MSG;

        let tx_bytes = hex::decode(tx_bytes_hex).unwrap();

        let tx = protoTx::tx::Tx::parse_from_bytes(tx_bytes.as_slice()).unwrap();

        let msg = tx.body.unwrap().messages[0].clone();

        assert_eq!(super::message_is_wasm(&msg), true)
    }

    pub fn check_message_in_multisig() {}

    pub fn test_wasm_msg_tracker() {
        let tx_bytes_hex = TX_RAW_SINGLE_WASM_MSG;

        let tx_bytes = hex::decode(tx_bytes_hex).unwrap();

        let tx = protoTx::tx::Tx::parse_from_bytes(tx_bytes.as_slice()).unwrap();

        let ref_tx = tx.clone();

        super::VERIFIED_MESSAGES
            .lock()
            .unwrap()
            .append_wasm_from_tx(tx);

        assert_eq!(
            super::VERIFIED_MESSAGES.lock().unwrap().remaining(),
            1 as usize
        );
        assert_eq!(
            super::VERIFIED_MESSAGES.lock().unwrap().get_next().unwrap(),
            ref_tx.body.unwrap().messages[0].value
        );
        assert_eq!(
            super::VERIFIED_MESSAGES.lock().unwrap().remaining(),
            0 as usize
        );
    }

    pub fn test_wasm_msg_tracker_multiple_msgs() {
        let tx_bytes_hex = TX_RAW_2_WASM_1_BANK_MSG;

        let tx_bytes = hex::decode(tx_bytes_hex).unwrap();

        let tx = protoTx::tx::Tx::parse_from_bytes(tx_bytes.as_slice()).unwrap();

        let ref_tx = tx.clone();
        let ref_msgs = ref_tx.body.unwrap().messages;
        super::VERIFIED_MESSAGES
            .lock()
            .unwrap()
            .append_wasm_from_tx(tx);

        assert_eq!(
            super::VERIFIED_MESSAGES.lock().unwrap().remaining(),
            2 as usize
        );
        assert_eq!(
            &super::VERIFIED_MESSAGES.lock().unwrap().get_next().unwrap(),
            &ref_msgs[0].value
        );
        assert_eq!(
            super::VERIFIED_MESSAGES.lock().unwrap().remaining(),
            1 as usize
        );
        assert_eq!(
            &super::VERIFIED_MESSAGES.lock().unwrap().get_next().unwrap(),
            &ref_msgs[1].value
        );
        assert_eq!(
            super::VERIFIED_MESSAGES.lock().unwrap().remaining(),
            0 as usize
        );
    }
    //
}
