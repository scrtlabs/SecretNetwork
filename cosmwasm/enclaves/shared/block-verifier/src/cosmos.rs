use cosmrs::{tx as cosmtx, Tx};
use enclave_utils::tx_bytes::TxBytesForHeight;
use log::error;
use sgx_types::{sgx_status_t, SgxResult};

pub fn parse_tx(raw_txs: &TxBytesForHeight) -> SgxResult<Vec<cosmrs::Tx>> {
    let result: Result<Vec<cosmrs::Tx>, _> = raw_txs
        .txs
        .iter()
        .map_ok(|tx| {
            Tx::from_bytes(tx.tx.as_slice()).map_err(|e| {
                error!("Failed to parse tx");
                sgx_status_t::SGX_ERROR_INVALID_SIGNATURE
            })
        })
        .collect();

    result
}

pub struct TxForValidation {
    current: cosmrs::Tx,
    remaining: Vec<Tx>,
}

impl TxForValidation {
    fn get_next_sign_bytes(&mut self) {
        self.current.body.into_bytes()
    }
}

lazy_static! {
    static ref CURRENT_TX: SgxMutex = SgxMutex::new(MsgCounter::default());
}
