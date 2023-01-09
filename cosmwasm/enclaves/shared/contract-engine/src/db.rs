use enclave_crypto::consts::{CONSENSUS_SEED_VERSION, ENCRYPTED_KEY_MAGIC_BYTES};
use enclave_crypto::key_manager::SeedsHolder;
use log::*;

use sgx_types::sgx_status_t;

use enclave_ffi_types::{Ctx, EnclaveBuffer, OcallReturn, UntrustedVmError};

use enclave_crypto::{sha_256, AESKey, Kdf, SIVEncryptable, KEY_MANAGER};

use crate::external::{ecalls, ocalls};

use enclave_utils::kv_cache::KvCache;

use super::contract_validation::ContractKey;
use super::errors::WasmEngineError;

//#[cfg(not(feature = "query-only"))]
pub fn write_multiple_keys(
    context: &Ctx,
    keys: Vec<(Vec<u8>, Vec<u8>)>,
) -> Result<u64, WasmEngineError> {
    let mut ocall_return = OcallReturn::Success;

    if keys.is_empty() {
        return Ok(0);
    }

    let x = serde_json::to_vec(&keys).unwrap();
    let len = x.len();
    let ptr = x.as_ptr();

    let mut vm_err = UntrustedVmError::default();
    let mut gas_used = 0_u64;
    match unsafe {
        ocalls::ocall_multiple_write_db(
            (&mut ocall_return) as *mut _,
            context.unsafe_clone(),
            (&mut vm_err) as *mut _,
            (&mut gas_used) as *mut _,
            ptr,
            len,
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

// #[cfg(not(feature = "query-only"))]
#[allow(dead_code)]
pub fn write_to_encrypted_state(
    plaintext_key: &[u8],
    plaintext_value: &[u8],
    context: &Ctx,
    contract_key: &ContractKey,
) -> Result<u64, WasmEngineError> {
    // Get the state key from the key manager

    let (scrambled_field_name, used_gas_for_key_creation, db_data) =
        create_encrypted_key(plaintext_key, plaintext_value, context, contract_key)?;

    // Write the new data as concat(ad, encrypted_val)
    let used_gas_for_write = write_db(context, &scrambled_field_name, &db_data).map_err(|err| {
        warn!(
            "write_db() go an error from ocall_write_db, stopping wasm: {:?}",
            err
        );
        err
    })?;

    Ok(used_gas_for_key_creation + used_gas_for_write)
}

// #[cfg(not(feature = "query-only"))]
pub fn create_encrypted_key(
    plaintext_key: &[u8],
    plaintext_value: &[u8],
    context: &Ctx,
    contract_key: &ContractKey,
) -> Result<(Vec<u8>, u64, Vec<u8>), WasmEngineError> {
    let scrambled_field_name = field_name_digest(plaintext_key, contract_key);
    let gas_used_remove = remove_db(context, &scrambled_field_name).map_err(|err| {
        warn!(
            "write_db() got an error from ocall_remove_db, stopping wasm: {:?}",
            err
        );
        err
    })?;

    let encrypted_key = encrypt_key_new(plaintext_key, contract_key)?;
    let encrypted_value = encrypt_value_new(&encrypted_key, plaintext_value, contract_key)?;

    let mut encrypted_key_with_header: Vec<u8> = vec![];
    encrypted_key_with_header.extend_from_slice(ENCRYPTED_KEY_MAGIC_BYTES);
    encrypted_key_with_header.extend_from_slice(&CONSENSUS_SEED_VERSION.to_be_bytes());
    encrypted_key_with_header.extend_from_slice(&encrypted_key);

    debug!(
        "Removed scrambled field name: {:?} and created new key with magic: {:?}",
        scrambled_field_name, encrypted_key_with_header
    );

    Ok((encrypted_key_with_header, gas_used_remove, encrypted_value))
}

pub fn read_from_encrypted_state(
    plaintext_key: &[u8],
    context: &Ctx,
    contract_key: &ContractKey,
    has_write_permissions: bool,
    kv_cache: &mut KvCache,
) -> Result<(Option<Vec<u8>>, u64), WasmEngineError> {
    // Try reading with the new encryption format
    let encrypted_key = encrypt_key_new(plaintext_key, contract_key)?;

    let mut encrypted_key_with_header: Vec<u8> = vec![];
    encrypted_key_with_header.extend_from_slice(ENCRYPTED_KEY_MAGIC_BYTES);
    encrypted_key_with_header.extend_from_slice(&CONSENSUS_SEED_VERSION.to_be_bytes());
    encrypted_key_with_header.extend_from_slice(&encrypted_key);

    let mut maybe_plaintext_value: Option<Vec<u8>>;
    let gas_used_first_read: u64;
    (maybe_plaintext_value, gas_used_first_read) =
        match read_db(context, &encrypted_key_with_header) {
            Ok((maybe_encrypted_value, gas_used)) => match maybe_encrypted_value {
                Some(encrypted_value) => {
                    match decrypt_value_new(&encrypted_key, &encrypted_value, contract_key) {
                        Ok(plaintext_value) => Ok((Some(plaintext_value), gas_used)),
                        // This error case is why we have all the matches here.
                        // If we successfully collected a value, but failed to decrypt it, then we propagate that error.
                        Err(err) => Err(err),
                    }
                }
                None => Ok((None, gas_used)),
            },
            Err(err) => Err(err),
        }?;

    if let Some(plaintext_value) = maybe_plaintext_value {
        return Ok((Some(plaintext_value), gas_used_first_read));
    }

    // Key doesn't exist, try reading with the old encryption format
    let scrambled_field_name = field_name_digest(plaintext_key, contract_key);

    trace!(
        "Reading from scrambled field name: {:?}",
        scrambled_field_name
    );

    let gas_used_second_read: u64;
    (maybe_plaintext_value, gas_used_second_read) = match read_db(context, &scrambled_field_name) {
        Ok((encrypted_value, gas_used)) => match encrypted_value {
            Some(plaintext_value) => {
                match decrypt_value_old(&scrambled_field_name, &plaintext_value, contract_key) {
                    Ok(plaintext_value) => {
                        let _ = kv_cache.write(plaintext_key, &plaintext_value);
                        Ok((Some(plaintext_value), gas_used))
                    }
                    // This error case is why we have all the matches here.
                    // If we successfully collected a value, but failed to decrypt it, then we propagate that error.
                    Err(err) => Err(err),
                }
            }
            None => Ok((None, gas_used)),
        },
        Err(err) => Err(err),
    }?;

    let mut gas_used_write: u64 = 0;
    if has_write_permissions {
        if let Some(ref plaintext_value) = maybe_plaintext_value {
            // Key exists with the old format, rewriting with the new format
            gas_used_write =
                write_to_encrypted_state(plaintext_key, plaintext_value, context, contract_key)?;
        }
    }

    Ok((
        maybe_plaintext_value,
        gas_used_first_read + gas_used_second_read + gas_used_write,
    ))
}

pub fn remove_from_encrypted_state(
    plaintext_key: &[u8],
    context: &Ctx,
    contract_key: &ContractKey,
) -> Result<u64, WasmEngineError> {
    // TODO in the future we can check if all the state keys are new
    // then skip removing the old key step

    // Remove key with old format
    let scrambled_field_name = field_name_digest(plaintext_key, contract_key);

    trace!("Removing scrambled field name: {:?}", scrambled_field_name);

    let gas_used_first_remove = remove_db(context, &scrambled_field_name).map_err(|err| {
        warn!(
            "remove_db() got an error from ocall_remove_db on old key remove, stopping wasm: {:?}",
            err
        );
        err
    })?;

    // Remove key with new format
    let encrypted_key = encrypt_key_new(plaintext_key, contract_key)?;

    let mut encrypted_key_with_header: Vec<u8> = vec![];
    encrypted_key_with_header.extend_from_slice(ENCRYPTED_KEY_MAGIC_BYTES);
    encrypted_key_with_header.extend_from_slice(&CONSENSUS_SEED_VERSION.to_be_bytes());
    encrypted_key_with_header.extend_from_slice(&encrypted_key);

    let gas_used_second_remove = remove_db(context, &encrypted_key_with_header).map_err(|err| {
        warn!(
            "remove_db() got an error from ocall_remove_db on new key remove, stopping wasm: {:?}",
            err
        );
        err
    })?;

    Ok(gas_used_first_remove + gas_used_second_remove)
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
//#[cfg(not(feature = "query-only"))]
#[allow(dead_code)]
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

fn decrypt_value_old(
    field_name: &[u8],
    value: &[u8],
    contract_key: &ContractKey,
) -> Result<Vec<u8>, WasmEngineError> {
    let decryption_key = get_symmetrical_key_old(field_name, contract_key);

    // Slice ad from `value`
    let (ad, encrypted_value) = value.split_at(32);

    decryption_key.decrypt_siv(encrypted_value, Some(&[ad])).map_err(|err| {
        warn!(
            "read_db() got an error while trying to decrypt the value for key {:?}, stopping wasm: {:?}",
            String::from_utf8_lossy(field_name),
            err
        );
        WasmEngineError::DecryptionError
    })
}

fn get_symmetrical_key_old(field_name: &[u8], contract_key: &ContractKey) -> AESKey {
    let consensus_state_ikm = KEY_MANAGER.get_consensus_state_ikm().unwrap();

    // Derive the key to the specific field name
    let mut derivation_data = field_name.to_vec();
    derivation_data.extend_from_slice(contract_key.to_vec().as_slice());
    consensus_state_ikm
        .genesis
        .derive_key_from_this(&derivation_data)
}

fn get_symmetrical_key_new(contract_key: &ContractKey) -> AESKey {
    let consensus_state_ikm: SeedsHolder<AESKey> = KEY_MANAGER.get_consensus_state_ikm().unwrap();
    consensus_state_ikm
        .current
        .derive_key_from_this(contract_key)
}

fn encrypt_value_new(
    encrypted_state_key: &[u8],
    plaintext_state_value: &[u8],
    contract_key: &ContractKey,
) -> Result<Vec<u8>, WasmEngineError> {
    let encryption_key = get_symmetrical_key_new(contract_key);

    encryption_key
        .encrypt_siv(plaintext_state_value, Some(&[encrypted_state_key]))
        .map_err(|err| {
            warn!(
                "write_db() got an error while trying to encrypt_value_new the value '{:?}', stopping wasm: {:?}",
                String::from_utf8_lossy(plaintext_state_value),
                err
            );
            WasmEngineError::EncryptionError
    })
}

/// encrypted_state_key without the header
fn decrypt_value_new(
    encrypted_state_key: &[u8],
    encrypted_state_value: &[u8],
    contract_key: &ContractKey,
) -> Result<Vec<u8>, WasmEngineError> {
    let decryption_key = get_symmetrical_key_new(contract_key);

    decryption_key.decrypt_siv(encrypted_state_value, Some(&[encrypted_state_key])).map_err(|err| {
        warn!(
            "read_db() got an error while trying to decrypt_value_new the value {:?} for key {:?}, stopping wasm: {:?}",
            encrypted_state_value,
            encrypted_state_key,
            err
        );
        WasmEngineError::DecryptionError
    })
}

fn encrypt_key_new(
    plaintext_state_key: &[u8],
    contract_key: &ContractKey,
) -> Result<Vec<u8>, WasmEngineError> {
    let encryption_key = get_symmetrical_key_new(contract_key);

    encryption_key
        .encrypt_siv(plaintext_state_key, Some(&[]))
        .map_err(|err| {
            warn!(
                "write_db() got an error while trying to encrypt_key_new the key '{:?}', stopping wasm: {:?}",
                String::from_utf8_lossy(plaintext_state_key),
                err
            );
            WasmEngineError::EncryptionError
    })
}
