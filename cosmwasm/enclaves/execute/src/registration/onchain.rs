///
/// These functions run on-chain and must be deterministic across all nodes
///
use log::*;
use std::panic;

use enclave_ffi_types::NodeAuthResult;

use crate::registration::attestation::{verify_quote_sgx, AttestationCombined};

use enclave_utils::{
    oom_handler::{self, get_then_clear_oom_happened},
    validate_const_ptr, validate_mut_ptr, KEY_MANAGER,
};

use sgx_types::sgx_ql_qv_result_t;

use enclave_crypto::consts::SELF_REPORT_BODY;

use super::seed_exchange::encrypt_seed;
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
    pub_key: &mut [u8; 32],
) -> NodeAuthResult {
    let tm_s = get_current_block_time_s();
    trace!("Current block time: {}", tm_s);

    let report_body = match verify_quote_sgx(attestation, tm_s, true) {
        Ok(r) => {
            trace!("Remote quote verified ok");
            if r.1 != sgx_ql_qv_result_t::SGX_QL_QV_RESULT_OK {
                trace!("WARNING: {}", r.1);
            }
            r.0
        }
        Err(e) => {
            trace!("Remote quote verification failed: {}", e);
            return NodeAuthResult::InvalidCert;
        }
    };

    if (report_body.mr_enclave.m) != SELF_REPORT_BODY.mr_enclave.m {
        error!(
            "mrenclave expected={}, actual={}",
            hex::encode(SELF_REPORT_BODY.mr_enclave.m),
            hex::encode(report_body.mr_enclave.m)
        );
        return NodeAuthResult::MrEnclaveMismatch;
    }

    pub_key.copy_from_slice(&report_body.report_data.d[..32]);

    NodeAuthResult::Success
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
) -> NodeAuthResult {
    if let Err(_err) = oom_handler::register_oom_handler() {
        error!("Could not register OOM handler!");
        return NodeAuthResult::MemorySafetyAllocationError;
    }

    validate_mut_ptr!(p_seeds, n_seeds as usize, NodeAuthResult::InvalidInput);
    validate_const_ptr!(cert, cert_len as usize, NodeAuthResult::InvalidInput);

    let cert_slice = std::slice::from_raw_parts(cert, cert_len as usize);

    #[cfg(feature = "light-client-validation")]
    if !check_cert_in_current_block(cert_slice) {
        return NodeAuthResult::SignatureInvalid;
    }

    let mut target_public_key: [u8; 32] = [0u8; 32];

    let attestation = AttestationCombined::from_blob(cert, cert_len as usize);

    if attestation.quote.is_empty() || attestation.coll.is_empty() {
        warn!("No valid attestation method provided");
        return NodeAuthResult::InvalidCert;
    }

    let res = verify_attestation_dcap(&attestation, &mut target_public_key);
    if NodeAuthResult::Success != res {
        return res;
    }

    let result = panic::catch_unwind(|| -> Result<Vec<u8>, NodeAuthResult> {
        trace!(
            "ecall_get_encrypted_seed target_public_key key pk: {:?}",
            &target_public_key.to_vec()
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
                trace!("Done encrypting seed, got {:?}, {:?}", res.len(), res);

                let actual_size = res.len() as u32;
                *p_seeds_size = actual_size;

                if n_seeds < actual_size {
                    warn!("insufficient seeds buffer!");
                    return NodeAuthResult::InvalidInput;
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
