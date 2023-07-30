use enclave_utils::validator_set::ValidatorSetForHeight;
use sgx_types::sgx_status_t;

pub fn get_validator_set_for_height() -> Result<ValidatorSetForHeight, sgx_status_t> {
    let validator_set_result = ValidatorSetForHeight::unseal()?;

    Ok(validator_set_result)
}
