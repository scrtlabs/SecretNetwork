use log::*;

use sgx_types::sgx_status_t;

use enclave_ffi_types::{Ctx, EnclaveBuffer, OcallReturn, UntrustedVmError};

use enclave_crypto::{sha_256, AESKey, Kdf, SIVEncryptable, KEY_MANAGER};

use crate::external::{ecalls, ocalls};

use super::contract_validation::ContractKey;
use super::errors::WasmEngineError;

#[cfg(not(feature = "query-only"))]
pub fn write_encrypted_key(
    key: &[u8],
    value: &[u8],
    context: &Ctx,
    contract_key: &ContractKey,
) -> Result<u64, WasmEngineError> {
    // Get the state key from the key manager

    let scrambled_field_name = field_name_digest(key, contract_key);

    info!(
        "Writing to scrambled field name: {:?}",
        scrambled_field_name
    );

    let (ad, ad_used_gas) = derive_ad_for_field(&scrambled_field_name, &context)?;

    let encrypted_value = encrypt_key(&scrambled_field_name, value, contract_key, &ad)?;

    let mut db_data: Vec<u8> = ad.to_vec();
    db_data.extend_from_slice(encrypted_value.as_slice());

    // Write the new data as concat(ad, encrypted_val)
    let write_used_gas = write_db(context, &scrambled_field_name, &db_data).map_err(|err| {
        warn!(
            "write_db() go an error from ocall_write_db, stopping wasm: {:?}",
            err
        );
        err
    })?;

    Ok(ad_used_gas + write_used_gas)
}

pub fn read_encrypted_key(
    key: &[u8],
    context: &Ctx,
    contract_key: &ContractKey,
) -> Result<(Option<Vec<u8>>, u64), WasmEngineError> {
    let scrambled_field_name = field_name_digest(key, contract_key);

    info!(
        "Reading from scrambled field name: {:?}",
        scrambled_field_name
    );

    // Call read_db (this bubbles up to Tendermint via ocalls and FFI to Go code)
    // This returns the value from Tendermint
    match read_db(context, &scrambled_field_name) {
        Ok((value, gas_used)) => match value {
            Some(value) => match decrypt_key(&scrambled_field_name, &value, contract_key) {
                Ok(decrypted) => Ok((Some(decrypted), gas_used)),
                // This error case is why we have all the matches here.
                // If we successfully collected a value, but failed to decrypt it, then we propagate that error.
                Err(err) => Err(err),
            },
            None => Ok((None, gas_used)),
        },
        Err(err) => Err(err),
    }
}

#[cfg(not(feature = "query-only"))]
pub fn remove_encrypted_key(
    key: &[u8],
    context: &Ctx,
    contract_key: &ContractKey,
) -> Result<u64, WasmEngineError> {
    let scrambled_field_name = field_name_digest(key, contract_key);

    info!("Removing scrambled field name: {:?}", scrambled_field_name);

    // Call remove_db (this bubbles up to Tendermint via ocalls and FFI to Go code)
    // fn remove_db(context: Ctx, key: &[u8]) {
    let gas_used = remove_db(context, &scrambled_field_name).map_err(|err| {
        warn!(
            "remove_db() got an error from ocall_remove_db, stopping wasm: {:?}",
            err
        );
        err
    })?;
    Ok(gas_used)
}

pub fn field_name_digest(field_name: &[u8], contract_key: &ContractKey) -> [u8; 32] {
    let mut data = field_name.to_vec();
    data.extend_from_slice(contract_key);

    sha_256(&data)
}

/// Safe wrapper around reads from the contract storage
fn read_db(context: &Ctx, key: &[u8]) -> Result<(Option<Vec<u8>>, u64), WasmEngineError> {
    let mut ocall_return = OcallReturn::Success;
    let mut enclave_buffer = std::mem::MaybeUninit::<EnclaveBuffer>::uninit();
    let mut vm_err = UntrustedVmError::default();
    let mut gas_used = 0_u64;
    let value = unsafe {
        let status = ocalls::ocall_read_db(
            (&mut ocall_return) as *mut _,
            context.unsafe_clone(),
            (&mut vm_err) as *mut _,
            (&mut gas_used) as *mut _,
            enclave_buffer.as_mut_ptr(),
            key.as_ptr(),
            key.len(),
        );
        match status {
            sgx_status_t::SGX_SUCCESS => { /* continue */ }
            error_status => {
                warn!(
                    "read_db() got an error from ocall_read_db, stopping wasm: {:?}",
                    error_status
                );
                return Err(WasmEngineError::FailedOcall(vm_err));
            }
        }

        match ocall_return {
            OcallReturn::Success => {
                let enclave_buffer = enclave_buffer.assume_init();
                ecalls::recover_buffer(enclave_buffer)?
            }
            OcallReturn::Failure => {
                return Err(WasmEngineError::FailedOcall(vm_err));
            }
            OcallReturn::Panic => return Err(WasmEngineError::Panic),
        }
    };

    Ok((value, gas_used))
}

