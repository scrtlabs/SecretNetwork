use std::collections::HashMap;

use serde::{Deserialize, Serialize};

#[cfg(feature = "random")]
use cw_types_generic::{ContractFeature, CwEnv};

use cw_types_generic::{BaseAddr, BaseEnv};

use cw_types_v010::encoding::Binary;
use cw_types_v010::types::{CanonicalAddr, HumanAddr};

use enclave_cosmos_types::types::{ContractCode, HandleType, SigInfo, VerifyParamsType};
use enclave_crypto::Ed25519PublicKey;
use enclave_ffi_types::{Ctx, EnclaveError};
use log::*;

use crate::cosmwasm_config::ContractOperation;

#[cfg(feature = "light-client-validation")]
use crate::contract_validation::verify_block_info;

use crate::contract_validation::{
    generate_admin_proof, generate_contract_key_proof, ReplyParams, ValidatedMessage,
};
use crate::external::results::{
    HandleSuccess, InitSuccess, MigrateSuccess, QuerySuccess, UpdateAdminSuccess,
};
use crate::message::{is_ibc_msg, parse_message};
use crate::types::ParsedMessage;

use crate::random::update_msg_counter;

#[cfg(feature = "random")]
use crate::random::derive_random;
#[cfg(feature = "random")]
use crate::wasm3::Engine;

use super::contract_validation::{
    generate_contract_key, validate_contract_key, validate_msg, verify_params, ContractKey,
};
use super::gas::WasmCosts;
use super::io::{
    finalize_raw_output, manipulate_callback_sig_for_plaintext, post_process_output,
    set_all_logs_to_plaintext,
};
use super::types::{IoNonce, SecretMessage};

/*
Each contract is compiled with these functions already implemented in wasm:
fn cosmwasm_api_0_6() -> i32;  // Seems unused, but we should support it anyways
fn allocate(size: usize) -> *mut c_void;
fn deallocate(pointer: *mut c_void);
fn init(env_ptr: *mut c_void, msg_ptr: *mut c_void) -> *mut c_void
fn handle(env_ptr: *mut c_void, msg_ptr: *mut c_void) -> *mut c_void
fn query(msg_ptr: *mut c_void) -> *mut c_void

Re `init`, `handle` and `query`: We need to pass `env` & `msg`
down to the wasm implementations, but because they are buffers
we need to allocate memory regions inside the VM's instance and copy
`env` & `msg` into those memory regions inside the VM's instance.
*/

#[cfg_attr(feature = "cargo-clippy", allow(clippy::too_many_arguments))]
pub fn init(
    context: Ctx,       // need to pass this to read_db & write_db
    gas_limit: u64,     // gas limit for this execution
    used_gas: &mut u64, // out-parameter for gas used in execution
    contract: &[u8],    // contract wasm bytes
    env: &[u8],         // blockchain state
    msg: &[u8],         // probably function call and args
    sig_info: &[u8],    // info about signature verification
    admin: &[u8],       // admin's canonical address or null if no admin
) -> Result<InitSuccess, EnclaveError> {
    trace!("Starting init");

    //let start = Instant::now();
    let contract_code = ContractCode::new(contract);
    let contract_hash = contract_code.hash();
    // let duration = start.elapsed();
    // trace!("Time elapsed in ContractCode::new is: {:?}", duration);
    debug!(
        "******************** init RUNNING WITH CODE: {:x?}",
        contract_hash
    );

    //let start = Instant::now();
    let base_env: BaseEnv = extract_base_env(env)?;

    #[cfg(feature = "light-client-validation")]
    verify_block_info(&base_env)?;

    // let duration = start.elapsed();
    // trace!("Time elapsed in extract_base_env is: {:?}", duration);
    let query_depth = extract_query_depth(env)?;

    //let start = Instant::now();
    let (sender, contract_address, block_height, sent_funds) = base_env.get_verification_params();
    // let duration = start.elapsed();
    // trace!("Time elapsed in get_verification_paramsis: {:?}", duration);

    let canonical_contract_address = to_canonical(contract_address)?;
    let canonical_sender_address = to_canonical(sender)?;
    let canonical_admin_address = CanonicalAddr::from_vec(admin.to_vec());

    // contract_key is a unique key for each contract
    // it's used in state encryption to prevent the same
    // encryption keys from being used for different contracts
    let og_contract_key = generate_contract_key(
        &canonical_sender_address,
        &block_height,
        &contract_hash,
        &canonical_contract_address,
        None,
    )?;

    let parsed_sig_info: SigInfo = extract_sig_info(sig_info)?;

    let secret_msg = SecretMessage::from_slice(msg)?;

    //let start = Instant::now();
    verify_params(
        &parsed_sig_info,
        sent_funds,
        &canonical_sender_address,
        contract_address,
        &secret_msg,
        true,
        true,
        VerifyParamsType::Init,
        Some(&canonical_admin_address),
        None,
    )?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in verify_params: {:?}", duration);

    //let start = Instant::now();
    let decrypted_msg = secret_msg.decrypt()?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in decrypt: {:?}", duration);

    //let start = Instant::now();
    let ValidatedMessage {
        validated_msg,
        reply_params,
    } = validate_msg(
        &canonical_contract_address,
        &decrypted_msg,
        &contract_hash,
        None,
        None,
    )?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in validate_msg: {:?}", duration);

    //let start = Instant::now();
    let mut engine = start_engine(
        context,
        gas_limit,
        &contract_code,
        &og_contract_key,
        ContractOperation::Init,
        query_depth,
        secret_msg.nonce,
        secret_msg.user_public_key,
        base_env.0.block.time,
    )?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in start_engine: {:?}", duration);

    let mut versioned_env = base_env
        .clone()
        .into_versioned_env(&engine.get_api_version());

    versioned_env.set_contract_hash(&contract_hash);

    #[cfg(feature = "random")]
    set_random_in_env(
        block_height,
        &og_contract_key,
        &mut engine,
        &mut versioned_env,
    );

    update_msg_counter(block_height);
    //let start = Instant::now();
    let result = engine.init(&versioned_env, validated_msg);
    // let duration = start.elapsed();
    // trace!("Time elapsed in engine.init: {:?}", duration);

    *used_gas = engine.gas_used();

    let output = result?;

    engine
        .flush_cache()
        .map_err(|_| EnclaveError::FailedFunctionCall)?;

    // TODO: copy cosmwasm's structures to enclave
    // TODO: ref: https://github.com/CosmWasm/cosmwasm/blob/b971c037a773bf6a5f5d08a88485113d9b9e8e7b/packages/std/src/init_handle.rs#L129
    // TODO: ref: https://github.com/CosmWasm/cosmwasm/blob/b971c037a773bf6a5f5d08a88485113d9b9e8e7b/packages/std/src/query.rs#L13
    //let start = Instant::now();

    let output = post_process_output(
        output,
        &secret_msg,
        &canonical_contract_address,
        versioned_env.get_contract_hash(),
        reply_params,
        &canonical_sender_address,
        false,
        false,
    )?;

    // let duration = start.elapsed();
    // trace!("Time elapsed in encrypt_output: {:?}", duration);

    // todo: can move the key to somewhere in the output message if we want

    let admin_proof = generate_admin_proof(&canonical_admin_address.0 .0, &og_contract_key);

    Ok(InitSuccess {
        output,
        contract_key: og_contract_key,
        admin_proof,
    })
}

