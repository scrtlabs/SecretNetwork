use super::contract_validation::ContractKey;
use super::errors::DbError;
use crate::crypto::{sha_256, AESKey, Kdf, SIVEncryptable, KEY_MANAGER};
use crate::{exports, imports};

use enclave_ffi_types::{Ctx, EnclaveBuffer};

use log::*;
use sgx_types::{sgx_status_t, SgxError, SgxResult};

pub fn write_encrypted_key(
    key: &[u8],
    value: &[u8],
    context: &Ctx,
    contract_key: &ContractKey,
) -> Result<(), DbError> {
    // Get the state key from the key manager

    let scrambled_field_name = field_name_digest(key, contract_key);

    debug!(
        "Writing to scrambled field name: {:?}",
        scrambled_field_name
    );

    let ad = derive_ad_for_field(&scrambled_field_name, &context)?;

    let encrypted_value = encrypt_key(&scrambled_field_name, value, contract_key, &ad)?;

    let mut db_data: Vec<u8> = ad.to_vec();
    db_data.extend_from_slice(encrypted_value.as_slice());

    // Write the new data as concat(ad, encrypted_val)
    write_db(context, &scrambled_field_name, &db_data).map_err(|err| {
        error!(
            "write_db() go an error from ocall_write_db, stopping wasm: {:?}",
            err
        );
        DbError::FailedWrite
    })?;

    Ok(())
}

pub fn read_encrypted_key(
    key: &[u8],
    context: &Ctx,
    contract_key: &ContractKey,
) -> Result<Vec<u8>, DbError> {
    let scrambled_field_name = field_name_digest(key, contract_key);

    debug!(
        "Reading from scrambled field name: {:?}",
        scrambled_field_name
    );

    // Call read_db (this bubbles up to Tendermint via ocalls and FFI to Go code)
    // This returns the value from Tendermint
    // fn read_db(context: Ctx, key: &[u8]) -> Option<Vec<u8>> {
    let value = read_db(context, &scrambled_field_name)
        .map_err(|err| {
            error!(
                "read_db() got an error from ocall_read_db, stopping wasm: {:?}",
                err
            );
            DbError::FailedRead
        })
        .map(|val| {
            if val.is_some() {
                decrypt_key(&scrambled_field_name, val.unwrap().as_slice(), contract_key)
            } else {
                return Err(DbError::EmptyValue);
            }
        })?;

    value
}

pub fn field_name_digest(field_name: &[u8], contract_key: &ContractKey) -> [u8; 32] {
    let mut data: Vec<u8> = field_name.to_vec();
    data.extend_from_slice(contract_key);

    sha_256(&data)
}

/// Safe wrapper around reads from the contract storage
fn read_db(context: &Ctx, key: &[u8]) -> SgxResult<Option<Vec<u8>>> {
    let mut enclave_buffer = std::mem::MaybeUninit::<EnclaveBuffer>::uninit();
    unsafe {
        match imports::ocall_read_db(
            enclave_buffer.as_mut_ptr(),
            context.clone(),
            key.as_ptr(),
            key.len(),
        ) {
            sgx_status_t::SGX_SUCCESS => { /* continue */ }
            error_status => return Err(error_status),
        }
        let enclave_buffer = enclave_buffer.assume_init();
        // TODO add validation of this pointer before returning its contents.
        Ok(exports::recover_buffer(enclave_buffer))
    }
}

/// Safe wrapper around writes to the contract storage
fn write_db(context: &Ctx, key: &[u8], value: &[u8]) -> SgxError {
    match unsafe {
        imports::ocall_write_db(
            context.clone(),
            key.as_ptr(),
            key.len(),
            value.as_ptr(),
            value.len(),
        )
    } {
        sgx_status_t::SGX_SUCCESS => Ok(()),
        err => Err(err),
    }
}

fn derive_ad_for_field(field_name: &[u8], context: &Ctx) -> Result<[u8; 32], DbError> {
    let ad = match read_db(context, field_name).map_err(|err| {
        error!(
            "read_db() got an error from ocall_read_db, stopping wasm: {:?}",
            err
        );
        DbError::FailedRead
    })? {
        None => {
            // No data exist yet for this state_key_name, so creating a new `ad`
            sha_256(&field_name)
        }
        Some(old_value) => {
            // Extract previous_ad to calculate the new ad (first 32 bytes)
            let (prev_ad, _) = old_value.split_at(32);
            sha_256(prev_ad)
        }
    };

    Ok(ad)
}

fn encrypt_key(
    field_name: &[u8],
    value: &[u8],
    contract_key: &ContractKey,
    ad: &[u8],
) -> Result<Vec<u8>, DbError> {
    let encryption_key = get_symmetrical_key(field_name, contract_key);

    let mut encrypted_value = encryption_key
        .encrypt_siv(&value, &vec![ad])
        .map_err(|err| {
            error!(
                "write_db() got an error while trying to encrypt the value {:?}, stopping wasm: {:?}",
                String::from_utf8_lossy(&value),
                err
            );
            DbError::FailedEncryption
        });

    encrypted_value
}

fn decrypt_key(
    field_name: &[u8],
    value: &[u8],
    contract_key: &ContractKey,
) -> Result<Vec<u8>, DbError> {
    let decryption_key = get_symmetrical_key(field_name, contract_key);

    // Slice ad from `value`
    let (ad, encrypted_value) = value.split_at(32);

    let decrypted_value = decryption_key.decrypt_siv(&encrypted_value, &vec![ad]).map_err(|err| {
        error!(
            "read_db() got an error while trying to decrypt the value for key {:?}, stopping wasm: {:?}",
            String::from_utf8_lossy(&field_name),
            err
        );
        DbError::FailedDecryption
    });

    decrypted_value
}

fn get_symmetrical_key(field_name: &[u8], contract_key: &ContractKey) -> AESKey {
    let consensus_state_ikm = KEY_MANAGER.get_consensus_state_ikm().unwrap();

    // Derive the key to the specific field name
    let mut derivation_data = field_name.to_vec();
    derivation_data.extend_from_slice(contract_key.to_vec().as_slice());
    let encryption_key = consensus_state_ikm.derive_key_from_this(&derivation_data);

    encryption_key
}
