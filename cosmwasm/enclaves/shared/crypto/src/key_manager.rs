use crate::consts::*;
use crate::traits::{Kdf, SealedKey};
use crate::CryptoError;
use crate::{AESKey, KeyPair, Seed};
use enclave_ffi_types::EnclaveError;
use lazy_static::lazy_static;
use log::*;

pub struct Keychain {
    consensus_seed: Option<Seed>,
    consensus_state_ikm: Option<AESKey>,
    consensus_seed_exchange_keypair: Option<KeyPair>,
    consensus_io_exchange_keypair: Option<KeyPair>,
    consensus_callback_secret: Option<AESKey>,
    registration_key: Option<KeyPair>,
}

lazy_static! {
    pub static ref KEY_MANAGER: Keychain = Keychain::new();
}

#[allow(clippy::new_without_default)]
impl Keychain {
    pub fn new() -> Self {
        let consensus_seed = match Seed::unseal(&CONSENSUS_SEED_SEALING_PATH) {
            Ok(k) => Some(k),
            Err(_e) => None,
        };

        let registration_key = match KeyPair::unseal(&REGISTRATION_KEY_SEALING_PATH) {
            Ok(k) => Some(k),
            Err(_e) => None,
        };

        let mut x = Keychain {
            consensus_seed,
            registration_key,
            consensus_state_ikm: None,
            consensus_seed_exchange_keypair: None,
            consensus_io_exchange_keypair: None,
            consensus_callback_secret: None,
        };

        let _ = x.generate_consensus_master_keys();

        x
    }

    pub fn create_consensus_seed(&mut self) -> Result<(), CryptoError> {
        match Seed::new() {
            Ok(seed) => {
                if let Err(_e) = self.set_consensus_seed(seed) {
                    return Err(CryptoError::KeyError);
                }
            }
            Err(err) => return Err(err),
        };
        Ok(())
    }

    pub fn create_registration_key(&mut self) -> Result<(), CryptoError> {
        match KeyPair::new() {
            Ok(key) => {
                if let Err(_e) = self.set_registration_key(key) {
                    return Err(CryptoError::KeyError);
                }
            }
            Err(err) => return Err(err),
        };
        Ok(())
    }

    pub fn is_consensus_seed_set(&self) -> bool {
        self.consensus_seed.is_some()
    }

    pub fn get_consensus_state_ikm(&self) -> Result<AESKey, CryptoError> {
        self.consensus_state_ikm.ok_or_else(|| {
            error!("Error accessing base_state_key (does not exist, or was not initialized)");
            CryptoError::ParsingError
        })
    }

    pub fn get_consensus_seed(&self) -> Result<Seed, CryptoError> {
        self.consensus_seed.ok_or_else(|| {
            error!("Error accessing consensus_seed (does not exist, or was not initialized)");
            CryptoError::ParsingError
        })
    }

    pub fn seed_exchange_key(&self) -> Result<KeyPair, CryptoError> {
        self.consensus_seed_exchange_keypair.ok_or_else(|| {
            error!("Error accessing consensus_seed_exchange_keypair (does not exist, or was not initialized)");
            CryptoError::ParsingError
        })
    }

    pub fn get_consensus_io_exchange_keypair(&self) -> Result<KeyPair, CryptoError> {
        self.consensus_io_exchange_keypair.ok_or_else(|| {
            error!("Error accessing consensus_io_exchange_keypair (does not exist, or was not initialized)");
            CryptoError::ParsingError
        })
    }

    pub fn get_consensus_callback_secret(&self) -> Result<AESKey, CryptoError> {
        self.consensus_callback_secret.ok_or_else(|| {
            error!("Error accessing consensus_callback_secret (does not exist, or was not initialized)");
            CryptoError::ParsingError
        })
    }

    pub fn get_registration_key(&self) -> Result<KeyPair, CryptoError> {
        self.registration_key.ok_or_else(|| {
            error!("Error accessing registration_key (does not exist, or was not initialized)");
            CryptoError::ParsingError
        })
    }

    pub fn set_registration_key(&mut self, kp: KeyPair) -> Result<(), EnclaveError> {
        if let Err(e) = kp.seal(&REGISTRATION_KEY_SEALING_PATH) {
            error!("Error sealing registration key");
            return Err(e);
        }
        self.registration_key = Some(kp);
        Ok(())
    }

