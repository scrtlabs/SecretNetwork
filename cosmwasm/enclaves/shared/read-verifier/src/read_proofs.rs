use ics23::{
    iavl_spec, verify_membership, verify_non_membership, CommitmentProof, HostFunctionsManager,
};
use log::*;
use prost::Message;

use crate::READ_PROOFER;

#[derive(Default)]
pub struct ReadProofer {
    pub app_hash: [u8; 32],
    pub store_merkle_root: Vec<u8>,
}

pub fn verify_read(key: &[u8], value: Option<Vec<u8>>, proof: &CommitmentProof) -> bool {
    let root = &READ_PROOFER.lock().unwrap().store_merkle_root;

    debug!("TOMMM key {:?}", key);
    debug!("TOMMM value {:?}", value);
    debug!("TOMMM proof {:?}", proof);
    debug!("TOMMM root {:?}", root);

    match value {
        Some(v) => verify_membership::<HostFunctionsManager>(proof, &iavl_spec(), root, key, &v),
        None => verify_non_membership::<HostFunctionsManager>(proof, &iavl_spec(), root, key),
    }
}

pub fn comm_proof_from_bytes(proof: &[u8]) -> Result<CommitmentProof, prost::DecodeError> {
    CommitmentProof::decode(proof)
}
