use crate::consts::*;
use crate::traits::{Kdf, SealedKey};
use crate::CryptoError;
use crate::{AESKey, KeyPair, Seed};
use enclave_ffi_types::EnclaveError;
use lazy_static::lazy_static;
use log::*;

// For phase 1 of the seed rotation, all consensus secrets come in two parts:
// 1. The genesis seed generated on 15 September 2020
// 2. The current seed
//
// The first "current seed" will be generated on the "phase 1 of the seed rotation" upgrade.
// Then we'll start building a list of state keys per contract, and we'll add to that list whenever
// we see a contract access a key that's not in the list.
//
// If a key is on the list - it means the value is encrypted with the current seed
// If a key is NOT on the list - it means the value is still encrypted with the genesis seed
//
// When we rotate the seed in the future, we'll iterate the list of keys and reencrypt the values with
// the new seed.
//
// When a contract accesses a key that's not on the list, we'll add the key to the list and reencrypt
// the value with the new seed.
//
// All of this is needed because currently the encryption key of the value is derived using the
// plaintext key, so we don't know the list of keys of any contract. The keys are stored
// as sha256(key) encrypted with the seed.
pub struct Keychain {
    consensus_seed_id: u16,
    consensus_seed: Option<SeedsHolder<Seed>>,
    consensus_state_ikm: Option<SeedsHolder<AESKey>>,
    consensus_seed_exchange_keypair: Option<SeedsHolder<KeyPair>>,
    consensus_io_exchange_keypair: Option<SeedsHolder<KeyPair>>,
    consensus_callback_secret: Option<SeedsHolder<AESKey>>,
    #[cfg(feature = "random")]
    pub random_encryption_key: Option<AESKey>,
    #[cfg(feature = "random")]
    pub initial_randomness_seed: Option<AESKey>,
    registration_key: Option<KeyPair>,
    admin_proof_secret: Option<AESKey>,
    contract_key_proof_secret: Option<AESKey>,
}

#[derive(Clone, Copy, Default)]
pub struct SeedsHolder<T> {
    pub genesis: T,
    pub current: T,
}

lazy_static! {
    pub static ref KEY_MANAGER: Keychain = Keychain::new();
}

#[allow(clippy::new_without_default)]
impl Keychain {
    pub fn new() -> Self {
        for path in [
            GENESIS_CONSENSUS_SEED_SEALING_PATH.as_str(),
            CURRENT_CONSENSUS_SEED_SEALING_PATH.as_str(),
        ] {
            println!("REUVEN");
            println!("exporting auto key for {:?}", path);
            let ak = std::sgxfs::export_auto_key(path).map(|k| Vec::from(k.as_ref()));
            println!("auto key for {:?} is {:?}", path, ak);
            let ak = std::sgxfs::export_align_auto_key(path).map(|k| Vec::from(k.key.as_ref()));
            println!("aligned auto key for {:?} is {:?}", path, ak);
        }

        let consensus_seed: Option<SeedsHolder<Seed>> = match (
            Seed::unseal(GENESIS_CONSENSUS_SEED_SEALING_PATH.as_str()),
            Seed::unseal(CURRENT_CONSENSUS_SEED_SEALING_PATH.as_str()),
        ) {
            (Ok(genesis), Ok(current)) => {
                trace!(
                    "New keychain created with the following seeds {:?}, {:?}",
                    genesis.as_slice(),
                    current.as_slice()
                );
                Some(SeedsHolder { genesis, current })
            }
            (Err(e), _) => {
                trace!("Failed to unseal seeds {}", e);
                None
            }
            (_, Err(e)) => {
                trace!("Failed to unseal seeds {}", e);
                None
            }
        };

        let registration_key = Self::unseal_registration_key();

        let mut x = Keychain {
            consensus_seed_id: CONSENSUS_SEED_VERSION,
            consensus_seed,
            registration_key,
            consensus_state_ikm: None,
            consensus_seed_exchange_keypair: None,
            consensus_io_exchange_keypair: None,
            consensus_callback_secret: None,
            #[cfg(feature = "random")]
            initial_randomness_seed: None,
            #[cfg(feature = "random")]
            random_encryption_key: None,
            admin_proof_secret: None,
            contract_key_proof_secret: None,
        };

        let _ = x.generate_consensus_master_keys();

        x
    }

    fn unseal_registration_key() -> Option<KeyPair> {
        match KeyPair::unseal(REGISTRATION_KEY_SEALING_PATH.as_str()) {
            Ok(k) => Some(k),
            _ => None,
        }
    }

