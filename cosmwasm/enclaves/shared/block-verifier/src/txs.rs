use tendermint_proto::types::Data;

// use protobuf::Message;

use enclave_crypto::sha_256;
use sgx_types::SgxResult;
use tendermint::merkle;

pub fn txs_from_bytes(raw_txs: &[u8]) -> SgxResult<Vec<Vec<u8>>> {
    let item_array: Data = bincode::serde::deserialize(raw_txs).unwrap();

    Ok(item_array.txs)
}

pub fn tx_from_bytes(raw_tx: &[u8]) -> SgxResult<Vec<u8>> {
    // let res = protoTx::tx::Tx::parse_from_bytes(raw_tx).unwrap();

    Ok(raw_tx.to_vec())
}

pub fn txs_hash(txs: &Vec<Vec<u8>>) -> [u8; 32] {
    let mut vec_hashes: Vec<Vec<u8>> = vec![];

    for tx in txs {
        vec_hashes.push(sha_256(tx.as_slice()).to_vec());
    }

    merkle::simple_hash_from_byte_vectors(vec_hashes)
}
