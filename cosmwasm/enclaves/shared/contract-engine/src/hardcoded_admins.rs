use cw_types_v010::types::{CanonicalAddr, HumanAddr};
use log::trace;
use std::collections::HashMap;

lazy_static::lazy_static! {
    /// Current hardcoded contract admins
    static ref HARDCODED_CONTRACT_ADMINS: HashMap<&'static str, &'static str> = HashMap::from([
        ("secret14svk0x3sztxwta9kv9dv6fwzqlc26mmjfyypc2", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret13yzengut04fpk0f9hs4axvyz4np30qczt0pa7z", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1e0k5jza9jqctc5dt7mltnxmwpu3a3kqe0a6hf3", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret17aku787dnktxtagrx2vp9xp2ym4wa7ktqv5h6r", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret13w6n5u3kpvqdunkavgfy40d7ma85xuhxrcxd0a", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1aeu5lcj8dhhaae406y7g4afy5wtcgvcwdpuh6n", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1x8gs5yja6f2mmvmf5thr4r7w6kp594lrhgxclt", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1umunptajd6j3j02wchdftqkhns48ysp0tguaad", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret16xak8matccjjn4k45em9fv4j28zu2c4hdw96hg", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1khkf49xfgjtqyprd39jlyqj90axyl8kw4nlmcz", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1qa0l6tt9drkf9jk9rty3f37p23ch6vpzvgetlu", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1z6l0pg9gynzgk7qsqdaj8d9nkx6w0hctukfx4v", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1qn07k2d7hcmy8kuk7d28f5evzwygwvwvqeqzhz", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1apqgmdm7d2emufxkdujwuglrgzhsskxj8xpjls", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1t5sq6mlggs04u4ukfqyhqa00h8aehf4e62f6xm", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret16dyx6yukjg6fvdwz9935glesqvw2mtujuplq9y", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret126tc6kwgwj33vqnllytjhjlrghnrrqd2llqr9y", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1lxkltxpft6suhf63x6dvyeghqlwqldz8t2wesz", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1j22chejflprk06wv9cgz9la3tm3fkjyd92s94r", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret12k8jf45n50exzu0299lalxzr3wy02yzrrxxd38", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1uler557j3xdkqu9ua637gu2lce5557grlnw0u0", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1pdk4wj2mtkpger96lky9ptjk6zmqv7f0cz256q", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1wae7v026p9q7vapatgxwdmrmpe020wlsesxkmd", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1d5qw64q68yz2qj3qgnr5f20kemyrtnghsgngql", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1h5fmyt9424cgae4jcnre70p9s05dmmyqx66lp0", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1q9le3t099ad6nh6tm0k2lqnsq59zpa9hdzwl4a", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret12vhfcdc90tygd499ecdhsg6dwfp0p6ncrl5x2d", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1d9pj42dfgnx45uwuxlup55k2fle7d0e5u94xvg", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret16f3j4kvecpeepg7cvrdu7fmj8fmpfjt52vfjh2", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret182js23ceyjywkvnxpqd6sge6v5062uh0q4gu3c", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret16076kg6k2dvypcdx4gfnmd8swquqyv23t6jz8s", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret1emph38m50343r9tj8l79quw5kpdapeaku8yzpq", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret18mf0gg96da5jjsjfaudsuh5kgmmfmjfg4r8zjj", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret15z660976c54e8apx6q83at74ekvp787qsrast8", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
        ("secret18tnfyh4xfetqjdmy6f4hpzkqgtnt7vlvam7kj7", "secret1fz7ugsneqqru9pex7h4pjyn77s4fsmcp7sycyl"),
    ]);

    /// The entire history of contracts that were deployed before v1.10 and have been migrated using the hardcoded admin feature.
    /// These contracts might have other contracts that call them with a wrong code_hash, because those other contracts have it stored from before the migration.
    static ref ALLOWED_CONTRACT_CODE_HASH: HashMap<&'static str, &'static str> = HashMap::from([
        ("secret14svk0x3sztxwta9kv9dv6fwzqlc26mmjfyypc2", "680fbb3c8f8eb1c920da13d857daaedaa46ab8f9a8e26e892bb18a16985ec29e"),
        ("secret13yzengut04fpk0f9hs4axvyz4np30qczt0pa7z", "b08ebfdce22783cb6d0c606f4276d663d305ba268f2b2dd62414b630638e900d"),
        ("secret1e0k5jza9jqctc5dt7mltnxmwpu3a3kqe0a6hf3", "b6ec3cc640d26b6658d52e0cfb5f79abc3afd1643ec5112cfc6a9fb51d848e69"),
        ("secret17aku787dnktxtagrx2vp9xp2ym4wa7ktqv5h6r", "1f86c1b8c5b923f5ace279632e6d9fc2c9c7fdd35abad5171825698c125134f3"),
        ("secret13w6n5u3kpvqdunkavgfy40d7ma85xuhxrcxd0a", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1aeu5lcj8dhhaae406y7g4afy5wtcgvcwdpuh6n", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1x8gs5yja6f2mmvmf5thr4r7w6kp594lrhgxclt", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1umunptajd6j3j02wchdftqkhns48ysp0tguaad", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret16xak8matccjjn4k45em9fv4j28zu2c4hdw96hg", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1khkf49xfgjtqyprd39jlyqj90axyl8kw4nlmcz", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1qa0l6tt9drkf9jk9rty3f37p23ch6vpzvgetlu", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1z6l0pg9gynzgk7qsqdaj8d9nkx6w0hctukfx4v", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1qn07k2d7hcmy8kuk7d28f5evzwygwvwvqeqzhz", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1apqgmdm7d2emufxkdujwuglrgzhsskxj8xpjls", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1t5sq6mlggs04u4ukfqyhqa00h8aehf4e62f6xm", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret16dyx6yukjg6fvdwz9935glesqvw2mtujuplq9y", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret126tc6kwgwj33vqnllytjhjlrghnrrqd2llqr9y", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1lxkltxpft6suhf63x6dvyeghqlwqldz8t2wesz", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1j22chejflprk06wv9cgz9la3tm3fkjyd92s94r", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret12k8jf45n50exzu0299lalxzr3wy02yzrrxxd38", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1uler557j3xdkqu9ua637gu2lce5557grlnw0u0", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1pdk4wj2mtkpger96lky9ptjk6zmqv7f0cz256q", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1wae7v026p9q7vapatgxwdmrmpe020wlsesxkmd", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1d5qw64q68yz2qj3qgnr5f20kemyrtnghsgngql", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1h5fmyt9424cgae4jcnre70p9s05dmmyqx66lp0", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1q9le3t099ad6nh6tm0k2lqnsq59zpa9hdzwl4a", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret12vhfcdc90tygd499ecdhsg6dwfp0p6ncrl5x2d", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1d9pj42dfgnx45uwuxlup55k2fle7d0e5u94xvg", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret16f3j4kvecpeepg7cvrdu7fmj8fmpfjt52vfjh2", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret182js23ceyjywkvnxpqd6sge6v5062uh0q4gu3c", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret16076kg6k2dvypcdx4gfnmd8swquqyv23t6jz8s", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret1emph38m50343r9tj8l79quw5kpdapeaku8yzpq", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret18mf0gg96da5jjsjfaudsuh5kgmmfmjfg4r8zjj", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret15z660976c54e8apx6q83at74ekvp787qsrast8", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
        ("secret18tnfyh4xfetqjdmy6f4hpzkqgtnt7vlvam7kj7", "1691e4e24714e324a8d2345183027a918bba5c737bb2cbdbedda3cf8e7672faf"),
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
        // trace!(
        //     "is_hardcoded_contract_admin: failed to convert admin to human address: {:?}",
        //     admin.err().unwrap()
        // );
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
