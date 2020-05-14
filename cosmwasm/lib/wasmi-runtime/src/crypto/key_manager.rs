use crate::consts::*;
use crate::crypto::keys::{KeyPair, Seed};
use crate::crypto::traits::*;
use enclave_ffi_types::{CryptoError, EnclaveError};
use lazy_static::lazy_static;
use log::*;

pub struct Keychain {
    consensus_seed: Option<Seed>,
    consensus_base_state_key: Option<Seed>,
    consensus_seed_exchange_keypair: Option<KeyPair>,
    consensus_io_exchange_keypair: Option<KeyPair>,
    new_node_seed_exchange_keypair: Option<KeyPair>,
}

lazy_static! {
    pub static ref KEY_MANAGER: Keychain = Keychain::new();
}

impl Keychain {
    pub fn new() -> Self {
        let consensus_seed = match Seed::unseal(CONSENSUS_SEED_SEALING_PATH) {
            Ok(k) => Some(k),
            Err(e) => None,
        };

        let mut x = Keychain {
            consensus_seed,
            consensus_base_state_key: None,
            consensus_seed_exchange_keypair: None,
            consensus_io_exchange_keypair: None,
            new_node_seed_exchange_keypair: None,
        };

        x.generate_consensus_master_keys();

        return x;

        // let kdf_salt: Vec<u8> = vec![
        //     0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x4b, 0xea, 0xd8, 0xdf,
        //     0x69, 0x99, 0x08, 0x52, 0xc2, 0x02, 0xdb, 0x0e, 0x00, 0x97, 0xc1, 0xa1, 0x2e, 0xa6,
        //     0x37, 0xd7, 0xe9, 0x6d,
        // ]; // Bitcoin halving block hash https://www.blockchain.com/btc/block/000000000000000000024bead8df69990852c202db0e0097c1a12ea637d7e96d
    }

    pub fn create_consensus_seed(&mut self) -> Result<(), CryptoError> {
        match Seed::new() {
            Ok(seed) => {
                if let Err(e) = self.set_consensus_seed(seed) {
                    return Err(CryptoError::KeyError);
                }
            }
            Err(err) => return Err(err),
        };
        Ok(())
    }

    pub fn create_new_node_seed_exchange_keypair(&mut self) -> Result<(), CryptoError> {
        match KeyPair::new() {
            Ok(key) => {
                if let Err(e) = self.set_new_node_seed_exchange_keypair(key) {
                    return Err(CryptoError::KeyError);
                }
            }
            Err(err) => return Err(err),
        };
        Ok(())
    }

    pub fn is_new_node_seed_exchange_keypair_set(&self) -> bool {
        return self.new_node_seed_exchange_keypair.is_some();
    }

    pub fn is_consensus_base_state_key_set(&self) -> bool {
        return self.consensus_base_state_key.is_some();
    }

    pub fn is_consensus_seed_exchange_keypair_set(&self) -> bool {
        return self.consensus_seed_exchange_keypair.is_some();
    }

    pub fn is_consensus_io_exchange_keypair_set(&self) -> bool {
        return self.consensus_io_exchange_keypair.is_some();
    }

    pub fn is_consensus_seed_set(&self) -> bool {
        return self.consensus_seed.is_some();
    }

    pub fn get_consensus_base_state_key(&self) -> Result<Seed, CryptoError> {
        if self.consensus_base_state_key.is_some() {
            Ok(self.consensus_base_state_key.unwrap())
        } else {
            error!("Error accessing base_state_key (does not exist, or was not initialized)");
            Err(CryptoError::ParsingError)
        }
    }

    pub fn get_consensus_seed(&self) -> Result<Seed, CryptoError> {
        if self.consensus_seed.is_some() {
            Ok(self.consensus_seed.unwrap())
        } else {
            error!("Error accessing consensus_seed (does not exist, or was not initialized)");
            Err(CryptoError::ParsingError)
        }
    }

    pub fn get_consensus_seed_exchange_keypair(&self) -> Result<KeyPair, CryptoError> {
        if self.consensus_seed_exchange_keypair.is_some() {
            // KeyPair does not implement copy (due to internal type not implementing it
            Ok(self.consensus_seed_exchange_keypair.clone().unwrap())
        } else {
            error!("Error accessing consensus_seed_exchange_keypair (does not exist, or was not initialized)");
            Err(CryptoError::ParsingError)
        }
    }