    pub fn unseal_only_genesis(&mut self) -> Result<(), CryptoError> {
        match Seed::unseal(GENESIS_CONSENSUS_SEED_SEALING_PATH.as_str()) {
            Ok(genesis) => {
                let current = Seed::new()?;
                self.consensus_seed = Some(SeedsHolder { genesis, current });
                Ok(())
            }
            Err(e) => {
                trace!("Failed to unseal consensus seed {}", e);
                Err(CryptoError::KeyError)
            }
        }
    }

    pub fn create_consensus_seed(&mut self) -> Result<(), CryptoError> {
        match (Seed::new(), Seed::new()) {
            (Ok(genesis), Ok(current)) => {
                if let Err(_e) = self.set_consensus_seed(genesis, current) {
                    return Err(CryptoError::KeyError);
                }
            }
            (Err(err), _) => return Err(err),
            (_, Err(err)) => return Err(err),
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

    pub fn get_consensus_state_ikm(&self) -> Result<SeedsHolder<AESKey>, CryptoError> {
        self.consensus_state_ikm.ok_or_else(|| {
            error!("Error accessing base_state_key (does not exist, or was not initialized)");
            CryptoError::ParsingError
        })
    }

    pub fn get_consensus_seed_id(&self) -> u16 {
        self.consensus_seed_id
    }

    pub fn inc_consensus_seed_id(&mut self) {
        self.consensus_seed_id += 1;
    }

    pub fn get_consensus_seed(&self) -> Result<SeedsHolder<Seed>, CryptoError> {
        self.consensus_seed.ok_or_else(|| {
            error!("Error accessing consensus_seed (does not exist, or was not initialized)");
            CryptoError::ParsingError
        })
    }

    pub fn seed_exchange_key(&self) -> Result<SeedsHolder<KeyPair>, CryptoError> {
        self.consensus_seed_exchange_keypair.ok_or_else(|| {
            error!("Error accessing consensus_seed_exchange_keypair (does not exist, or was not initialized)");
            CryptoError::ParsingError
        })
    }

    pub fn get_consensus_io_exchange_keypair(&self) -> Result<SeedsHolder<KeyPair>, CryptoError> {
        self.consensus_io_exchange_keypair.ok_or_else(|| {
            error!("Error accessing consensus_io_exchange_keypair (does not exist, or was not initialized)");
            CryptoError::ParsingError
        })
    }

    pub fn get_consensus_callback_secret(&self) -> Result<SeedsHolder<AESKey>, CryptoError> {
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

    pub fn get_admin_proof_secret(&self) -> Result<AESKey, CryptoError> {
        self.admin_proof_secret.ok_or_else(|| {
            error!("Error accessing admin_proof_secret (does not exist, or was not initialized)");
            CryptoError::ParsingError
        })
    }

    pub fn get_contract_key_proof_secret(&self) -> Result<AESKey, CryptoError> {
        self.contract_key_proof_secret.ok_or_else(|| {
            error!("Error accessing contract_key_proof_secret (does not exist, or was not initialized)");
            CryptoError::ParsingError
        })
    }

    pub fn reseal_registration_key(&mut self) -> Result<(), EnclaveError> {
        match Self::unseal_registration_key() {
            Some(kp) => {
                if let Err(_e) = std::sgxfs::remove(&*REGISTRATION_KEY_SEALING_PATH) {
                    error!("Failed to reseal registration key - error code 0xC11");
                    return Err(EnclaveError::FailedSeal);
                };
                if let Err(_e) = kp.seal(REGISTRATION_KEY_SEALING_PATH.as_str()) {
                    error!("Failed to reseal registration key - error code 0xC12");
                    return Err(EnclaveError::FailedSeal);
                }
                Ok(())
            }
            None => Ok(()),
        }
    }

    pub fn set_registration_key(&mut self, kp: KeyPair) -> Result<(), EnclaveError> {
        if let Err(e) = kp.seal(REGISTRATION_KEY_SEALING_PATH.as_str()) {
            error!("Error sealing registration key - error code 0xC13");
            return Err(e);
        }
        self.registration_key = Some(kp);
        Ok(())
    }

    pub fn set_consensus_seed_exchange_keypair(&mut self, genesis: KeyPair, current: KeyPair) {
        self.consensus_seed_exchange_keypair = Some(SeedsHolder { genesis, current })
    }

    pub fn set_consensus_io_exchange_keypair(&mut self, genesis: KeyPair, current: KeyPair) {
        self.consensus_io_exchange_keypair = Some(SeedsHolder { genesis, current })
    }

    pub fn set_consensus_state_ikm(&mut self, genesis: AESKey, current: AESKey) {
        self.consensus_state_ikm = Some(SeedsHolder { genesis, current });
    }

    pub fn set_consensus_callback_secret(&mut self, genesis: AESKey, current: AESKey) {
        self.consensus_callback_secret = Some(SeedsHolder { genesis, current });
    }

    /// used to remove the consensus seed - usually we don't care whether deletion was successful or not,
    /// since we want to try and delete it either way
    pub fn delete_consensus_seed(&mut self) -> bool {
        debug!(
            "Removing genesis consensus seed in {}",
            *GENESIS_CONSENSUS_SEED_SEALING_PATH
        );
        if let Err(_e) = std::sgxfs::remove(GENESIS_CONSENSUS_SEED_SEALING_PATH.as_str()) {
            debug!("Error removing genesis consensus_seed");
            return false;
        }

        debug!(
            "Removing current consensus seed in {}",
            *CURRENT_CONSENSUS_SEED_SEALING_PATH
        );
        if let Err(_e) = std::sgxfs::remove(CURRENT_CONSENSUS_SEED_SEALING_PATH.as_str()) {
            debug!("Error removing genesis consensus_seed");
            return false;
        }
        self.consensus_seed = None;
        true
    }

    pub fn set_consensus_seed(&mut self, genesis: Seed, current: Seed) -> Result<(), EnclaveError> {
        trace!(
            "Consensus seeds were set to be the following {:?}, {:?}",
            genesis.as_slice(),
            current.as_slice()
        );

        debug!(
            "Sealing genesis consensus seed in {}",
            *GENESIS_CONSENSUS_SEED_SEALING_PATH
        );
        if let Err(e) = genesis.seal(GENESIS_CONSENSUS_SEED_SEALING_PATH.as_str()) {
            error!("Error sealing genesis consensus_seed - error code 0xC14");
            return Err(e);
        }

        debug!(
            "Sealing current consensus seed in {}",
            *CURRENT_CONSENSUS_SEED_SEALING_PATH
        );
        if let Err(e) = current.seal(CURRENT_CONSENSUS_SEED_SEALING_PATH.as_str()) {
            error!("Error sealing current consensus_seed - error code 0xC14");
            return Err(e);
        }

        self.consensus_seed = Some(SeedsHolder { genesis, current });
        Ok(())
    }

    pub fn generate_consensus_master_keys(&mut self) -> Result<(), EnclaveError> {
        if !self.is_consensus_seed_set() {
            trace!("Seed not initialized, skipping derivation of enclave keys");
            return Ok(());
        }

        // consensus_seed_exchange_keypair

        let consensus_seed_exchange_keypair_genesis_bytes = self
            .consensus_seed
            .unwrap()
            .genesis
            .derive_key_from_this(&CONSENSUS_SEED_EXCHANGE_KEYPAIR_DERIVE_ORDER.to_be_bytes());
        let consensus_seed_exchange_keypair_genesis =
            KeyPair::from(consensus_seed_exchange_keypair_genesis_bytes);
        trace!(
            "consensus_seed_exchange_keypair_genesis: {:?}",
            hex::encode(consensus_seed_exchange_keypair_genesis.get_pubkey())
        );

        let consensus_seed_exchange_keypair_current_bytes = self
            .consensus_seed
            .unwrap()
            .current
            .derive_key_from_this(&CONSENSUS_SEED_EXCHANGE_KEYPAIR_DERIVE_ORDER.to_be_bytes());
        let consensus_seed_exchange_keypair_current =
            KeyPair::from(consensus_seed_exchange_keypair_current_bytes);
        trace!(
            "consensus_seed_exchange_keypair_current: {:?}",
            hex::encode(consensus_seed_exchange_keypair_current.get_pubkey())
        );

        self.set_consensus_seed_exchange_keypair(
            consensus_seed_exchange_keypair_genesis,
            consensus_seed_exchange_keypair_current,
        );

        // consensus_io_exchange_keypair

        let consensus_io_exchange_keypair_genesis_bytes = self
            .consensus_seed
            .unwrap()
            .genesis
            .derive_key_from_this(&CONSENSUS_IO_EXCHANGE_KEYPAIR_DERIVE_ORDER.to_be_bytes());
        let consensus_io_exchange_keypair_genesis =
            KeyPair::from(consensus_io_exchange_keypair_genesis_bytes);
        trace!(
            "consensus_io_exchange_keypair_genesis: {:?}",
            hex::encode(consensus_io_exchange_keypair_genesis.get_pubkey())
        );

        let consensus_io_exchange_keypair_current_bytes = self
            .consensus_seed
            .unwrap()
            .current
            .derive_key_from_this(&CONSENSUS_IO_EXCHANGE_KEYPAIR_DERIVE_ORDER.to_be_bytes());
        let consensus_io_exchange_keypair_current =
            KeyPair::from(consensus_io_exchange_keypair_current_bytes);
        trace!(
            "consensus_io_exchange_keypair_current: {:?}",
            hex::encode(consensus_io_exchange_keypair_current.get_pubkey())
        );

        self.set_consensus_io_exchange_keypair(
            consensus_io_exchange_keypair_genesis,
            consensus_io_exchange_keypair_current,
        );

        // consensus_state_ikm

        let consensus_state_ikm_genesis = self
            .consensus_seed
            .unwrap()
            .genesis
            .derive_key_from_this(&CONSENSUS_STATE_IKM_DERIVE_ORDER.to_be_bytes());

        trace!(
            "consensus_state_ikm_genesis: {:?}",
            hex::encode(consensus_state_ikm_genesis.get())
        );

        let consensus_state_ikm_current = self
            .consensus_seed
            .unwrap()
            .current
            .derive_key_from_this(&CONSENSUS_STATE_IKM_DERIVE_ORDER.to_be_bytes());

        trace!(
            "consensus_state_ikm_current: {:?}",
            hex::encode(consensus_state_ikm_current.get())
        );

        self.set_consensus_state_ikm(consensus_state_ikm_genesis, consensus_state_ikm_current);

        // consensus_state_ikm

        let consensus_callback_secret_genesis = self
            .consensus_seed
            .unwrap()
            .genesis
            .derive_key_from_this(&CONSENSUS_CALLBACK_SECRET_DERIVE_ORDER.to_be_bytes());

        trace!(
            "consensus_callback_secret_genesis: {:?}",
            hex::encode(consensus_state_ikm_genesis.get())
        );

        let consensus_callback_secret_current = self
            .consensus_seed
            .unwrap()
            .current
            .derive_key_from_this(&CONSENSUS_CALLBACK_SECRET_DERIVE_ORDER.to_be_bytes());

        trace!(
            "consensus_callback_secret_current: {:?}",
            hex::encode(consensus_state_ikm_current.get())
        );

        self.set_consensus_callback_secret(
            consensus_callback_secret_genesis,
            consensus_callback_secret_current,
        );

        #[cfg(feature = "random")]
        {
            let rek =
                self.consensus_seed.unwrap().current.derive_key_from_this(
                    &RANDOMNESS_ENCRYPTION_KEY_SECRET_DERIVE_ORDER.to_be_bytes(),
                );

            let irs =
                self.consensus_seed.unwrap().current.derive_key_from_this(
                    &INITIAL_RANDOMNESS_SEED_SECRET_DERIVE_ORDER.to_be_bytes(),
                );

            self.initial_randomness_seed = Some(irs);
            self.random_encryption_key = Some(rek);

            trace!("initial_randomness_seed: {:?}", hex::encode(irs.get()));
            trace!("random_encryption_key: {:?}", hex::encode(rek.get()));

            self.write_randomness_keys();
        }

        let admin_proof_secret = self
            .consensus_seed
            .unwrap()
            .current
            .derive_key_from_this(&ADMIN_PROOF_SECRET_DERIVE_ORDER.to_be_bytes());

        self.admin_proof_secret = Some(admin_proof_secret);

        trace!(
            "admin_proof_secret: {:?}",
            hex::encode(admin_proof_secret.get())
        );

        let contract_key_proof_secret = self
            .consensus_seed
            .unwrap()
            .current
            .derive_key_from_this(&CONTRACT_KEY_PROOF_SECRET_DERIVE_ORDER.to_be_bytes());

        self.contract_key_proof_secret = Some(contract_key_proof_secret);

        trace!(
            "contract_key_proof_secret: {:?}",
            hex::encode(contract_key_proof_secret.get())
        );

        Ok(())
    }

    #[cfg(feature = "random")]
    pub fn write_randomness_keys(&self) {
        self.random_encryption_key.unwrap().seal(&REK_PATH).unwrap();
        self.initial_randomness_seed
            .unwrap()
            .seal(&IRS_PATH)
            .unwrap();
    }
}

#[cfg(feature = "test")]
pub mod tests {

    use super::{
        Keychain, CURRENT_CONSENSUS_SEED_SEALING_PATH, GENESIS_CONSENSUS_SEED_SEALING_PATH,
        /*KEY_MANAGER,*/ REGISTRATION_KEY_SEALING_PATH,
    };
    // use crate::crypto::CryptoError;
    // use crate::crypto::{KeyPair, Seed};

    // todo: fix test vectors to actually work
    fn _test_initial_keychain_state() {
        // clear previous data (if any)
        let _ = std::sgxfs::remove(&*GENESIS_CONSENSUS_SEED_SEALING_PATH);
        let _ = std::sgxfs::remove(&*CURRENT_CONSENSUS_SEED_SEALING_PATH);
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
