use crate::txs;
use crate::txs::txs_hash;
use log::error;
use sgx_types::sgx_status_t;
use tendermint::block::signed_header::SignedHeader;
use tendermint::Hash::Sha256;

pub fn validate_txs(txs_slice: &[u8], header: &SignedHeader) -> Result<Vec<Vec<u8>>, sgx_status_t> {
    // validate the tx bytes with the hash in the header
    let txs = txs::txs_from_bytes(txs_slice).map_err(|e| {
        error!("Error parsing txs from proto: {:?}", e);
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    })?;

    let calculated_tx_hash = txs_hash(&txs);
    if Some(Sha256(calculated_tx_hash)) != header.header.data_hash {
        error!("Error verifying data hash");
        return Err(sgx_status_t::SGX_ERROR_INVALID_PARAMETER);
    }

    Ok(txs)
}
