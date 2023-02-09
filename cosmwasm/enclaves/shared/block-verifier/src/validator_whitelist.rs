use tendermint_light_client_verifier::types::UntrustedBlockState;

const WHITELIST_FROM_FILE: &str = include_str!("../../validator_list.txt");
const VALIDATOR_THRESHOLD: usize = 3;

#[derive(Debug, Clone, Serialize, Deserialize)]
struct ValidatorList(pub Vec<String>);

// todo: add test & decode properly
fn whitelisted_validators_in_block(untrusted_block: &UntrustedBlockState) -> bool {
    let decoded: ValidatorList = base64::decode(WHITELIST_FROM_FILE.trim()).unwrap(); //will never fail since data is constant

    untrusted_block.validators.validators().iter().filter(|&a| {decoded.0.contains(&a.address.to_string())}).count() >= VALIDATOR_THRESHOLD
}