    pub fn set_consensus_seed_exchange_keypair(&mut self, kp: KeyPair) {
        self.consensus_seed_exchange_keypair = Some(kp)
    }

    pub fn set_consensus_io_exchange_keypair(&mut self, kp: KeyPair) {
        self.consensus_io_exchange_keypair = Some(kp)
    }

    pub fn set_consensus_state_ikm(&mut self, consensus_state_ikm: AESKey) {
        self.consensus_state_ikm = Some(consensus_state_ikm);
    }

    pub fn set_consensus_callback_secret(&mut self, consensus_callback_secret: AESKey) {
        self.consensus_callback_secret = Some(consensus_callback_secret);
    }

    pub fn set_consensus_seed(&mut self, consensus_seed: Seed) -> Result<(), EnclaveError> {
        debug!("Sealing consensus seed in {}", *CONSENSUS_SEED_SEALING_PATH);
        if let Err(e) = consensus_seed.seal(&CONSENSUS_SEED_SEALING_PATH) {
            error!("Error sealing consensus_seed");
            return Err(e);
        }
        self.consensus_seed = Some(consensus_seed);
        Ok(())
    }

    pub fn generate_consensus_master_keys(&mut self) -> Result<(), EnclaveError> {
        if !self.is_consensus_seed_set() {
            trace!("Seed not initialized, skipping derivation of enclave keys");
            return Ok(());
        }

        // consensus_seed_exchange_keypair

        let consensus_seed_exchange_keypair_bytes = self
            .consensus_seed
            .unwrap()
            .derive_key_from_this(&CONSENSUS_SEED_EXCHANGE_KEYPAIR_DERIVE_ORDER.to_be_bytes());
        let consensus_seed_exchange_keypair = KeyPair::from(consensus_seed_exchange_keypair_bytes);
        trace!(
            "consensus_seed_exchange_keypair: {:?}",
            hex::encode(consensus_seed_exchange_keypair.get_pubkey())
        );
        self.set_consensus_seed_exchange_keypair(consensus_seed_exchange_keypair);

        // consensus_io_exchange_keypair

        let consensus_io_exchange_keypair_bytes = self
            .consensus_seed
            .unwrap()
            .derive_key_from_this(&CONSENSUS_IO_EXCHANGE_KEYPAIR_DERIVE_ORDER.to_be_bytes());
        let consensus_io_exchange_keypair = KeyPair::from(consensus_io_exchange_keypair_bytes);
        trace!(
            "consensus_io_exchange_keypair: {:?}",
            hex::encode(consensus_io_exchange_keypair.get_pubkey())
        );
        self.set_consensus_io_exchange_keypair(consensus_io_exchange_keypair);

        // consensus_state_ikm

        let consensus_state_ikm = self
            .consensus_seed
            .unwrap()
            .derive_key_from_this(&CONSENSUS_STATE_IKM_DERIVE_ORDER.to_be_bytes());

        trace!(
            "consensus_state_ikm: {:?}",
            hex::encode(consensus_state_ikm.get())
        );
        self.set_consensus_state_ikm(consensus_state_ikm);

        let consensus_callback_secret = self
            .consensus_seed
            .unwrap()
            .derive_key_from_this(&CONSENSUS_CALLBACK_SECRET_DERIVE_ORDER.to_be_bytes());

        trace!(
            "consensus_state_ikm: {:?}",
            hex::encode(consensus_state_ikm.get())
        );
        self.set_consensus_callback_secret(consensus_callback_secret);

        Ok(())
    }
}

#[cfg(feature = "test")]
pub mod tests {

    use super::{
        Keychain, CONSENSUS_SEED_SEALING_PATH, /*KEY_MANAGER,*/ REGISTRATION_KEY_SEALING_PATH,
    };
    // use crate::crypto::CryptoError;
    // use crate::crypto::{KeyPair, Seed};

    // todo: fix test vectors to actually work
    fn _test_initial_keychain_state() {
        // clear previous data (if any)
        let _ = std::sgxfs::remove(&*CONSENSUS_SEED_SEALING_PATH);
        let _ = std::sgxfs::remove(&*REGISTRATION_KEY_SEALING_PATH);

        let _keys = Keychain::new();

        // todo: replace with actual checks
        // assert_eq!(keys.get_registration_key(), Err(CryptoError));
        // assert_eq!(keys.get_consensus_seed(), Err(CryptoError));
        // assert_eq!(keys.get_consensus_io_exchange_keypair(), Err(CryptoError));
        // assert_eq!(keys.get_consensus_state_ikm(), Err(CryptoError));
    }

