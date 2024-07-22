use cosmos_proto::tx::tx::Tx;
use enclave_crypto::sha_256;
use log::error;
use protobuf::Message;
use sgx_types::sgx_status_t;
use sgx_types::SgxResult;
use tendermint::block::Data;
use tendermint::crypto::default::Sha256;
use tendermint::merkle;
use tendermint_proto::v0_38::types::Data as RawData;
use tendermint_proto::Protobuf;

pub fn txs_from_bytes(raw_txs: &[u8]) -> SgxResult<Vec<Vec<u8>>> {
    let item_array = <Data as Protobuf<RawData>>::decode(raw_txs).map_err(|e| {
        error!("Error parsing tx data from proto: {:?}", e);
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    })?;

    Ok(item_array.txs)
}

pub fn tx_from_bytes(raw_tx: &[u8]) -> SgxResult<Tx> {
    let res = Tx::parse_from_bytes(raw_tx).unwrap();

    Ok(res)
}

pub fn txs_hash(txs: &Vec<Vec<u8>>) -> [u8; 32] {
    let mut vec_hashes: Vec<Vec<u8>> = vec![];

    for tx in txs {
        vec_hashes.push(sha_256(tx.as_slice()).to_vec());
    }

    merkle::simple_hash_from_byte_vectors::<Sha256>(&vec_hashes)
}
