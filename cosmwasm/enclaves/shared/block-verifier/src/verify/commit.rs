use log::error;
use sgx_types::sgx_status_t;
use tendermint::block::Commit;
use tendermint_proto::v0_38::types::Commit as RawCommit;
use tendermint_proto::Protobuf;

pub fn decode(commit_slice: &[u8]) -> Result<Commit, sgx_status_t> {
    let commit = <Commit as Protobuf<RawCommit>>::decode(commit_slice).map_err(|e| {
        error!("Error parsing commit from proto: {:?}", e);
        sgx_status_t::SGX_ERROR_INVALID_PARAMETER
    })?;

    Ok(commit)
}
