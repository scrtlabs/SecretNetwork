//!
/// These functions run off chain, and so are not limited by deterministic limitations. Feel free
/// to go crazy with random generation entropy, time requirements, or whatever else
///
use log::*;
#[cfg(feature = "SGX_MODE_HW")]
use sgx_types::{sgx_platform_info_t, sgx_update_info_bit_t};
use sgx_types::{sgx_quote_sign_type_t, sgx_status_t, SgxResult};
use std::slice;

#[cfg(feature = "SGX_MODE_HW")]
use enclave_ffi_types::NodeAuthResult;

use enclave_crypto::consts::{
    SigningMethod, ATTESTATION_CERT_PATH, ENCRYPTED_SEED_SIZE, IO_CERTIFICATE_SAVE_PATH,
    SEED_EXCH_CERTIFICATE_SAVE_PATH,
};
use enclave_crypto::{KeyPair, Keychain, KEY_MANAGER, PUBLIC_KEY_SIZE};
use enclave_utils::pointers::validate_mut_slice;
use enclave_utils::storage::write_to_untrusted;
use enclave_utils::{validate_const_ptr, validate_mut_ptr};

#[cfg(feature = "SGX_MODE_HW")]
use crate::registration::report::AttestationReport;

use super::attestation::create_attestation_certificate;
use super::cert::verify_ra_cert;
#[cfg(feature = "SGX_MODE_HW")]
use super::cert::{ocall_get_update_info, verify_quote_status};
use super::seed_exchange::decrypt_seed;

///
/// `ecall_init_bootstrap`
///
/// Function to handle the initialization of the bootstrap node. Generates the master private/public
/// key (seed + pk_io/sk_io). This happens once at the initialization of a chain. Returns the master
/// public key (pk_io), which is saved on-chain, and used to propagate the seed to registering nodes
///
/// # Safety
///  Something should go here
///
#[no_mangle]
pub unsafe extern "C" fn ecall_init_bootstrap(
    public_key: &mut [u8; PUBLIC_KEY_SIZE],
    spid: *const u8,
    spid_len: u32,
    api_key: *const u8,
    api_key_len: u32,
) -> sgx_status_t {
    validate_mut_ptr!(
        public_key.as_mut_ptr(),
        public_key.len(),
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );

    validate_const_ptr!(spid, spid_len as usize, sgx_status_t::SGX_ERROR_UNEXPECTED);
    let spid_slice = slice::from_raw_parts(spid, spid_len as usize);

    validate_const_ptr!(
        api_key,
        api_key_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );
    let api_key_slice = slice::from_raw_parts(api_key, api_key_len as usize);

    let mut key_manager = Keychain::new();

    if let Err(_e) = key_manager.create_consensus_seed() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(_e) = key_manager.generate_consensus_master_keys() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(_e) = key_manager.create_registration_key() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let kp = key_manager.seed_exchange_key().unwrap();
    if let Err(status) = attest_from_key(
        &kp,
        SEED_EXCH_CERTIFICATE_SAVE_PATH,
        spid_slice,
        api_key_slice,
    ) {
        return status;
    }

    let kp = key_manager.get_consensus_io_exchange_keypair().unwrap();
    if let Err(status) = attest_from_key(&kp, IO_CERTIFICATE_SAVE_PATH, spid_slice, api_key_slice) {
        return status;
    }

    public_key.copy_from_slice(&key_manager.seed_exchange_key().unwrap().get_pubkey());
    trace!(
        "ecall_init_bootstrap consensus_seed_exchange_keypair public key: {:?}",
        hex::encode(public_key)
    );

    sgx_status_t::SGX_SUCCESS
}