#[cfg(feature = "random")]
fn update_random_with_msg_counter(
    block_height: u64,
    contract_key: &[u8; 64],
    versioned_env: &mut CwEnv,
) {
    let old_random = versioned_env.get_random();
    debug!("Old random: {:x?}", old_random);

    // rand is None if env is v0.10
    if let Some(rand) = old_random {
        versioned_env.set_random(Some(derive_random(&rand, contract_key, block_height)));
    }

    debug!("New random: {:x?}", versioned_env.get_random());
}

fn to_canonical(contract_address: &BaseAddr) -> Result<CanonicalAddr, EnclaveError> {
    CanonicalAddr::from_human(contract_address).map_err(|err| {
        warn!(
            "error while trying to deserialize address from bech32 string to bytes {:?}: {}",
            contract_address, err
        );
        EnclaveError::FailedToDeserialize
    })
}

lazy_static::lazy_static! {
    /// Current hardcoded contract admins
    static ref HARDCODED_CONTRACT_ADMINS: HashMap<&'static str, &'static str> = HashMap::from([
        ("secret1k0jntykt7e4g3y88ltc60czgjuqdy4c9e8fzek", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret14mzwd0ps5q277l20ly2q3aetqe3ev4m4260gf4", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1k8cge73c3nh32d4u0dsd5dgtmk63shtlrfscj5", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1smmc5k24lcn4j2j8f3w0yaeafga6wmzl0qct03", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1zwwealwm0pcl9cul4nt6f38dsy6vzplw8lp3qg", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1ntvxnf5hzhzv8g87wn76ch6yswdujqlgmjh32w", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1rw2l7z22s3ed6dl5v70ktvnckhurldy23a3a58", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1tatdlkyznf00m3a7hftw5daaq2nk38ugfphuyr", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1grg9unv2ue8cf98t50ea45prce7gcrj2n232kq", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1dtghxvrx35nznt8es3fwxrv4qh56tvxv22z79d", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret16cwf53um7hgdvepfp3jwdzvwkt5qe2f9vfkuwv", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1kjqktuq2wq6mk7l0ecvk2cwcskjmv3ghpklctn", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1gaew7k9tv4hlx2f4wq4ta4utggj4ywpkjysqe8", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1w8d0ntrhrys4yzcfxnwprts7gfg5gfw86ccdpf", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret159p22zvq2wzsdtqhm2plp4wg33srxp2hf0qudc", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1x0dqckf2khtxyrjwhlkrx9lwwmz44k24vcv2vv", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret17gg8xcx04ldqkvkrd7r9w60rdae4ck8aslt9cf", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1h5d3555tz37crrgl5rppu2np2fhaugq3q8yvv9", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1n4dp5dk6fufqmaalu9y7pnmk2r0hs7kc66a55f", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret15rxfz2w2tallu9gr9zjxj8wav2lnz4gl9pjccj", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1vcau4rkn7mvfwl8hf0dqa9p0jr59983e3qqe3z", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1vkq022x4q8t8kx9de3r84u669l65xnwf2lg3e6", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret139qfh3nmuzfgwsx2npnmnjl4hrvj3xq5rmq8a0", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1guyayjwg5f84daaxl7w84skd8naxvq8vz9upqx", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret19xsac2kstky8nhgvvz257uszt44g0cu6ycd5e4", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1t642ayn9rhl5q9vuh4n2jkx0gpa9r6c3sl96te", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1c2prkwd8e6ratk42l4vrnwz34knfju6hmp7mg7", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1wk5j2cntwg2fgklf0uta3tlkvt87alfj7kepuw", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1egqlkasa6xe6efmfp9562sfj07lq44z7jngu5k", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret16e230j6qm5u5q30pcc6qv726ae30ak6lzq0zvf", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1tqmms5awftpuhalcv5h5mg76fa0tkdz4jv9ex4", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1yxjmepvyl2c25vnt53cr2dpn8amknwausxee83", "secret1lrnpnp6ltfxwuhjeaz97htnajh096q7y72rp5d"),
        ("secret1hvg7am0cwfu6hfnjhere35kne23f3z6z80rlty", "secret1nnt3t7ms82vf86jwq88zvwvzvm2mkhxxtevzut"),
        ("secret1tejwnma86amug6mfy74qhwclsx92zutd9rfquy", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1k5kn0a9gqap7uex0l2xj96sw6lxwqwsghewlvn", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret139gyx9n6ahk7lnq0kt0nczt3tmruzmfx0fgk4h", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1kl86lu8v3mwkjhvvfrz3p60qvmsrtyxre6d7mj", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret19qyld7sfp9xnh9qt8efllttdnxu5pt9vrmvulr", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1q08savjzkejanz2s7n56yn8ccekaj0h8d4xk7h", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1gt6g8dhdr4v7lhtkpxmvr8us9k9cd4zga7cnz9", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret19qyld7sfp9xnh9qt8efllttdnxu5pt9vrmvulr", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1v3uvahkhtzxnq0m767ekkmknlflh4y5nrvdy7l", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1fhh6fjy0wk25qcn6fd977cfwr0mzumkus33e75", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1gel0l6qwjzwnhmu9egr4alzagg7h9g3a06pk9l", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1s6eugslqmwmpkd2gt29r02tr4v2sspcmf8rflw", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1l0nmjc3kv6s57pctm84g4w7nvsdkfsk9g84ewr", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1j9mv67qjrlcmlq7d5tdeau5s4zqm22p3880e8g", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1s06m6mjmvxnrpsr8dwkndeec40u65p4ll8cs72", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1d3pjs4fh7ssjdlganmt55sm4j3gqml706ntedw", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1kd5jaxvz946scme034nrfnvp03dhct7r9tl52c", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1wjxyyklxerp00wqmc52hjxskjja5mwrm0pqy69", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret16tz5uwmv47v3jlln56fq5h2f6frl3a944ys3qk", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1h6g03h0uf9e59kmc40p7fc4kggjd4umw8u9tc6", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret13c7gglkw6hh6fl2gejswsz3pkcu00044zczrx9", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1duqnqrsnzu53z6dpvegeqjfnrzfm7c3sq09hzr", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1d3ksc0tmq2352nj4ke64emxxtvlpp24spxklkf", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1krpyrk6r83fveu5w7ukp4v6833gf79kw9tm0mu", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1jzcxa66yw4vha92202pmzwwjanljh3mm6qte6m", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1fp4p5htcs9cpqw0n8mhm9zvjsu7mn2sdx5fqxt", "secret1j7tmjrh5wkxf4yx0kas0ja4an6wktss7mvqenm"),
        ("secret1s09x2xvfd2lp2skgzm29w2xtena7s8fq98v852", "secret1jj30ulmuxem55awzhfnr802ml7rddufe0jadf7"),
        ("secret167wxv45r2m3r5krlwyjskrk4g5tvmksktvqe6t", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1qxk2scacpgj2mmm0af60674afl9e6qneg7yuny", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1mk2yt0gywtz704439mkqzjmntj09r837vc73s3", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1wdxqz26acf2e6rsac8007pd53ak7n8tgeqr46w", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret18y86hldtdp9ndj0jekcch49kwr0gwy7upe3ffw", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1jxryqg50gxppm6rukju22hw3g2rar4det40935", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1lst3x7ye06n2xthfmhs9mqtxtkhg6nnrpdwqjp", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1hcz23784w6znz3cmqml7ha8g4x6s7qq9v93mtl", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1dajnm39rdfnhxemhxqk95dmgzffltwx292l97e", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1lrtayuylgdgdc9ekqw7ln7yhujapy9dg7x5qd0", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1y6px5x7jzrk8hyvy67f06ytn8v0jwculypwxws", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1qxexanyg0gj93xulm7jex85f2p0wgjv0xsme7a", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1552yh3rplmyrjwhcxrq0egg35uy6zwjtszecf0", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret10n2xl5jmez6r9umtdrth78k0vwmce0l5m9f5dm", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1jnp0yzwdwnft4smpnnywt6yxr288xep4aur5d4", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1qctuscrtpruqdegx576uam674yw6e5culm5ajj", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1ctsxnmn4nxqrms5kf42hppzzcn7gs8uafjkv80", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1lgq7h9lmvc2pf408j2st649n52w50xln529jwg", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1aut9gnc2leamxhsa0ud76lnf4gge2y4emewrpv", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret166dngdltwaex4vfsdrv957g7qzavl309lcg3d5", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret153wu605vvp934xhd4k9dtd640zsep5jkesstdm", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1fl449muk5yq8dlad7a22nje4p5d2pnsgymhjfd", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1k6u0cy4feepm6pehnz804zmwakuwdapm69tuc4", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1ja0hcwvy76grqkpgwznxukgd7t8a8anmmx05pp", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1pjhdug87nxzv0esxasmeyfsucaj98pw4334wyc", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1qyt4l47yq3x43ezle4nwlh5q0sn6f9sesat7ap", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret10egcg03euavu336fzed87m4zdx8jkgzzz7zgmh", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1vgtmfvzdn7ztn7kcrqd7p6f2z97wvauavp3udh", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1wn9tdlvut2nz0cpv28qtv74pqx20p847j8gx3w", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1ffre8nf653pem9hn5f4ep5pg70dd837tucgdyv", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret17ue98qd2akjazu2w2r95cz06mh8pfl3v5hva4j", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1uekg0c2qenz4mxwpg5j4s439rqu25p4a6wlhk6", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1nc07allpcszfugmqdse266g4qvhmtt4gzwxdjv", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1q36njy5vvxnacsjglzsccalmst23ve7qk4dua5", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret19964kxsa07lvz7pmujehpe6mrjfqxf73m86d3j", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1salm9wmngkn4ukr30gqscmjy6yeau4q8w6esaw", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret149n35d9av2vs874nc3y34n6ukmf49f3ygsmru6", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1y5ay9sw43rqydyyds6tuam0ugt4rxxu3cmpc79", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1m393r84za0pwpzxdthhcsqj27qjl7d8ss02hwy", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1vzczp0z4edjamgcw9dc9y08v7h7vxwg5un229a", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret14xsrnkfv5r5qh7m3csps72z9vg49tkgf7an0d5", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1u3mp0jtmszw0xn7s5dn69gl0332lx9f60kt8xk", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret19wcw34ddys3d2geyunlf9hn3rz3ycf56pwxevf", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1a6efnz9y702pctmnzejzkjdyq0m62jypwsfk92", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1a9g4p64jh7cty5v544lv57yj5auynvjkv62ztf", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1zm2q7jl70cjk20tjpwflcedfch0ev64txm96zw", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1l34fyc9g23fnlk896693nw57phevnyha7pt6gj", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1zw9gwj6kx7vd3xax7wf45y6dmawkj3pd3dk7wt", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret13j4n5gj8857h2j4cnempdkfygrw9snasx4yzw2", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1fe22vmduz3xt53r5vxcmd567z08g3yryzck8az", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1c5lu8wz8cfyufng6zpx4jnygkvgsqvj0nmklwd", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret13p8tzt9knzz3eq6u05qtmwjjwzx0cgckpw22us", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1jas8rrntj4u77qu4vt5wk8y05vtcz40acp3kh9", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1xr00xvkevscgy3tqm8mnek2x5fj43r2v8wf0y5", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1jkxd060v6cl0ylj5g9lweg8vrykccpc3uauwrk", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1tscv0n6hhzfha8rnqrtvanhwa93wn3cdjzdf8q", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret19eptg5ek2n47v5t27fz373wsu0vx9c4vkgv9mu", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1mad087955ryfa8hxzjtpdrcj7m2qwz8mwa8k8a", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1u0yg9w8mhj5tlkh8cjr4vhzxwu02hrn4nxan8j", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret16xw90uydr0fplpyx2yljv692k4eem2s4v2e5u2", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret19zqa3hzgywnlt3cn9j9ml2g9uxugkte6n7kk70", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret152alvf6ha9wk3gddkslkrpdlh97w5k32nusf3l", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret10sdpvsf8jvxxed9lsv73t3feun92hq2zkhlwnr", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1nwx39c3wkz92v3mh5fauvca4ngjt76egu668r5", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1s03ypg620j7r0dg003qq30x23nmujc8a53dd99", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1ukec4axjfgqga2gz6pkvll3pmr536f2vrrasjw", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1chx2cwjn0lnn387t7krzdu4mr4997z9ehaks8v", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1ygwaq7rxlyfnungn0d268z36mm3c8un76f8atc", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1z0qac3md6ppa6nvlelx5tazr950pn80edu65dv", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1nt24y379xjn096z6ep9n0ewlyda6jdmjymf2v4", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1hnev28m6s2hkzkkdfn7m79kdxg57haacqzwu7g", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1zcu2dfs62zpc6x4zc7206r45aqkq0ja2y7kxkt", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret17d5xmnkzm2z7376587nlltqgz24jvn5s6v9arm", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1kfp76a8g9kma0rwg2xxp3xmz35f77u6a58kx30", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1ltcgd7vrdfx95048yyerlt0hna77t4crfwyd0p", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret12z88kzlqt8agtqsk50r56mxslfpx0k3lwmydu5", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1sjf4hpn0xc04n68qyxcp88rw6m6lut9uuqzjq9", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1tykpk8epqp52vtd8d7namhxpkkxxafngku60t2", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1dmxmqc094rcwdxqfvycfj953zllwe7ejvwwzek", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1ekgzws0qs854kyr6dlnj6dsvs8l4cqvpw5zax5", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1avj6r42p258ufqdf0028kfkdhnxdvjayy0rkll", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1mg86lhvjrswj732w5ztucj425fachvk65kz28s", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1gkpew7c465pppzxqxuzg94fuylxd7qepf7x8cf", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret10u7mwt8zuqg3jm0fr3n67q3l8c3tmn48nhae2y", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1daq6wanf2avekg87unx9x3ze3wsvwhtg4m20kz", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1xj2vyl0xy5evex5j7dcs700ppncmqz4fzxdfh5", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1sas56qmtsjnjf5u6ctxefazja67laf0kd5va8t", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1qgjv37xn24mf6pnurt4xqqrr73rthmech23lv4", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1t7ka0aw9gpvds5nh3ld76ep6cfgncgpydwqphn", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1y9tgcv4cf8up9kk0vsx57w8448avfszw8jmfwv", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1jdzytfds8zvpj885rk6pkqje25g73ux29rtlgw", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1qt3g0wattnh94jw5gd466wfytezuu8ekds4v8k", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1n23zgcc8qvkd6dnkwwx4jrrv488ng3znufde9j", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret12kwrx4jmzasj7sc4926l49dx5ry3rqnxzk3kny", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1973luk5acx3kda67jq55vn72h996x7ymctf7xa", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret126ncrl75d5pznp7vgpjnj5e9nksl8lwrpprvfq", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1ldt92gzs07jx5mqwtrvpev89733jn88gjp0p3w", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1wjjqxf4gmxgg22926q32cyv4q98wp3fa8erqx2", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1g2c90l9x8kqdva22v0kp6sp5d55f4cjtw2a3w8", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1kw8d63a3945r42rgcx5x68f3a6ecfsxtg4zk46", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1lrlfevkpmwc0kfxl9e59x0er5d8pzh48t68m0e", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret10jcfg560hymw7zmua2rq5h4n2gz4hggmx3sa6h", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1ctgxt7tqrpjxqcqpz46hcch5cghcvx2kxkn4k7", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1cqk6t9jjzqelwm0f72n5u2utvljdfgsq047cqu", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1qptd85mmy0g250xqq76km3804k9ka950435hck", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1cxr62nxugnxmpde44spjpy5urqgwcfvrtdtnqg", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1qz57pea4k3ndmjpy6tdjcuq4tzrvjn0aphca0k", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1gcq0jyy07fkg7q8ekhhw9asgza28w3v65e2qtv", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1l0f53wjf0x8qdylrcha888gg4r5vrvlhhtpl0g", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret10szrjlyza5u7yqcqvqenf28nmhwph4pad9csyw", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1grwgyezs60v08683ncs6lep9f09zrzk5jf5d0w", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1sk5fj35xe0wdagu7dermas9q2u3tl4smvfahpz", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret19nldywqd78rwf0vd7srg7nr76u2sxzekt64pg0", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret10qhn3vtpln9g20syecctufnz6am673jqfr6wxd", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1sdcqvyv96jk324y9vq9u6nljxs7palu85nh0wj", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1a65a9xgqrlsgdszqjtxhz069pgsh8h4a83hwt0", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1kmjr03phgn4v4u0altvvuc53lfmy033wmvddy5", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1hh9kgm00kfcjc78kefsf29g0fvxnd3f2tt9lrs", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1gxqsuht45uh2tpqdpru6z6tsw3uyll6md7mzka", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1zwvfkzeslfcytw6elp4yj20v8vd0l8ws0j9llp", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1ygauj7gn3f4skj3x09erxhkujftu89s05drhyc", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret12wxpcquw2jx6an6da5nxyz6l7qd955u23ljcjn", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1lzdv4s665m42ge6ya063xqa7zn3sa7jeqzrccu", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1v3v08kj7ngca3686hma5k02j8whdzp57qd4a8d", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1y6w45fwg9ln9pxd6qys8ltjlntu9xa4f2de7sp", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1tv80wnyljtre8l8mfvdr77tp59mq7wf94sgf3e", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret18dlxp9zu8kgkrr4qvlwdktvfdj9xen3kddc97j", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1dw4kkuh4h88a6g3spqyu7gkt3v0mqf8rl88cfv", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1uacy0hjvymf7khrweekmnh5qgr553x0qn3n49h", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1rrwyqw9rx6rjyp6f6k05uwdemqxx0kltapkvca", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1c26v64jmesejsauxx5uamaycfe4zt3rth3yg4e", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret17nmgfelgmmzdnzpfgr0g09kfjyk6sn5l9s0m2x", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1qvgkgtnelmqf2m6kjdaetws2geukdfpyp8t7qz", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret18537ttv4l4k2ea0xp6ay3sv4c243fyjtj2uqz7", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1l2u35dcx2a4wyx9a6lxn9va6e66z493ycqxtmx", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret16h5sqd79x43wutne8ge3pdz3e3lngw62vy5lmr", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1f6kw62rzgn3fwc0jfp7nxjks0l45jv3r6tpc0x", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret15a09wzvz3wlem2cfuwnphh46te2pnmk6263c6g", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf"),
        ("secret1mr0eu9smlq4ac97rhr3np0nl8yq7k6n9gjm9t2", "secret1y277c499f44nxe7geeaqw8t6gpge68rcpla9lf")
    ]);

    /// The entire history of contracts that were deployed before v1.10 and have been migrated using the hardcoded admin feature.
    /// These contracts might have other contracts that call them with a wrong code_hash, because those other contracts have it stored from before the migration.
    static ref ALLOWED_CONTRACT_CODE_HASH: HashMap<&'static str, &'static str> = HashMap::from([
    (
        "secret1exampleContractAddress1",
        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    ),
    (
        "secret1exampleContractAddress2",
        "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    ),
]);
}

/// Current hardcoded contract admins
fn is_hardcoded_contract_admin(
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

#[cfg_attr(feature = "cargo-clippy", allow(clippy::too_many_arguments))]
pub fn migrate(
    context: Ctx,
    gas_limit: u64,
    used_gas: &mut u64,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
    sig_info: &[u8],
    admin: &[u8],
    admin_proof: &[u8],
) -> Result<MigrateSuccess, EnclaveError> {
    debug!("Starting migrate");

    //let start = Instant::now();
    let contract_code = ContractCode::new(contract);
    let contract_hash = contract_code.hash();
    // let duration = start.elapsed();
    // trace!("Time elapsed in ContractCode::new is: {:?}", duration);
    debug!(
        "******************** migrate RUNNING WITH CODE: {:x?}",
        contract_hash
    );

    //let start = Instant::now();
    let base_env: BaseEnv = extract_base_env(env)?;

    #[cfg(feature = "light-client-validation")]
    verify_block_info(&base_env)?;

    // let duration = start.elapsed();
    // trace!("Time elapsed in extract_base_env is: {:?}", duration);
    let query_depth = extract_query_depth(env)?;

    //let start = Instant::now();
    let (sender, contract_address, block_height, sent_funds) = base_env.get_verification_params();
    // let duration = start.elapsed();
    // trace!("Time elapsed in get_verification_paramsis: {:?}", duration);

    let canonical_contract_address = to_canonical(contract_address)?;
    let canonical_sender_address = to_canonical(sender)?;
    let canonical_admin_address = CanonicalAddr::from_vec(admin.to_vec());

    let og_contract_key = base_env.get_og_contract_key()?;

    if is_hardcoded_contract_admin(
        &canonical_contract_address,
        &canonical_admin_address,
        admin_proof,
    ) {
        debug!("Found hardcoded admin for migrate");
    } else {
        let sender_admin_proof =
            generate_admin_proof(&canonical_sender_address.0 .0, &og_contract_key);

        if admin_proof != sender_admin_proof {
            error!("Failed to validate sender as current admin for migrate");
            return Err(EnclaveError::ValidationFailure);
        }
        debug!("Validated migrate proof successfully");
    }

    let parsed_sig_info: SigInfo = extract_sig_info(sig_info)?;

    let secret_msg = SecretMessage::from_slice(msg)?;

    //let start = Instant::now();
    verify_params(
        &parsed_sig_info,
        sent_funds,
        &canonical_sender_address,
        contract_address,
        &secret_msg,
        true,
        true,
        VerifyParamsType::Migrate,
        Some(&canonical_admin_address),
        None,
    )?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in verify_params: {:?}", duration);

    //let start = Instant::now();
    let decrypted_msg = secret_msg.decrypt()?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in decrypt: {:?}", duration);

    //let start = Instant::now();
    let ValidatedMessage {
        validated_msg,
        reply_params,
    } = validate_msg(
        &canonical_contract_address,
        &decrypted_msg,
        &contract_hash,
        None,
        None,
    )?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in validate_msg: {:?}", duration);

    //let start = Instant::now();
    let mut engine = start_engine(
        context,
        gas_limit,
        &contract_code,
        &og_contract_key,
        ContractOperation::Migrate,
        query_depth,
        secret_msg.nonce,
        secret_msg.user_public_key,
        base_env.0.block.time,
    )?;
    // let duration = start.elapsed();
    // trace!("Time elapsed in start_engine: {:?}", duration);

    let mut versioned_env = base_env.into_versioned_env(&engine.get_api_version());

    versioned_env.set_contract_hash(&contract_hash);

    let new_contract_key = generate_contract_key(
        &canonical_sender_address,
        &block_height,
        &contract_hash,
        &canonical_contract_address,
        Some(&og_contract_key),
    )?;

    #[cfg(feature = "random")]
    set_random_in_env(
        block_height,
        &new_contract_key,
        &mut engine,
        &mut versioned_env,
    );

    update_msg_counter(block_height);
    let result = engine.migrate(&versioned_env, validated_msg);

    *used_gas = engine.gas_used();

    let output = result?;

    engine
        .flush_cache()
        .map_err(|_| EnclaveError::FailedFunctionCall)?;

    let output = post_process_output(
        output,
        &secret_msg,
        &canonical_contract_address,
        versioned_env.get_contract_hash(),
        reply_params,
        &canonical_sender_address,
        false,
        false,
    )?;

    // let duration = start.elapsed();
    // trace!("Time elapsed in encrypt_output: {:?}", duration);

    // todo: can move the key to somewhere in the output message if we want

    let new_contract_key_proof = generate_contract_key_proof(
        &canonical_contract_address.0 .0,
        &contract_code.hash(),
        &og_contract_key,
        &new_contract_key,
    );

    debug!(
        "Migrate success: {:x?}, {:x?}",
        new_contract_key, new_contract_key_proof
    );

    Ok(MigrateSuccess {
        output,
        new_contract_key,
        new_contract_key_proof,
    })
}

pub fn update_admin(
    env: &[u8],
    sig_info: &[u8],
    current_admin: &[u8],
    current_admin_proof: &[u8],
    new_admin: &[u8],
) -> Result<UpdateAdminSuccess, EnclaveError> {
    debug!("Starting update_admin");

    let base_env: BaseEnv = extract_base_env(env)?;

    #[cfg(feature = "light-client-validation")]
    verify_block_info(&base_env)?;

    let (sender, contract_address, _block_height, sent_funds) = base_env.get_verification_params();

    let canonical_sender_address = to_canonical(sender)?;
    let canonical_current_admin_address = CanonicalAddr::from_vec(current_admin.to_vec());
    let canonical_new_admin_address = CanonicalAddr::from_vec(new_admin.to_vec());

    let canonical_contract_address = to_canonical(contract_address)?;

    if is_hardcoded_contract_admin(
        &canonical_contract_address,
        &canonical_current_admin_address,
        current_admin_proof,
    ) {
        debug!(
            "Found hardcoded admin for update_admin. Cannot update admin for hardcoded contracts."
        );
        return Err(EnclaveError::ValidationFailure);
    }

    let og_contract_key = base_env.get_og_contract_key()?;

    let sender_admin_proof = generate_admin_proof(&canonical_sender_address.0 .0, &og_contract_key);

    if sender_admin_proof != current_admin_proof {
        error!("Failed to validate sender as current admin for update_admin");
        return Err(EnclaveError::ValidationFailure);
    }
    debug!("Validated update_admin proof successfully");

    let parsed_sig_info: SigInfo = extract_sig_info(sig_info)?;

    verify_params(
        &parsed_sig_info,
        sent_funds,
        &canonical_sender_address,
        contract_address,
        &SecretMessage {
            nonce: [0; 32],
            user_public_key: [0; 32],
            msg: vec![], // must be empty vec for callback_sig verification
        },
        true,
        true,
        VerifyParamsType::UpdateAdmin,
        Some(&canonical_current_admin_address),
        Some(&canonical_new_admin_address),
    )?;

    let new_admin_proof = generate_admin_proof(&canonical_new_admin_address.0 .0, &og_contract_key);

    debug!("update_admin success: {:?}", new_admin_proof);

    Ok(UpdateAdminSuccess { new_admin_proof })
}

#[cfg_attr(feature = "cargo-clippy", allow(clippy::too_many_arguments))]
pub fn handle(
    context: Ctx,
    gas_limit: u64,
    used_gas: &mut u64,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
    sig_info: &[u8],
    handle_type: u8,
) -> Result<HandleSuccess, EnclaveError> {
    trace!("Starting handle");

    let contract_code = ContractCode::new(contract);
    let contract_hash = contract_code.hash();

    debug!(
        "******************** HANDLE RUNNING WITH CODE: {:x?}",
        contract_hash
    );

    let base_env: BaseEnv = extract_base_env(env)?;

    #[cfg(feature = "light-client-validation")]
    verify_block_info(&base_env)?;

    let query_depth = extract_query_depth(env)?;

    let (sender, contract_address, block_height, sent_funds) = base_env.get_verification_params();

    let canonical_contract_address = to_canonical(contract_address)?;

    validate_contract_key(&base_env, &canonical_contract_address, &contract_code)?;

    let parsed_sig_info: SigInfo = extract_sig_info(sig_info)?;

    // The flow of handle is now used for multiple messages (such ash Handle, Reply, IBC)
    // When the message is handle, we expect it always to be encrypted while in Reply & IBC it might be plaintext
    let parsed_handle_type = HandleType::try_from(handle_type)?;

    trace!("Handle type is {:?}", parsed_handle_type);

    let ParsedMessage {
        should_verify_sig_info,
        should_verify_input,
        was_msg_encrypted,
        should_encrypt_output,
        secret_msg,
        decrypted_msg,
        data_for_validation,
    } = parse_message(msg, &parsed_handle_type)?;

    let canonical_sender_address = match to_canonical(sender) {
        Ok(can) => can,
        Err(_) => CanonicalAddr::from_vec(vec![]),
    };

    // There is no signature to verify when the input isn't signed.
    // Receiving an unsigned messages is only possible in Handle (Init tx are always signed).
    // All of these scenarios go through here but the data isn't signed:
    // - Plaintext replies (resulting from an IBC call)
    // - IBC WASM Hooks
    // - (In the future:) ICA
    verify_params(
        &parsed_sig_info,
        sent_funds,
        &canonical_sender_address,
        contract_address,
        &secret_msg,
        should_verify_sig_info,
        should_verify_input,
        VerifyParamsType::HandleType(parsed_handle_type),
        None,
        None,
    )?;

    let mut validated_msg = decrypted_msg.clone();
    let mut reply_params: Option<Vec<ReplyParams>> = None;
    if was_msg_encrypted {
        let x = validate_msg(
            &canonical_contract_address,
            &decrypted_msg,
            &contract_hash,
            data_for_validation,
            Some(parsed_handle_type),
        )?;
        validated_msg = x.validated_msg;
        reply_params = x.reply_params;
    }

    let og_contract_key = base_env.get_og_contract_key()?;

    // Although the operation here is not always handle it is irrelevant in this case
    // because it only helps to decide whether to check floating points or not
    // In this case we want to do the same as in Handle both for Reply and for others so we can always pass "Handle".
    let mut engine = start_engine(
        context,
        gas_limit,
        &contract_code,
        &og_contract_key,
        ContractOperation::Handle,
        query_depth,
        secret_msg.nonce,
        secret_msg.user_public_key,
        base_env.0.block.time,
    )?;

    let mut versioned_env = base_env
        .clone()
        .into_versioned_env(&engine.get_api_version());

    // We want to allow executing contracts with plaintext input via IBC,
    // even though the sender of an IBC packet cannot be verified.
    // But we don't want malicious actors using this enclave setting to fake any sender they want.
    // Therefore we'll use a null sender if it cannot be verified.
    match parsed_handle_type {
        // Execute: msg.sender was already verified
        HandleType::HANDLE_TYPE_EXECUTE => {}
        // Reply & IBC stuff: no msg.sender, set it to null just in case
        // WASM Hooks: cannot verify sender, set it to null
        HandleType::HANDLE_TYPE_REPLY
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_OPEN
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_CONNECT
        | HandleType::HANDLE_TYPE_IBC_CHANNEL_CLOSE
        | HandleType::HANDLE_TYPE_IBC_PACKET_RECEIVE
        | HandleType::HANDLE_TYPE_IBC_PACKET_ACK
        | HandleType::HANDLE_TYPE_IBC_PACKET_TIMEOUT
        | HandleType::HANDLE_TYPE_IBC_WASM_HOOKS_INCOMING_TRANSFER
        | HandleType::HANDLE_TYPE_IBC_WASM_HOOKS_OUTGOING_TRANSFER_ACK
        | HandleType::HANDLE_TYPE_IBC_WASM_HOOKS_OUTGOING_TRANSFER_TIMEOUT => {
            versioned_env.set_msg_sender("")
        }
    }

    #[cfg(feature = "random")]
    {
        let contract_key_for_random = base_env.get_latest_contract_key()?;
        set_random_in_env(
            block_height,
            &contract_key_for_random,
            &mut engine,
            &mut versioned_env,
        );
    }

    versioned_env.set_contract_hash(&contract_hash);

    update_msg_counter(block_height);

    let result = engine.handle(&versioned_env, validated_msg, &parsed_handle_type);

    *used_gas = engine.gas_used();

    let mut output = result?;

    // This gets refunded because it will get charged later by the sdk
    let refund_cache_gas = engine
        .flush_cache()
        .map_err(|_| EnclaveError::FailedFunctionCall)?;
    *used_gas = used_gas.saturating_sub(refund_cache_gas);

    debug!(
        "(2) nonce just before encrypt_output: nonce = {:x?} pubkey = {:x?}",
        secret_msg.nonce, secret_msg.user_public_key
    );
    if should_encrypt_output {
        output = post_process_output(
            output,
            &secret_msg,
            &canonical_contract_address,
            versioned_env.get_contract_hash(),
            reply_params,
            &canonical_sender_address,
            false,
            is_ibc_msg(parsed_handle_type),
        )?;
    } else {
        let mut raw_output =
            manipulate_callback_sig_for_plaintext(&canonical_contract_address, output)?;
        set_all_logs_to_plaintext(&mut raw_output);

        output = finalize_raw_output(raw_output, false, is_ibc_msg(parsed_handle_type), false)?;
    }

    Ok(HandleSuccess { output })
}

#[cfg(feature = "random")]
fn set_random_in_env(
    block_height: u64,
    contract_key: &[u8; 64],
    engine: &mut Engine,
    versioned_env: &mut CwEnv,
) {
    {
        if engine
            .supported_features()
            .contains(&ContractFeature::Random)
        {
            debug!("random is enabled by contract");
            update_random_with_msg_counter(block_height, contract_key, versioned_env);
        } else {
            versioned_env.set_random(None);
        }
    }
}

fn extract_sig_info(sig_info: &[u8]) -> Result<SigInfo, EnclaveError> {
    serde_json::from_slice(sig_info).map_err(|err| {
        warn!(
            "handle got an error while trying to deserialize sig info input bytes into json {:?}: {}",
            String::from_utf8_lossy(sig_info),
            err
        );
        EnclaveError::FailedToDeserialize
    })
}

pub fn query(
    context: Ctx,
    gas_limit: u64,
    used_gas: &mut u64,
    contract: &[u8],
    env: &[u8],
    msg: &[u8],
) -> Result<QuerySuccess, EnclaveError> {
    trace!("Entered query");

    let contract_code = ContractCode::new(contract);
    let contract_hash = contract_code.hash();

    let base_env: BaseEnv = extract_base_env(env)?;
    let query_depth = extract_query_depth(env)?;

    let (_, contract_address, _, _) = base_env.get_verification_params();

    let canonical_contract_address = to_canonical(contract_address)?;

    validate_contract_key(&base_env, &canonical_contract_address, &contract_code)?;

    let secret_msg = SecretMessage::from_slice(msg)?;
    let decrypted_msg = secret_msg.decrypt()?;

    let ValidatedMessage { validated_msg, .. } = validate_msg(
        &canonical_contract_address,
        &decrypted_msg,
        &contract_hash,
        None,
        None,
    )?;

    let og_contract_key = base_env.get_og_contract_key()?;

    let mut engine = start_engine(
        context,
        gas_limit,
        &contract_code,
        &og_contract_key,
        ContractOperation::Query,
        query_depth,
        secret_msg.nonce,
        secret_msg.user_public_key,
        base_env.0.block.time,
    )?;

    let mut versioned_env = base_env
        .clone()
        .into_versioned_env(&engine.get_api_version());

    versioned_env.set_contract_hash(&contract_hash);

    let result = engine.query(&versioned_env, validated_msg);
    *used_gas = engine.gas_used();
    let output = result?;

    let output = post_process_output(
        output,
        &secret_msg,
        &CanonicalAddr(Binary(Vec::new())), // Not used for queries (can't init a new contract from a query)
        "",   // Not used for queries (can't call a sub-message from a query),
        None, // Not used for queries (Query response is not replied to the caller),
        &CanonicalAddr(Binary(Vec::new())), // Not used for queries (used only for replies)
        true,
        false,
    )?;

    Ok(QuerySuccess { output })
}

#[allow(clippy::too_many_arguments)]
fn start_engine(
    context: Ctx,
    gas_limit: u64,
    contract_code: &ContractCode,
    og_contract_key: &ContractKey,
    operation: ContractOperation,
    query_depth: u32,
    nonce: IoNonce,
    user_public_key: Ed25519PublicKey,
    timestamp: u64,
) -> Result<crate::wasm3::Engine, EnclaveError> {
    crate::wasm3::Engine::new(
        context,
        gas_limit,
        WasmCosts::default(),
        contract_code,
        *og_contract_key,
        operation,
        nonce,
        user_public_key,
        query_depth,
        timestamp,
    )
}

fn extract_base_env(env: &[u8]) -> Result<BaseEnv, EnclaveError> {
    serde_json::from_slice(env)
        .map_err(|err| {
            warn!(
                "error while deserializing env from json {:?}: {}",
                String::from_utf8_lossy(env),
                err
            );
            EnclaveError::FailedToDeserialize
        })
        .map(|base_env| {
            trace!("base env: {:?}", base_env);
            base_env
        })
}

#[derive(Debug, Serialize, Deserialize)]
struct EnvWithQD {
    query_depth: u32,
}

/// Extract the query_depth from the env parameter.
///
/// This is done in a separate method and type definition in order
/// to simplify the code and avoid further coupling of the query depth
/// parameter and the CW Env type.
fn extract_query_depth(env: &[u8]) -> Result<u32, EnclaveError> {
    serde_json::from_slice::<EnvWithQD>(env)
        .map_err(|err| {
            warn!(
                "error while deserializing env into json {:?}: {}",
                String::from_utf8_lossy(env),
                err
            );
            EnclaveError::FailedToDeserialize
        })
        .map(|env| {
            trace!("env.query_depth: {:?}", env);
            env.query_depth
        })
}
