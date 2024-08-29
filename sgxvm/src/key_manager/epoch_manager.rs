use sgx_types::{sgx_status_t, SgxResult};
use serde::{Deserialize, Serialize};
use std::vec::Vec;
use std::string::String;

use crate::key_manager::{utils, keys, consts};
use crate::encryption;

#[derive(Serialize, Deserialize, Clone)]
pub struct Epoch {
    pub epoch_number: u16,
    pub starting_block: u64,
    epoch_key: [u8; 32],
}

impl Epoch {
    pub fn get_tx_key(&self) -> keys::TransactionEncryptionKey {
        let tx_key_bytes = utils::derive_key(&self.epoch_key, consts::TX_KEY_PREFIX);
        keys::TransactionEncryptionKey::from(tx_key_bytes)
    }

    pub fn get_state_key(&self) -> keys::StateEncryptionKey {
        let state_key_bytes = utils::derive_key(&self.epoch_key, consts::STATE_KEY_PREFIX);
        keys::StateEncryptionKey::from(state_key_bytes)
    }
}

#[derive(Serialize, Deserialize, Clone)]
pub struct EpochManager {
    epochs: Vec<Epoch>
}

impl EpochManager {

    #[cfg(feature = "attestation_server")]
    /// Generates new epoch with random epoch key, which starts from provided
    /// `starting_block` param. Returns new instance of EpochManager with added epoch.
    pub fn add_new_epoch(&self, starting_block: u64) -> SgxResult<EpochManager> {
        let mut updated_epoch_manager = self.clone();

        // Check starting block value. It should be greater than existing epochs starting blocks
        for epoch in &updated_epoch_manager.epochs {
            if epoch.starting_block >= starting_block {
                println!("[EpochManager] There is already existing epoch with greater starting block. Existing: {:?}, Provided: {:?}", epoch.starting_block, starting_block);
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }
        }

        // Get number of latest epoch
        let latest_epoch_number = match updated_epoch_manager.epochs.iter().max_by_key(|epoch| epoch.epoch_number) {
            Some(epoch) => epoch.epoch_number,
            None => {
                println!("[EpochManager] There are no epochs");
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }
        };

        let epoch_key = utils::random_bytes32().map_err(|err| {
            println!("[EpochManager] Cannot create random epoch key. Reason: {:?}", err);
            err
        })?;

        let epoch = Epoch {
            epoch_key, 
            starting_block,
            epoch_number: latest_epoch_number + 1,
        };

        updated_epoch_manager.epochs.push(epoch);

        Ok(updated_epoch_manager)
    }


    #[cfg(feature = "attestation_server")]
    /// Removes latest epoch. 
    /// Returns error if there only one epoch or epoch manager was not properly initialized.
    /// Returns updated epoch manager without latest epoch.
    pub fn remove_latest_epoch(&self) -> SgxResult<EpochManager> {
        if self.epochs.len() <= 1 {
            println!("[EpochManager] Cannot remove last epoch");
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }

        let mut updated_epochs = self.epochs.clone();

        // Get number of latest epoch
        let latest_epoch_number = match updated_epochs.iter().max_by_key(|epoch| epoch.epoch_number) {
            Some(epoch) => epoch.epoch_number,
            None => {
                println!("[EpochManager] There are no epochs");
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }
        };

        // Remove latest epoch
        let latest_epoch_position = match updated_epochs.iter().position(|epoch| epoch.epoch_number == latest_epoch_number) {
            Some(pos) => pos,
            None => {
                println!("[EpochManager] Cannot find latest epoch. Looks like a bug");
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }
        };
        updated_epochs.remove(latest_epoch_position);

        let updated_epoch_manager = EpochManager {
            epochs: updated_epochs
        };

        Ok(updated_epoch_manager)
    }

    // Returns epoch number and starting block for each stored epoch
    pub fn list_epochs(&self) -> Vec<(u16, u64, Vec<u8>)> {
        let mut output = Vec::new();

        for epoch in self.epochs.iter() {
            output.push((epoch.epoch_number, epoch.starting_block, epoch.get_tx_key().public_key()));
        }

        output
    }

    pub fn get_epoch(&self, epoch_number: u16) -> Option<&Epoch> {
        for epoch in self.epochs.iter() {
            if epoch.epoch_number == epoch_number {
                return Some(epoch)
            }
        }
        None
    }