///
///  `ecall_init_node`
///
/// This function is called during initialization of __non__ bootstrap nodes.
///
/// It receives the master public key (pk_io) and uses it, and its node key (generated in [ecall_key_gen])
/// to decrypt the seed.
///
/// The seed was encrypted using Diffie-Hellman in the function [ecall_get_encrypted_seed]
///
/// This function happens off-chain, so if we panic for some reason it _can_ be acceptable,
///  though probably not recommended
///
/// # Safety
///  Something should go here
///
#[no_mangle]
pub unsafe extern "C" fn ecall_init_node(
    master_cert: *const u8,
    master_cert_len: u32,
    encrypted_seed: *const u8,
    encrypted_seed_len: u32,
) -> sgx_status_t {
    validate_const_ptr!(
        master_cert,
        master_cert_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );

    validate_const_ptr!(
        encrypted_seed,
        encrypted_seed_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );

    let cert_slice = slice::from_raw_parts(master_cert, master_cert_len as usize);

    if (encrypted_seed_len as usize) != ENCRYPTED_SEED_SIZE {
        warn!(
            "Got encrypted seed with the wrong size: {:?}",
            encrypted_seed_len
        );
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let encrypted_seed_slice = slice::from_raw_parts(encrypted_seed, encrypted_seed_len as usize);

    let mut encrypted_seed = [0u8; ENCRYPTED_SEED_SIZE];
    encrypted_seed.copy_from_slice(&encrypted_seed_slice);

    // public keys in certificates don't have 0x04, so we'll copy it here
    let mut target_public_key: [u8; PUBLIC_KEY_SIZE] = [0u8; PUBLIC_KEY_SIZE];

    // validate certificate w/ attestation report
    // testing only
    let pk = match verify_ra_cert(cert_slice, Some(SigningMethod::MRSIGNER)) {
        Err(e) => {
            error!("Error in validating certificate: {:?}", e);
            return sgx_status_t::SGX_ERROR_UNEXPECTED;
        }
        Ok(res) => res,
    };

    // just make sure the of the public key isn't messed up
    if pk.len() != PUBLIC_KEY_SIZE {
        error!(
            "Got public key from certificate with the wrong size: {:?}",
            pk.len()
        );
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }
    target_public_key.copy_from_slice(&pk);

    let mut key_manager = Keychain::new();
    let seed = match decrypt_seed(&key_manager, target_public_key, encrypted_seed) {
        Ok(result) => result,
        Err(status) => return status,
    };

    if let Err(_e) = key_manager.set_consensus_seed(seed) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    if let Err(_e) = key_manager.generate_consensus_master_keys() {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    sgx_status_t::SGX_SUCCESS
}

#[no_mangle]
/**
 * `ecall_get_attestation_report`
 *
 * Creates the attestation report to be used to authenticate with the blockchain. The output of this
 * function is an X.509 certificate signed by the enclave, which contains the report signed by Intel.
 *
 * Verifying functions will verify the public key bytes sent in the extra data of the __report__ (which
 * may or may not match the public key of the __certificate__ -- depending on implementation choices)
 *
 * This x509 certificate can be used in the future for mutual-RA cross-enclave TLS channels, or for
 * other creative usages.
 * # Safety
 * Something should go here
 */
pub unsafe extern "C" fn ecall_get_attestation_report(
    spid: *const u8,
    spid_len: u32,
    api_key: *const u8,
    api_key_len: u32,
) -> sgx_status_t {
    validate_const_ptr!(spid, spid_len as usize, sgx_status_t::SGX_ERROR_UNEXPECTED);
    let spid_slice = slice::from_raw_parts(spid, spid_len as usize);

    validate_const_ptr!(
        api_key,
        api_key_len as usize,
        sgx_status_t::SGX_ERROR_UNEXPECTED,
    );
    let api_key_slice = slice::from_raw_parts(api_key, api_key_len as usize);

    let kp = KEY_MANAGER.get_registration_key().unwrap();
    trace!(
        "ecall_get_attestation_report key pk: {:?}",
        &kp.get_pubkey().to_vec()
    );
    let (_private_key_der, cert) = match create_attestation_certificate(
        &kp,
        sgx_quote_sign_type_t::SGX_UNLINKABLE_SIGNATURE,
        spid_slice,
        api_key_slice,
    ) {
        Err(e) => {
            warn!("Error in create_attestation_certificate: {:?}", e);
            return e;
        }
        Ok(res) => res,
    };

    //let path_prefix = ATTESTATION_CERT_PATH.to_owned();
    if let Err(status) = write_to_untrusted(cert.as_slice(), &ATTESTATION_CERT_PATH) {
        return status;
    }

    print_local_report_info(cert.as_slice());

    sgx_status_t::SGX_SUCCESS
}

///
/// This function generates the registration_key, which is used in the attestation and registration
/// process
///
#[no_mangle]
pub unsafe extern "C" fn ecall_key_gen(
    public_key: &mut [u8; PUBLIC_KEY_SIZE],
) -> sgx_types::sgx_status_t {
    if let Err(_e) = validate_mut_slice(public_key) {
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    }

    let mut key_manager = Keychain::new();
    if let Err(_e) = key_manager.create_registration_key() {
        error!("Failed to create registration key");
        return sgx_status_t::SGX_ERROR_UNEXPECTED;
    };

    let pubkey = key_manager.get_registration_key().unwrap().get_pubkey();
    public_key.clone_from_slice(&pubkey);
    trace!("ecall_key_gen key pk: {:?}", public_key.to_vec());
    sgx_status_t::SGX_SUCCESS
}

pub fn attest_from_key(
    kp: &KeyPair,
    save_path: &str,
    spid: &[u8],
    api_key: &[u8],
) -> SgxResult<()> {
    let (_, cert) = match create_attestation_certificate(
        &kp,
        sgx_quote_sign_type_t::SGX_UNLINKABLE_SIGNATURE,
        spid,
        api_key,
    ) {
        Err(e) => {
            error!("Error in create_attestation_certificate: {:?}", e);
            return Err(e);
        }
        Ok(res) => res,
    };

    if let Err(status) = write_to_untrusted(cert.as_slice(), save_path) {
        return Err(status);
    }

    Ok(())
}

#[cfg(not(feature = "SGX_MODE_HW"))]
fn print_local_report_info(_cert: &[u8]) {}

#[cfg(feature = "SGX_MODE_HW")]
fn print_local_report_info(cert: &[u8]) {
    let report = match AttestationReport::from_cert(cert) {
        Ok(r) => r,
        Err(_) => {
            error!("Error parsing report");
            return;
        }
    };

    let node_auth_result = NodeAuthResult::from(&report.sgx_quote_status);
    // print
    match verify_quote_status(&report.sgx_quote_status, &report.advisroy_ids) {
        Err(status) => match status {
            NodeAuthResult::SwHardeningAndConfigurationNeeded => {
                println!("Platform status is SW_HARDENING_AND_CONFIGURATION_NEEDED. This means is updated but requires further BIOS configuration");
            }
            NodeAuthResult::GroupOutOfDate => {
                println!("Platform status is GROUP_OUT_OF_DATE. This means that one of the system components is missing a security update");
            }
            _ => {
                println!("Platform status is {:?}", status);
            }
        },
        _ => println!("Platform Okay!"),
    }

    // print platform blob info
    match node_auth_result {
        NodeAuthResult::GroupOutOfDate | NodeAuthResult::SwHardeningAndConfigurationNeeded => unsafe {
            print_platform_info(&report)
        },
        _ => {}
    }
}

#[cfg(feature = "SGX_MODE_HW")]
unsafe fn print_platform_info(report: &AttestationReport) {
    if let Some(platform_info) = &report.platform_info_blob {
        let mut update_info = sgx_update_info_bit_t::default();
        let mut rt = sgx_status_t::default();
        let res = ocall_get_update_info(
            &mut rt as *mut sgx_status_t,
            platform_info[4..].as_ptr() as *const sgx_platform_info_t,
            1,
            &mut update_info,
        );

        if res != sgx_status_t::SGX_SUCCESS {
            println!("res={:?}", res);
            return;
        }

        if rt != sgx_status_t::SGX_SUCCESS {
            if update_info.ucodeUpdate != 0 {
                println!("Processor Firmware Update (ucodeUpdate). A security upgrade for your computing\n\
                            device is required for this application to continue to provide you with a high degree of\n\
                            security. Please contact your device manufacturer’s support website for a BIOS update\n\
                            for this system");
            }

            if update_info.csmeFwUpdate != 0 {
                println!("Intel Manageability Engine Update (csmeFwUpdate). A security upgrade for your\n\
                            computing device is required for this application to continue to provide you with a high\n\
                            degree of security. Please contact your device manufacturer’s support website for a\n\
                            BIOS and/or Intel® Manageability Engine update for this system");
            }

            if update_info.pswUpdate != 0 {
                println!("Intel SGX Platform Software Update (pswUpdate). A security upgrade for your\n\
                              computing device is required for this application to continue to provide you with a high\n\
                              degree of security. Please visit this application’s support website for an Intel SGX\n\
                              Platform SW update");
            }
        }
    }
}
