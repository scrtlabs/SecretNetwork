///
/// These functions run on-chain and must be deterministic across all nodes
///
use log::*;
use std::panic;

use enclave_ffi_types::NodeAuthResult;

use crate::registration::attestation::{
    allow_list, verify_quote_sgx, AttestationCombined, VerifiedSgxQuote,
};

use enclave_utils::{
    oom_handler::{self, get_then_clear_oom_happened},
    validate_const_ptr, validate_mut_ptr, KEY_MANAGER,
};

use sgx_types::sgx_ql_qv_result_t;

use enclave_crypto::consts::SELF_REPORT_BODY;

use super::seed_exchange::encrypt_seed;
use std::convert::TryInto;
use std::slice;

#[cfg(feature = "light-client-validation")]
use enclave_contract_engine::check_cert_in_current_block;

#[cfg(feature = "light-client-validation")]
use block_verifier::VERIFIED_BLOCK_MESSAGES;

#[cfg(feature = "light-client-validation")]
fn get_current_block_time_s() -> i64 {
    let verified_msgs = VERIFIED_BLOCK_MESSAGES.lock().unwrap();
    let tm_ns = verified_msgs.time();
    (tm_ns / 1000000000) as i64
}

#[cfg(not(feature = "light-client-validation"))]
fn get_current_block_time_s() -> i64 {
    return 0 as i64;
}

fn verify_attestation_dcap(
    attestation: &AttestationCombined,
) -> Result<VerifiedSgxQuote, NodeAuthResult> {
    let tm_s = get_current_block_time_s();
    trace!("Current block time: {}", tm_s);

    let res = match verify_quote_sgx(attestation, tm_s, true) {
        Ok(res) => {
            trace!("Remote quote verified ok");
            if res.qv_result != sgx_ql_qv_result_t::SGX_QL_QV_RESULT_OK {
                trace!("WARNING: {}", res.qv_result);
            }
            res
        }
        Err(e) => {
            trace!("Remote quote verification failed: {}", e);
            return Err(NodeAuthResult::InvalidCert);
        }
    };

    if (res.body.mr_enclave.m) != SELF_REPORT_BODY.mr_enclave.m {
        error!(
            "mrenclave expected={}, actual={}",
            hex::encode(SELF_REPORT_BODY.mr_enclave.m),
            hex::encode(res.body.mr_enclave.m)
        );
        return Err(NodeAuthResult::MrEnclaveMismatch);
    }

    Ok(res)
}

unsafe fn copy_machine_data_res(
    p_machine_data: *mut u8,
    machine: &allow_list::MachineID,
    owner: &allow_list::Owner,
) {
    slice::from_raw_parts_mut(p_machine_data, allow_list::OWNER_LEN).copy_from_slice(owner);

    slice::from_raw_parts_mut(
        p_machine_data.add(allow_list::OWNER_LEN),
        allow_list::MACHINE_ID_LEN,
    )
    .copy_from_slice(machine);
}

