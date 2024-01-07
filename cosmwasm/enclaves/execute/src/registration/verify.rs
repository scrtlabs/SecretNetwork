///
/// These functions run on-chain and must be deterministic across all nodes
///
use log::*;
use secret_attestation_token::{AttestationType, SecretAttestationToken, VerificationError};
use std::panic;

use enclave_ffi_types::NodeAuthResult;

use crate::registration::seed_exchange::SeedType;
use enclave_crypto::consts::OUTPUT_ENCRYPTED_SEED_SIZE;
use enclave_crypto::PUBLIC_KEY_SIZE;
use enclave_utils::{
    oom_handler::{self, get_then_clear_oom_happened},
    validate_const_ptr, validate_mut_ptr,
};

// use super::cert::verify_ra_cert;
use super::seed_exchange::encrypt_seed;

#[cfg(feature = "light-client-validation")]
use enclave_contract_engine::check_cert_in_current_block;

///
/// `ecall_legacy_verify_node_on_chain`
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
pub unsafe extern "C" fn ecall_legacy_verify_node_on_chain(
    auth_material: *const u8,
    auth_material_len: u32,
    seed: &mut [u8; OUTPUT_ENCRYPTED_SEED_SIZE as usize],
) -> NodeAuthResult {
    if let Err(_err) = oom_handler::register_oom_handler() {
        error!("Could not register OOM handler!");
        return NodeAuthResult::MemorySafetyAllocationError;
    }

    validate_mut_ptr!(seed.as_mut_ptr(), seed.len(), NodeAuthResult::InvalidInput);
    validate_const_ptr!(cert, cert_len as usize, NodeAuthResult::InvalidInput);

    let cert_slice = std::slice::from_raw_parts(cert, cert_len as usize);
    validate_const_ptr!(
        auth_material,
        auth_material_len as usize,
        NodeAuthResult::InvalidInput
    );
    let material: SecretAttestationToken = serde_json::from_slice(std::slice::from_raw_parts(
        auth_material,
        auth_material_len as usize,
    ))
        .unwrap();

    #[cfg(feature = "light-client-validation")]
    if !check_cert_in_current_block(cert_slice) {
        return NodeAuthResult::SignatureInvalid;
    }

    let result = panic::catch_unwind(|| -> Result<Vec<u8>, NodeAuthResult> {
        // verify certificate, and return the public key in the extra data of the report

        let pk = match material.attestation_type {
            #[cfg(feature = "SGX_MODE_HW")]
            AttestationType::SgxEpid => epid::verify_authentication_material(&material),
            #[cfg(feature = "dcap")]
            AttestationType::SgxDcap => dcap::verify_authentication_material(&material),
            #[cfg(not(feature = "SGX_MODE_HW"))]
            AttestationType::SgxSw => epid::verify_authentication_material(&material),
            _ => {
                error!("Unsupported authentication type");
                Err(VerificationError::ErrorGeneric)
            }
        }
            .map_err(|_e| {
                error!("Error verifying tx"); //todo: add more verifying error types and print
                NodeAuthResult::InvalidInput
            })?;

        // just make sure the length isn't wrong for some reason (certificate may be malformed)
        if pk.len() != PUBLIC_KEY_SIZE {
            warn!(
                "Got public key from certificate with the wrong size: {:?}",
                pk.len()
            );
            return Err(NodeAuthResult::MalformedPublicKey);
        }

        let mut target_public_key: [u8; 32] = [0u8; 32];
        target_public_key.copy_from_slice(&pk);
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
