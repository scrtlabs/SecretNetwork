use cw_types_v010::types::{CanonicalAddr, HumanAddr};
use log::trace;
use std::collections::HashMap;

lazy_static::lazy_static! {
    /// Current hardcoded contract admins
    static ref HARDCODED_CONTRACT_ADMINS: HashMap<&'static str, &'static str> = HashMap::from([
        ("secret1mfk7n6mc2cg6lznujmeckdh4x0a5ezf6hx6y8q", "secret1ap26qrlp8mcq2pg6r47w43l0y8zkqm8a450s03"),
    ]);

    /// The entire history of contracts that were deployed before v1.10 and have been migrated using the hardcoded admin feature.
    /// These contracts might have other contracts that call them with a wrong code_hash, because those other contracts have it stored from before the migration.
    static ref ALLOWED_CONTRACT_CODE_HASH: HashMap<&'static str, &'static str> = HashMap::from([
        ("secret1mfk7n6mc2cg6lznujmeckdh4x0a5ezf6hx6y8q", "d45dc9b951ed5e9416bd52ccf28a629a52af0470a1a129afee7e53924416f555"),
    ]);
}

/// Current hardcoded contract admins
pub fn is_hardcoded_contract_admin(
    contract: &CanonicalAddr,
    admin: &CanonicalAddr,
    admin_proof: &[u8],
) -> bool {
    if admin_proof != [0; enclave_crypto::HASH_SIZE] {
        return false;
    }

    let contract = HumanAddr::from_canonical(contract);
    if contract.is_err() {
        trace!(
            "is_hardcoded_contract_admin: failed to convert contract to human address: {:?}",
            contract.err().unwrap()
        );
        return false;
    }
    let contract = contract.unwrap();

    let admin = HumanAddr::from_canonical(admin);
    if admin.is_err() {
        trace!(
            "is_hardcoded_contract_admin: failed to convert admin to human address: {:?}",
            admin.err().unwrap()
        );
        return false;
    }
    let admin = admin.unwrap();

    HARDCODED_CONTRACT_ADMINS.get(contract.as_str()) == Some(&admin.as_str())
}

/// The entire history of contracts that were deployed before v1.10 and have been migrated using the hardcoded admin feature.
/// These contracts might have other contracts that call them with a wrong code_hash, because those other contracts have it stored from before the migration.
pub fn is_code_hash_allowed(contract_address: &CanonicalAddr, code_hash: &str) -> bool {
    let contract_address = HumanAddr::from_canonical(contract_address);
    if contract_address.is_err() {
        trace!(
            "is_code_hash_allowed: failed to convert contract to human address: {:?}",
            contract_address.err().unwrap()
        );
        return false;
    }
    let contract = contract_address.unwrap();

    ALLOWED_CONTRACT_CODE_HASH.get(contract.as_str()) == Some(&code_hash)
}