    // commented out since it uses outdated methods
    // // todo: fix test vectors to actually work
    // fn test_initialize_keychain_seed() {
    //     // clear previous data (if any)
    //     std::sgxfs::remove(&*CONSENSUS_SEED_SEALING_PATH);
    //     std::sgxfs::remove(&*REGISTRATION_KEY_SEALING_PATH);
    //
    //     let mut keys = Keychain::new();
    //
    //     let seed = Seed::new_from_slice(b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA");
    //
    //     keys.set_consensus_seed(seed);
    //     keys.generate_consensus_master_keys();
    //     // todo: replace with actual checks
    //     // assert_eq!(keys.get_registration_key(), Err(CryptoError));
    //     assert_eq!(keys.get_consensus_seed().unwrap(), seed);
    // }

    // // todo: fix test vectors to actually work
    // fn test_initialize_keychain_registration() {
    //     // clear previous data (if any)
    //     std::sgxfs::remove(&*CONSENSUS_SEED_SEALING_PATH);
    //     std::sgxfs::remove(&*REGISTRATION_KEY_SEALING_PATH);
    //
    //     let mut keys = Keychain::new();
    //
    //     let kp = KeyPair::new_from_slice(b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA").unwrap();
    //
    //     keys.set_registration_key(kp);
    //     // todo: replace with actual checks
    //     assert_eq!(keys.get_registration_key().unwrap(), kp);
    // }
    //
    // // todo: fix test vectors to actually work
    // fn test_initialize_keys() {
    //     // clear previous data (if any)
    //     std::sgxfs::remove(&*CONSENSUS_SEED_SEALING_PATH);
    //     std::sgxfs::remove(&*REGISTRATION_KEY_SEALING_PATH);
    //
    //     let mut keys = Keychain::new();
    //
    //     let seed = Seed::new_from_slice(b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA");
    //
    //     keys.set_consensus_seed(seed);
    //     keys.generate_consensus_master_keys();
    //     // todo: replace with actual checks
    //     assert_eq!(keys.get_consensus_io_exchange_keypair().unwrap(), seed);
    //     assert_eq!(keys.get_consensus_state_ikm().unwrap(), seed);
    // }
    //
    // // todo: fix test vectors to actually work
    // fn test_key_manager() {
    //     // clear previous data (if any)
    //     std::sgxfs::remove(&*CONSENSUS_SEED_SEALING_PATH);
    //     std::sgxfs::remove(&*REGISTRATION_KEY_SEALING_PATH);
    //
    //     let seed = Seed::new_from_slice(b"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA");
    //     let mut keys = Keychain::new();
    //     keys.set_consensus_seed(seed);
    //     keys.generate_consensus_master_keys();
    //
    //     // todo: replace with actual checks
    //     assert_eq!(
    //         KEY_MANAGER.get_consensus_io_exchange_keypair().unwrap(),
    //         seed
    //     );
    //     assert_eq!(KEY_MANAGER.get_consensus_state_ikm().unwrap(), seed);
    // }

    // use crate::crypto::{AESKey, SIVEncryptable, Seed, KEY_MANAGER};
    // // This is commented out because it's trying to modify KEY_MANAGER which is immutable.
    // // todo: fix test vectors to actually work
    // pub fn test_msg_decrypt() {
    //     let seed = Seed::new().unwrap();
    //
    //     KEY_MANAGER
    //         .set_consensus_seed(seed)
    //         .expect("Failed to set seed");
    //
    //     let nonce = [0u8; 32];
    //     let user_public_key = [0u8; 32];
    //
    //     let msg = "{\"ok\": \"{\"balance\": \"108\"}\"}";
    //     let key = calc_encryption_key(&nonce, &user_public_key);
    //
    //     let encrypted_msg = key.encrypt_siv(msg.as_bytes(), &[&[]]);
    //
    //     let secret_msg = SecretMessage {
    //         nonce,
    //         user_public_key,
    //         msg: encrypted_msg,
    //     };
    //
    //     let decrypted_msg = secret_msg.decrypt()?;
    //
    //     assert_eq!(decrypted_msg, msg)
    // }
}
