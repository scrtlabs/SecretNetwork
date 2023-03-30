#![cfg(feature = "light-client-validation")]

use cosmos_proto::tx as protoTx;

use protobuf::Message;

use enclave_crypto::sha_256;
use sgx_types::SgxResult;
use tendermint::merkle;

pub fn txs_from_bytes(raw_txs: &[u8]) -> SgxResult<protoTx::tx::Txs> {
    let item_array = protoTx::tx::Txs::parse_from_bytes(raw_txs).unwrap();

    Ok(item_array)
}

pub fn tx_from_bytes(raw_tx: &[u8]) -> SgxResult<protoTx::tx::Tx> {
    let res = protoTx::tx::Tx::parse_from_bytes(raw_tx).unwrap();

    Ok(res)
}

pub fn txs_hash(txs: &protoTx::tx::Txs) -> [u8; 32] {
    let mut vec_hashes: Vec<Vec<u8>> = vec![];

    for tx in &txs.tx {
        vec_hashes.push(sha_256(tx.as_slice()).to_vec());
    }

    merkle::simple_hash_from_byte_vectors(vec_hashes)
}