///
/// `ecall_authenticate_new_node`
///
/// This call is used to help new nodes register in the network. The function will authenticate the
/// new node, based on a received certificate. If the node is authenticated successfully, the seed
/// will be encrypted and shared with the registering node.
///
/// The seed is encrypted with a key derived from the secret master key of the chain, and the public
/// key of the requesting chain
///
/// This function happens on-chain, so any panic here might cause the chain to go boom
///
/// # Safety
/// Safety first
#[no_mangle]
pub unsafe extern "C" fn ecall_authenticate_new_node(
    cert: *const u8,
    cert_len: u32,
    p_seeds: *mut u8,
    n_seeds: u32,
    p_seeds_size: *mut u32,
    p_machine_pop: *const u8,
    p_machine_del_add: *mut u8,
) -> NodeAuthResult {
    if let Err(_err) = oom_handler::register_oom_handler() {
        error!("Could not register OOM handler!");
        return NodeAuthResult::MemorySafetyAllocationError;
    }

    let machine_data = allow_list::OWNER_LEN + allow_list::MACHINE_ID_LEN;

    validate_mut_ptr!(p_seeds, n_seeds as usize, NodeAuthResult::InvalidInput);
    validate_const_ptr!(
        p_machine_pop,
        allow_list::MACHINE_ID_LEN,
        NodeAuthResult::InvalidInput
    );
    validate_mut_ptr!(
        p_machine_del_add,
        machine_data * 2,
        NodeAuthResult::InvalidInput
    );
    validate_const_ptr!(cert, cert_len as usize, NodeAuthResult::InvalidInput);

    let cert_slice = std::slice::from_raw_parts(cert, cert_len as usize);

    #[cfg(feature = "light-client-validation")]
    if !check_cert_in_current_block(cert_slice) {
        return NodeAuthResult::SignatureInvalid;
    }

    let attestation = AttestationCombined::from_blob(cert, cert_len as usize);

    if attestation.quote.is_empty() || attestation.coll.is_empty() {
        warn!("No valid attestation method provided");
        return NodeAuthResult::InvalidCert;
    }

    let verified_quote = match verify_attestation_dcap(&attestation) {
        Ok(res) => res,
        Err(e) => {
            return e;
        }
    };

    let target_public_key: &[u8; 32] = verified_quote.body.report_data.d[0..32].try_into().unwrap();

    let result = panic::catch_unwind(|| -> Result<Vec<u8>, NodeAuthResult> {
        trace!(
            "ecall_get_encrypted_seed target_public_key key pk: {}",
            hex::encode(target_public_key)
        );

        let seeds = KEY_MANAGER.get_consensus_seed().unwrap();
        let mut res = Vec::new();

        for s in &seeds.arr {
            let res_current: Vec<u8> = encrypt_seed(target_public_key, s, false)
                .map_err(|_| NodeAuthResult::SeedEncryptionFailed)?;
            res.extend(&res_current);
        }

        Ok(res)
    });

    if let Err(_err) = oom_handler::restore_safety_buffer() {
        error!("Could not restore OOM safety buffer!");
        return NodeAuthResult::MemorySafetyAllocationError;
    }

    if let Ok(res) = result {
        match res {
            Ok(res) => {
                let actual_size = res.len() as u32;
                *p_seeds_size = actual_size;

                if n_seeds < actual_size {
                    warn!("insufficient seeds buffer!");
                    return NodeAuthResult::InvalidInput;
                }

                if let Some(machine_id_hash) = verified_quote.machine_id_hash {
                    // handle changes to the SGX allow-list
                    let mut allow_list = crate::registration::attestation::PPID_WHITELIST
                        .lock()
                        .unwrap();

                    let owner: &allow_list::Owner =
                        &verified_quote.body.report_data.d[32..].try_into().unwrap();

                    let machine_pop: &allow_list::MachineID =
                        slice::from_raw_parts(p_machine_pop, allow_list::MACHINE_ID_LEN)
                            .try_into()
                            .unwrap();

                    let height = {
                        let extra = KEY_MANAGER.extra_data.lock().unwrap();
                        extra.height
                    };

                    // if swap-res failed - never mind. This is probably because the machine was added with proof-of-cloud
                    if let Some(swap_res) =
                        allow_list.update(&machine_id_hash, owner, machine_pop, height)
                    {
                        copy_machine_data_res(
                            p_machine_del_add.add(machine_data),
                            &machine_id_hash,
                            owner,
                        );

                        copy_machine_data_res(p_machine_del_add, &swap_res.0, &swap_res.1);
                    }
                }

                slice::from_raw_parts_mut(p_seeds, res.len()).copy_from_slice(&res);
                trace!("returning with seed: {}, {}", res.len(), hex::encode(&res));

                NodeAuthResult::Success
            }
            Err(e) => {
                trace!("error encrypting seed {:?}", e);
                e
            }
        }
    } else {
        // There's no real need here to test if oom happened
        get_then_clear_oom_happened();
        warn!("Enclave call ecall_authenticate_new_node panic!");
        NodeAuthResult::Panic
    }
}