    pub fn get_consensus_io_exchange_keypair(&self) -> Result<KeyPair, CryptoError> {
        if self.consensus_io_exchange_keypair.is_some() {
            // KeyPair does not implement copy (due to internal type not implementing it
            Ok(self.consensus_io_exchange_keypair.clone().unwrap())
        } else {
            error!("Error accessing consensus_io_exchange_keypair (does not exist, or was not initialized)");
            Err(CryptoError::ParsingError)
        }
    }

    pub fn get_new_node_seed_exchange_keypair(&self) -> Result<KeyPair, CryptoError> {
        if self.new_node_seed_exchange_keypair.is_some() {
            // KeyPair does not implement copy (due to internal type not implementing it
            Ok(self.new_node_seed_exchange_keypair.clone().unwrap())
        } else {
            error!("Error accessing new_node_seed_exchange_keypair (does not exist, or was not initialized)");
            Err(CryptoError::ParsingError)
        }
    }

    pub fn set_new_node_seed_exchange_keypair(&mut self, kp: KeyPair) -> Result<(), EnclaveError> {
        if let Err(e) = kp.seal(NEW_NODE_SEED_EXCHANGE_KEYPAIR_SEALING_PATH) {
            error!("Error sealing new_node_seed_exchange_keypair");
            return Err(e);
        }
        Ok(self.new_node_seed_exchange_keypair = Some(kp.clone()))
    }

    pub fn set_consensus_seed_exchange_keypair(&mut self, kp: KeyPair) {
        self.consensus_seed_exchange_keypair = Some(kp.clone())
    }

    pub fn set_consensus_io_exchange_keypair(&mut self, kp: KeyPair) {
        self.consensus_io_exchange_keypair = Some(kp.clone())
    }

    pub fn set_consensus_base_state_key(&mut self, consensus_base_state_key: Seed) {
        self.consensus_base_state_key = Some(consensus_base_state_key.clone());
    }

    pub fn set_consensus_seed(&mut self, consensus_seed: Seed) -> Result<(), EnclaveError> {
        if let Err(e) = consensus_seed.seal(CONSENSUS_SEED_SEALING_PATH) {
            error!("Error sealing consensus_seed");
            return Err(e);
        }
        Ok(self.consensus_seed = Some(consensus_seed.clone()))
    }

    pub fn generate_consensus_master_keys(&mut self) -> Result<(), EnclaveError> {
        if !self.is_consensus_seed_set() {
            error!("Seed not initialized! Cannot derive enclave keys");
            return Err(EnclaveError::FailedUnseal);
        }

        // consensus_seed_exchange_keypair

        let consensus_seed_exchange_keypair_bytes = self
            .consensus_seed
            .unwrap()
            .derive_key_from_this(CONSENSUS_SEED_EXCHANGE_KEYPAIR_DERIVE_ORDER);
        let consensus_seed_exchange_keypair =
            KeyPair::new_from_slice(&consensus_seed_exchange_keypair_bytes)
                .map_err(|err| {
                    error!(
                        "[Enclave] Error creating consensus_seed_exchange_keypair: {:?}",
                        err
                    );
                    EnclaveError::FailedUnseal /* change error type? */
                })
                .unwrap();

        self.set_consensus_seed_exchange_keypair(consensus_seed_exchange_keypair);

        // consensus_io_exchange_keypair

        let consensus_io_exchange_keypair_bytes = self
            .consensus_seed
            .unwrap()
            .derive_key_from_this(CONSENSUS_IO_EXCHANGE_KEYPAIR_DERIVE_ORDER);
        let consensus_io_exchange_keypair =
            KeyPair::new_from_slice(&consensus_io_exchange_keypair_bytes)
                .map_err(|err| {
                    error!(
                        "[Enclave] Error creating consensus_io_exchange_keypair: {:?}",
                        err
                    );
                    EnclaveError::FailedUnseal /* change error type? */
                })
                .unwrap();

        self.set_consensus_io_exchange_keypair(consensus_io_exchange_keypair);

        // consensus_base_state_key

        let consensus_base_state_key_bytes = self
            .consensus_seed
            .unwrap()
            .derive_key_from_this(CONSENSUS_BASE_STATE_KEY_DERIVE_ORDER);
        let consensus_base_state_key = Seed::new_from_slice(&consensus_base_state_key_bytes);

        self.set_consensus_base_state_key(consensus_base_state_key);

        Ok(())
    }
}