    pub fn get_current_epoch(&self, block_number: u64) -> Option<&Epoch> {
        if self.epochs.is_empty() {
            return None
        }
        
        let mut current_epoch = &self.epochs[0];
        for epoch in self.epochs.iter() {
            if epoch.starting_block > current_epoch.starting_block && epoch.starting_block <= block_number {
                current_epoch = epoch;
            }
        }

        Some(current_epoch)
    }

    pub fn serialize(&self) -> SgxResult<String> {
        let encoded = serde_json::to_string(&self).map_err(|err| {
            println!("[EpochManager] Cannot serialize. Reason: {:?}", err);
            sgx_status_t::SGX_ERROR_UNEXPECTED
        })?;

        Ok(encoded)
    }

    pub fn deserialize(input: &str) -> SgxResult<Self> {
        let epoch_manager: EpochManager = serde_json::from_str(input).map_err(|err| {
            println!("[EpochManager] Cannot deserialize. Reason: {:?}", err);
            sgx_status_t::SGX_ERROR_UNEXPECTED
        })?;

        Ok(epoch_manager)
    }

    pub fn deserialize_from_slice(input: &[u8]) -> SgxResult<Self> {
        let epoch_manager: EpochManager = serde_json::from_slice(input).map_err(|err| {
            println!("[EpochManager] Cannot deserialize from slice. Reason: {:?}", err);
            sgx_status_t::SGX_ERROR_UNEXPECTED
        })?;

        Ok(epoch_manager)
    }

    pub fn random_with_single_epoch() -> SgxResult<Self> {
        let epoch_key = utils::random_bytes32().map_err(|err| {
            println!("[EpochManager] Cannot create random epoch key. Reason: {:?}", err);
            err
        })?;
        let epoch_number = 0u16;
        let starting_block = 0u64;

        let epoch = Epoch {epoch_number, epoch_key, starting_block};
        Ok(Self {
            epochs: vec![epoch],
        })
    }

    pub fn from_seed(input: [u8; 32]) -> Self {
        let epoch = Epoch {
            epoch_number: 0u16,
            starting_block: 0u64,
            epoch_key: input,
        };

        Self {
            epochs: vec![epoch],
        }
    }

    #[cfg(feature = "attestation_server")]
    pub fn encrypt(
        &self,
        reg_key: &keys::RegistrationKey,
        public_key: Vec<u8>,
    ) -> SgxResult<Vec<u8>> {
        // Convert public key to appropriate format
        let public_key: [u8; 32] = public_key.try_into().map_err(|err| {
            println!("[EpochManager] Cannot convert public key during encryption. Reason: {:?}", err);
            sgx_status_t::SGX_ERROR_UNEXPECTED
        })?;
        let public_key = x25519_dalek::PublicKey::from(public_key);

        let shared_secret = reg_key.diffie_hellman(public_key);
        let encoded_epoch_manager = self.serialize()?;
        let encrypted_value = encryption::encrypt_deoxys(shared_secret.as_bytes(), encoded_epoch_manager.as_bytes().to_vec(), None).map_err(|err| {
            println!("[EpochManager] Cannot encrypt serialized epoch manager. Reason: {:?}", err);
            sgx_status_t::SGX_ERROR_UNEXPECTED
        })?;

        // Add public key as prefix
        let reg_public_key = reg_key.public_key();
        Ok([reg_public_key.as_bytes(), encrypted_value.as_slice()].concat())
    }

    pub fn decrypt(
        reg_key: &keys::RegistrationKey,
        public_key: Vec<u8>,
        encrypted_epoch_data: Vec<u8>,
    ) -> SgxResult<Self> {
        // Convert public key to appropriate format
        let public_key: [u8; 32] = public_key.try_into().map_err(|err| {
            println!("[EpochManager] Cannot convert public key during decryption. Reason: {:?}", err);
            sgx_status_t::SGX_ERROR_UNEXPECTED
        })?;
        let public_key = x25519_dalek::PublicKey::from(public_key);

        // Derive shared secret
        let shared_secret = reg_key.diffie_hellman(public_key);

        // Decrypt epoch data
        let epoch_data = encryption::decrypt_deoxys(shared_secret.as_bytes(), encrypted_epoch_data).map_err(|err| {
            println!("[EpochManager] Cannot decrypt serialized epoch manager. Reason: {:?}", err);
            sgx_status_t::SGX_ERROR_UNEXPECTED
        })?;
        let epoch_manager = EpochManager::deserialize_from_slice(&epoch_data)?;

        Ok(epoch_manager)
    }
}

