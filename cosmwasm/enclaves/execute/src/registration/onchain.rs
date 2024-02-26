///
/// These functions run on-chain and must be deterministic across all nodes
///
use log::*;
use std::panic;

use enclave_ffi_types::NodeAuthResult;

use crate::registration::seed_exchange::SeedType;
use crate::registration::attestation::verify_quote_ecdsa;
use crate::registration::cert::verify_ra_report;

use enclave_crypto::consts::OUTPUT_ENCRYPTED_SEED_SIZE;
use enclave_crypto::PUBLIC_KEY_SIZE;
use enclave_utils::{
    oom_handler::{self, get_then_clear_oom_happened},
    validate_const_ptr, validate_mut_ptr,
};

use sgx_types::sgx_ql_qv_result_t;

#[cfg(feature = "SGX_MODE_HW")]
use enclave_crypto::consts::SigningMethod;

use super::cert::verify_ra_cert;
use super::seed_exchange::encrypt_seed;
use core::mem;
use std::slice;

#[cfg(feature = "light-client-validation")]
use enclave_contract_engine::check_cert_in_current_block;

#[cfg(feature = "light-client-validation")]
use block_verifier::VERIFIED_BLOCK_MESSAGES;

#[cfg(feature = "light-client-validation")]
fn get_current_block_time_s() -> i64
{
    let verified_msgs = VERIFIED_BLOCK_MESSAGES.lock().unwrap();
    let tm_ns = verified_msgs.time();
    return (tm_ns / 1000000000) as i64;
}

#[cfg(not(feature = "light-client-validation"))]
fn get_current_block_time_s() -> i64
{
    return 0 as i64;
}


pub fn split_combined_cert(
    cert: *const u8,
    cert_len: u32,
) -> (Vec<u8>, Vec<u8>, Vec<u8>)
{
    let mut vec_cert : Vec<u8> = Vec::new();
    let mut vec_quote : Vec<u8> = Vec::new();
    let mut vec_coll : Vec<u8> = Vec::new();

    let n0 = mem::size_of::<u32>() as u32 * 3;

    if cert_len >= n0
    {
        let p_cert = cert as *const u32;
        let s0 = u32::from_le( unsafe { *p_cert } );
        let s1 = u32::from_le( unsafe { *(p_cert.offset(1)) } );
        let s2 = u32::from_le( unsafe { *(p_cert.offset(2)) } );

        let size_total =
            (n0 as u64) +
            (s0 as u64) +
            (s1 as u64) +
            (s2 as u64);

        if size_total <= cert_len as u64
        {
            vec_cert = unsafe { slice::from_raw_parts(cert.offset(n0 as isize), s0 as usize).to_vec() };
            vec_quote = unsafe { slice::from_raw_parts(cert.offset((n0 + s0) as isize), s1 as usize).to_vec() };
            vec_coll = unsafe { slice::from_raw_parts(cert.offset((n0 + s0 + s1) as isize), s2 as usize).to_vec() };
        }

    }

    (vec_cert, vec_quote, vec_coll)
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
    // seed structure 1 byte - length (96 or 48) | genesis seed bytes | current seed bytes (optional)
    seed: &mut [u8; OUTPUT_ENCRYPTED_SEED_SIZE as usize],
) -> NodeAuthResult {
    if let Err(_err) = oom_handler::register_oom_handler() {
        error!("Could not register OOM handler!");
        return NodeAuthResult::MemorySafetyAllocationError;
    }

    validate_mut_ptr!(seed.as_mut_ptr(), seed.len(), NodeAuthResult::InvalidInput);
    validate_const_ptr!(cert, cert_len as usize, NodeAuthResult::InvalidInput);

    let cert_slice = std::slice::from_raw_parts(cert, cert_len as usize);

    #[cfg(feature = "light-client-validation")]
    if !check_cert_in_current_block(cert_slice) {
        return NodeAuthResult::SignatureInvalid;
    }

    let mut target_public_key: [u8; 32] = [0u8; 32];

    let (vec_cert, vec_quote, vec_coll) = split_combined_cert(cert, cert_len);

    if vec_quote.is_empty() || vec_coll.is_empty() {

        if vec_cert.is_empty() {
            warn!("No valid attestation method provided");
            return NodeAuthResult::InvalidCert;
        }

        trace!("EPID attestation");

        //let result = panic::catch_unwind(|| {
        // verify certificate, and return the public key in the extra data of the report
        let pk = match verify_ra_cert(cert_slice, None, true) {
            Ok(retval) => {
                retval
            }
            Err(e) => {
                return e;
            }
        };

        // just make sure the length isn't wrong for some reason (certificate may be malformed)
        if pk.len() != PUBLIC_KEY_SIZE {
            warn!(
                    "Got public key from certificate with the wrong size: {:?}",
                    pk.len()
                );
            return NodeAuthResult::MalformedPublicKey;
        }

        target_public_key.copy_from_slice(&pk);
        //    NodeAuthResult::Success // not yet actually
        //});

        //if result.is_err() {
        //    // There's no real need here to test if oom happened
        //    get_then_clear_oom_happened();
        //    warn!("Enclave call ecall_authenticate_new_node panic!");
        //    return NodeAuthResult::Panic;
        //}

    } else {

        // DCAP
        trace!("DCAP attestation");

        let tm_s = get_current_block_time_s();
        trace!("Current block time: {}", tm_s);

        // test self
        let report_body = match verify_quote_ecdsa(&vec_quote, &vec_coll, tm_s) {
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

        let res2 = verify_ra_report(report_body.mr_signer.m, report_body.mr_enclave.m, Some(SigningMethod::MRSIGNER));
        if NodeAuthResult::Success != res2 {
            return res2;
        }

        target_public_key.copy_from_slice(&report_body.report_data.d[..32]);
    }

    let result = panic::catch_unwind(|| -> Result<Vec<u8>, NodeAuthResult> {

        trace!(
            "ecall_get_encrypted_seed target_public_key key pk: {:?}",
            &target_public_key.to_vec()
        );

        let mut res: Vec<u8> = encrypt_seed(target_public_key, SeedType::Genesis, false)
            .map_err(|_| NodeAuthResult::SeedEncryptionFailed)?;

        let res_current: Vec<u8> = encrypt_seed(target_public_key, SeedType::Current, false)
            .map_err(|_| NodeAuthResult::SeedEncryptionFailed)?;

        res.extend(&res_current);

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

                seed.copy_from_slice(&res);
                trace!("returning with seed: {:?}, {:?}", seed.len(), seed);
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
