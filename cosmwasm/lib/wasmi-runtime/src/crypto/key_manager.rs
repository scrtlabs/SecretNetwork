use crate::consts::{NODE_SK_SEALING_PATH, MASTER_STATE_KEY_PATH,
                    IO_KEY_SEALING_KEY_PATH, SEED_SEALING_PATH, IO_KEY_DERIVE_ORDER, STATE_MASTER_KEY_DERIVE_ORDER};
use crate::crypto::keys::{KeyPair, Seed};
use crate::crypto::traits::*;
use enclave_ffi_types::{EnclaveError, CryptoError};
use lazy_static::lazy_static;
use log::*;
use ring::hmac::Key;

pub struct Keychain {
    seed: Option<Seed>,
    master_state_key: Option<Seed>,
    io_key: Option<KeyPair>,
    node_key: Option<KeyPair>
}

lazy_static! {
    pub static ref KEY_MANAGER: Keychain = {
        Keychain::new()
    };
}

impl Keychain {
    pub fn new() -> Self {
        let seed = match Seed::unseal(SEED_SEALING_PATH) {
            Ok(k) => Some(k),
            Err(e) => None
        };

        let io_key = match KeyPair::unseal(IO_KEY_SEALING_KEY_PATH) {
            Ok(k) => Some(k),
            Err(e) => None
        };

        let master_state_key = match Seed::unseal(MASTER_STATE_KEY_PATH) {
            Ok(k) => Some(k),
            Err(e) => None
        };

        let node_key = match KeyPair::unseal(NODE_SK_SEALING_PATH) {
            Ok(k) => Some(k),
            Err(e) => None
        };

        Keychain {
            seed,
            master_state_key,
            io_key,
            node_key
        }
    }
    pub fn create_seed(&mut self) -> Result<(), CryptoError> {
        match Seed::new() {
            Ok(seed) => {
                if let Err(e) = self.set_seed(seed) {
                    return Err(CryptoError::KeyError);
                }
            },
            Err(err) => return Err(err),
        };
        Ok(())
    }

    pub fn create_node_key(&mut self) -> Result<(), CryptoError> {
        match KeyPair::new() {
            Ok(key) => {
                if let Err(e) = self.set_node_key(key) {
                    return Err(CryptoError::KeyError);
                }
            },
            Err(err) => return Err(err),
        };
        Ok(())
    }

    pub fn is_node_key_set(&self) -> bool {
        return self.node_key.is_some()
    }

    pub fn is_master_state_key_set(&self) -> bool {
        return self.master_state_key.is_some()
    }

    pub fn is_io_key_set(&self) -> bool {
        return self.io_key.is_some()
    }

    pub fn is_seed_set(&self) -> bool {
        return self.seed.is_some()
    }

    pub fn get_master_state_key(&self) -> Result<Seed, CryptoError> {
        if self.master_state_key.is_some() {
            Ok(self.master_state_key.unwrap())
        } else {
            error!("Error accessing master state key (does not exist, or was not initialized)");
            Err(CryptoError::ParsingError)
        }
    }

    pub fn get_seed(&self) -> Result<Seed, CryptoError> {
        if self.seed.is_some() {
            Ok(self.seed.unwrap())
        } else {
            error!("Error accessing seed (does not exist, or was not initialized)");
            Err(CryptoError::ParsingError)
        }
    }

    pub fn get_io_key(&self) -> Result<KeyPair, CryptoError> {
        if self.io_key.is_some() {
            // KeyPair does not implement copy (due to internal type not implementing it
            Ok(self.io_key.clone().unwrap())
        } else {
            error!("Error accessing io key (does not exist, or was not initialized)");
            Err(CryptoError::ParsingError)
        }
    }

    pub fn get_node_key(&self) -> Result<KeyPair, CryptoError> {
        if self.node_key.is_some() {
            // KeyPair does not implement copy (due to internal type not implementing it
            Ok(self.node_key.clone().unwrap())
        } else {
            error!("Error accessing node key (does not exist, or was not initialized)");
            Err(CryptoError::ParsingError)
        }
    }

    pub fn set_node_key(&mut self, kp: KeyPair) -> Result<(), EnclaveError> {
        if let Err(e) = kp.seal(NODE_SK_SEALING_PATH) {
            error!("Error setting node key");
            return Err(e);
        }
        Ok(self.node_key = Some(kp.clone()))
    }

    pub fn set_io_key(&mut self, kp: KeyPair) -> Result<(), EnclaveError> {
        if let Err(e) = kp.seal(IO_KEY_SEALING_KEY_PATH) {
            error!("Error setting io key");
            return Err(e);
        }
        Ok(self.io_key = Some(kp.clone()))
    }

    pub fn set_master_state_key(&mut self, seed: Seed) -> Result<(), EnclaveError> {
        if let Err(e) = seed.seal(MASTER_STATE_KEY_PATH) {
            error!("Error setting master state key");
            return Err(e);
        }
        Ok(self.master_state_key = Some(seed.clone()))
    }

    pub fn set_seed(&mut self, seed: Seed) -> Result<(), EnclaveError> {
        if let Err(e) = seed.seal(SEED_SEALING_PATH) {
            error!("Error setting seed");
            return Err(e);
        }
        Ok(self.seed = Some(seed.clone()))
    }

    pub fn generate_master_keys(&mut self) -> Result<(), EnclaveError>{

        if !self.is_seed_set() {
            error!("Seed not initialized! Cannot derive enclave keys");
            return Err(EnclaveError::FailedUnseal);
        }

        let io_key_bytes = self.seed.unwrap().derive_key_from_this(IO_KEY_DERIVE_ORDER);
        let io_key = KeyPair::new_from_slice(&io_key_bytes).map_err(|err| {
            error!("[Enclave] Error creating io_key: {:?}", err);
            EnclaveError::FailedUnseal /* change error type? */
        }).unwrap();

        if let Err(e) = self.set_io_key(io_key) {
            return Err(e);
        }

        let master_state_key_bytes = self.seed.unwrap().derive_key_from_this(STATE_MASTER_KEY_DERIVE_ORDER);
        let master_state_key = Seed::new_from_slice(&master_state_key_bytes);

        self.set_master_state_key(master_state_key)
    }
}
