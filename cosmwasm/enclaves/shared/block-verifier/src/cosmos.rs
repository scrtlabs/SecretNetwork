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
//     static ref CURRENT_TX: SgxMutex = SgxMutex::new(MsgCounter::default());
// }

#[cfg(feature = "test")]
pub mod tests {

    use hex;
    use cosmos_proto::tx as protoTx;
    use protobuf::Message;

    pub fn it_works() {
        let tx_bytes_hex = "0abe020abb020a2a2f7365637265742e636f6d707574652e763162657461312e4d736745786563757465436f6e7472616374128c020a14def8f4c5de676431f1bac48c892b5e4593f3b4f312143b1a7485c6162c5883ee45fb2d7477a87d8a4ce51add01a715462d5ca8feb6dceb58e21d8794e8f0257361871006e5e51a7d5e9e136e1d908024e1ea59a1008ef28c998b60a47b1836dbf70e3e8454078db37bbd0c7e3d1d5b939622fc5989ffc119684acae9a9750d910105d5aaaa6e4e008b44765802351814f3b25626dd4a545c494d174f312453ccbc88ead428d97426cedcd080b4a88d95f929bcf66693cfc497918ba94861f877df5a280a8424bfda118afd01a581dcff7bb1e983370a047275ca03a15fd2c4582e10f141a2228137c75206a975f1934050a96abe3a97d2a21aafe7845a15003d8a8cd01c0e5560d0ac2c12520a4e0a460a1f2f636f736d6f732e63727970746f2e736563703235366b312e5075624b657912230a2103fdeea779e2da196817e46ed6566eed937a00b31b3b26351fc86d7519a6ffac7f12040a02080112001a409b0cf1103b1b578fd1d4c0ea37a7ea258c39ae32918df1334c68ad18674cc1450a6433f0f693f2aee8b57e55eb184f8149e738d68cf4d3a954c43b9fab4b1f5b";

        let tx_bytes = hex::decode(tx_bytes_hex).unwrap();

        let tx = protoTx::tx::Tx::parse_from_bytes(tx_bytes.as_slice()).unwrap();

        println!("Number of messages: {}", tx.body.unwrap().messages.len())

    }
}