/// Safe wrapper around reads from the contract storage
#[cfg(not(feature = "query-only"))]
fn remove_db(context: &Ctx, key: &[u8]) -> Result<u64, WasmEngineError> {
    let mut ocall_return = OcallReturn::Success;
    let mut vm_err = UntrustedVmError::default();
    let mut gas_used = 0_u64;
    match unsafe {
        ocalls::ocall_remove_db(
            (&mut ocall_return) as *mut _,
            context.unsafe_clone(),
            (&mut vm_err) as *mut _,
            (&mut gas_used) as *mut _,
            key.as_ptr(),
            key.len(),
        )
    } {
        sgx_status_t::SGX_SUCCESS => { /* continue */ }
        _error_status => return Err(WasmEngineError::FailedOcall(vm_err)),
    }

    match ocall_return {
        OcallReturn::Success => Ok(gas_used),
        OcallReturn::Failure => Err(WasmEngineError::FailedOcall(vm_err)),
        OcallReturn::Panic => Err(WasmEngineError::Panic),
    }
}

/// Safe wrapper around writes to the contract storage
#[cfg(not(feature = "query-only"))]
fn write_db(context: &Ctx, key: &[u8], value: &[u8]) -> Result<u64, WasmEngineError> {
    let mut ocall_return = OcallReturn::Success;
    let mut vm_err = UntrustedVmError::default();
    let mut gas_used = 0_u64;
    match unsafe {
        ocalls::ocall_write_db(
            (&mut ocall_return) as *mut _,
            context.unsafe_clone(),
            (&mut vm_err) as *mut _,
            (&mut gas_used) as *mut _,
            key.as_ptr(),
            key.len(),
            value.as_ptr(),
            value.len(),
        )
    } {
        sgx_status_t::SGX_SUCCESS => { /* continue */ }
        _err_status => return Err(WasmEngineError::FailedOcall(vm_err)),
    }

    match ocall_return {
        OcallReturn::Success => Ok(gas_used),
        OcallReturn::Failure => Err(WasmEngineError::FailedOcall(vm_err)),
        OcallReturn::Panic => Err(WasmEngineError::Panic),
    }
}

#[cfg(not(feature = "query-only"))]
fn derive_ad_for_field(
    field_name: &[u8],
    context: &Ctx,
) -> Result<([u8; 32], u64), WasmEngineError> {
    let (old_value, gas_used) = read_db(context, field_name)?;
    let ad = sha_256(
        old_value
            .as_ref()
            // Extract previous_ad to calculate the new ad (first 32 bytes)
            .map(|old_value| old_value.split_at(32).0)
            // No data exist yet for this state_key_name, so creating a new `ad`
            .unwrap_or(field_name),
    );
    Ok((ad, gas_used))
}

#[cfg(not(feature = "query-only"))]
fn encrypt_key(
    field_name: &[u8],
    value: &[u8],
    contract_key: &ContractKey,
    ad: &[u8],
) -> Result<Vec<u8>, WasmEngineError> {
    let encryption_key = get_symmetrical_key(field_name, contract_key);

    encryption_key
        .encrypt_siv(&value, Some(&[ad]))
        .map_err(|err| {
            warn!(
                "write_db() got an error while trying to encrypt the value {:?}, stopping wasm: {:?}",
                String::from_utf8_lossy(&value),
                err
            );
            WasmEngineError::EncryptionError
    })
}

fn decrypt_key(
    field_name: &[u8],
    value: &[u8],
    contract_key: &ContractKey,
) -> Result<Vec<u8>, WasmEngineError> {
    let decryption_key = get_symmetrical_key(field_name, contract_key);

    // Slice ad from `value`
    let (ad, encrypted_value) = value.split_at(32);

    decryption_key.decrypt_siv(&encrypted_value, Some(&[ad])).map_err(|err| {
        warn!(
            "read_db() got an error while trying to decrypt the value for key {:?}, stopping wasm: {:?}",
            String::from_utf8_lossy(&field_name),
            err
        );
        WasmEngineError::DecryptionError
    })
}

fn get_symmetrical_key(field_name: &[u8], contract_key: &ContractKey) -> AESKey {
    let consensus_state_ikm = KEY_MANAGER.get_consensus_state_ikm().unwrap();

    // Derive the key to the specific field name
    let mut derivation_data = field_name.to_vec();
    derivation_data.extend_from_slice(contract_key.to_vec().as_slice());
    consensus_state_ikm.derive_key_from_this(&derivation_data)
}
