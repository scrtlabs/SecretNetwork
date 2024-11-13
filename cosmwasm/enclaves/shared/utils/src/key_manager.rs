use crate::storage::get_key_from_seed;
use crate::validator_set::ValidatorSetForHeight;
use core::default::{self, default};
use enclave_crypto::consts::*;
use enclave_crypto::ed25519::Ed25519PrivateKey;
use enclave_crypto::traits::{Kdf, SealedKey};
use enclave_crypto::CryptoError;
use enclave_crypto::{AESKey, KeyPair, Seed};
use enclave_ffi_types::EnclaveError;
use lazy_static::lazy_static;
use log::*;
use sgx_types::{sgx_key_128bit_t, sgx_measurement_t};
use std::io::{Read, Write};
use std::sgxfs::SgxFile;
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
    validator_set_for_height: ValidatorSetForHeight,
    pub next_mr_enclave: Option<sgx_measurement_t>,
}

#[derive(Clone, Copy, Default)]
pub struct SeedsHolder<T> {
    pub genesis: T,
    pub current: T,
}

lazy_static! {
    pub static ref SEALED_DATA_PATH: String = make_sgx_secret_path(SEALED_FILE_UNITED);
    pub static ref SEALING_KDK: sgx_key_128bit_t = get_key_from_seed("seal.kdk".as_bytes());
    pub static ref KEY_MANAGER: Keychain = Keychain::new();
}

#[allow(clippy::new_without_default)]
impl Keychain {
    fn serialize(&self, writer: &mut dyn Write) -> std::io::Result<()> {
        if let Some(seeds) = self.consensus_seed {
            writer.write_all(&[1_u8])?;
            writer.write_all(seeds.genesis.as_slice())?;
            writer.write_all(seeds.current.as_slice())?;
        } else {
            writer.write_all(&[0_u8])?;
        }

        if let Some(kp) = self.registration_key {
            writer.write_all(&[1_u8])?;
            writer.write_all(kp.get_privkey())?;
        } else {
            writer.write_all(&[0_u8])?;
        }

        writer.write_all(&self.validator_set_for_height.height.to_le_bytes())?;
        let val_size = self.validator_set_for_height.validator_set.len() as u64;
        writer.write_all(&val_size.to_le_bytes())?;
        writer.write_all(&self.validator_set_for_height.validator_set)?;

        if let Some(val) = self.next_mr_enclave {
            writer.write_all(&[1_u8])?;
            writer.write_all(&val.m)?;
        } else {
            writer.write_all(&[0_u8])?;
        }

        Ok(())
    }

    fn deserialize(&mut self, reader: &mut dyn Read) -> std::io::Result<()> {
        let mut flag_bytes = [0u8; 1];

        reader.read_exact(&mut flag_bytes)?;
        if flag_bytes[0] != 0 {
            let mut buf = Ed25519PrivateKey::default();
            reader.read_exact(buf.get_mut())?;
            let genesis = Seed::from(buf);

            reader.read_exact(buf.get_mut())?;
            let current = Seed::from(buf);

            self.consensus_seed = Some(SeedsHolder { genesis, current });
        }

        reader.read_exact(&mut flag_bytes)?;
        if flag_bytes[0] != 0 {
            let mut sk = Ed25519PrivateKey::default();
            reader.read_exact(sk.get_mut())?;

            self.registration_key = Some(KeyPair::from_sk(sk));
        }

        let mut buf_u64 = [0u8; 8];
        reader.read_exact(&mut buf_u64)?;
        self.validator_set_for_height.height = u64::from_le_bytes(buf_u64);

        reader.read_exact(&mut buf_u64)?;
        let val_size = u64::from_le_bytes(buf_u64);

        self.validator_set_for_height.validator_set = vec![0u8; val_size as usize];
        reader.read_exact(&mut self.validator_set_for_height.validator_set)?;

        reader.read_exact(&mut flag_bytes)?;
        if flag_bytes[0] != 0 {
            let mut val = sgx_measurement_t::default();
            reader.read_exact(&mut val.m)?;
            self.next_mr_enclave = Some(val);
        }

        Ok(())
    }

    pub fn save(&self) {
        let path: &str = &SEALED_DATA_PATH;
        let mut file = SgxFile::create_ex(path, &SEALING_KDK).unwrap();

        self.serialize(&mut file).unwrap();
    }

    fn load_ex(&mut self, key: &sgx_key_128bit_t) -> bool {
        let path: &str = &SEALED_DATA_PATH;
        match SgxFile::open_ex(path, key) {
            Ok(mut file) => {
                println!("Sealed data opened");
                self.deserialize(&mut file).unwrap();
                true
            }
            Err(err) => {
                println!("Seailed data can't be opened: {}", err);
                false
            }
        }
    }

    fn load(&mut self) {
        self.load_ex(&SEALING_KDK);
    }

    pub fn get_migration_keys() -> KeyPair {
        let mut sk = Ed25519PrivateKey::default();
        sk.get_mut()[..16].copy_from_slice(&get_key_from_seed("migrate.0.kdk".as_bytes()));
        sk.get_mut()[16..].copy_from_slice(&get_key_from_seed("migrate.1.kdk".as_bytes()));

        KeyPair::from_sk(sk)
    }

    pub fn new_empty() -> Self {
        Keychain {
            consensus_seed_id: CONSENSUS_SEED_VERSION,
            consensus_seed: None,
            registration_key: None,
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
            validator_set_for_height: ValidatorSetForHeight {
                height: 0,
                validator_set: Vec::new(),
            },
            next_mr_enclave: None,
        }
    }

    fn load_legacy_keys(&mut self) {
        self.registration_key =
            match KeyPair::unseal(&make_sgx_secret_path(SEALED_FILE_REGISTRATION_KEY)) {
                Ok(k) => Some(k),
                _ => None,
            };

        self.consensus_seed = match (
            Seed::unseal(&make_sgx_secret_path(
                SEALED_FILE_ENCRYPTED_SEED_KEY_GENESIS,
            )),
            Seed::unseal(&make_sgx_secret_path(
                SEALED_FILE_ENCRYPTED_SEED_KEY_CURRENT,
            )),
        ) {
            (Ok(genesis), Ok(current)) => {
                trace!(
                    "New keychain created with the following seeds {:?}, {:?}",
                    hex::encode(genesis.as_slice()),
                    hex::encode(current.as_slice())
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

        if let Ok(res) =
            ValidatorSetForHeight::unseal_from(&make_sgx_secret_path(SEALED_FILE_VALIDATOR_SET))
        {
            self.validator_set_for_height = res;
        }
    }

    pub fn new() -> Self {
        let mut x = Self::new_empty();
        x.load();
        let _ = x.generate_consensus_master_keys();
        x
    }

    pub fn new_from_legacy() -> Option<Self> {
        let mut x = Self::new_empty();
        x.load_legacy_keys();

        if x.registration_key.is_some() || x.consensus_seed.is_some() {
            let _ = x.generate_consensus_master_keys();
            Some(x)
        } else {
            None
        }
    }

    pub fn new_from_prev(key: &sgx_key_128bit_t) -> Option<Self> {
        let mut x = Self::new_empty();
        if x.load_ex(key) {
            let _ = x.generate_consensus_master_keys();
            Some(x)
        } else {
            None
        }
    }

    pub fn get_validator_set_for_height() -> ValidatorSetForHeight {
        // always re-read it
        Keychain::new().validator_set_for_height
    }

    pub fn set_validator_set_for_height(new_set: ValidatorSetForHeight) {
        // TODO: don't re-read it, use data from KEY_MANAGER
        let mut key_manager = Keychain::new();
        key_manager.validator_set_for_height = new_set;
        key_manager.save();
    }

    pub fn create_consensus_seed(&mut self) -> Result<(), CryptoError> {
        match (Seed::new(), Seed::new()) {
            (Ok(genesis), Ok(current)) => {
                self.set_consensus_seed(genesis, current);
            }
            (Err(err), _) => return Err(err),
            (_, Err(err)) => return Err(err),
        };
        Ok(())
    }

    pub fn create_registration_key(&mut self) -> Result<(), CryptoError> {
        match KeyPair::new() {
            Ok(key) => self.set_registration_key(key),
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

    pub fn set_registration_key(&mut self, kp: KeyPair) {
        self.registration_key = Some(kp);
        self.save();
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

    pub fn delete_consensus_seed(&mut self) {
        self.consensus_seed = None;
        self.save();
    }

    pub fn set_consensus_seed(&mut self, genesis: Seed, current: Seed) {
        trace!(
            "Consensus seeds were set to be the following {:?}, {:?}",
            genesis.as_slice(),
            current.as_slice()
        );
        self.consensus_seed = Some(SeedsHolder { genesis, current });
        self.save();
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
}

#[cfg(feature = "test")]
pub mod tests {

    use super::{
        Keychain, SEALED_DATA_PATH,
        /*KEY_MANAGER, REGISTRATION_KEY_SEALING_PATH,*/
    };
    // use crate::crypto::CryptoError;
    // use crate::crypto::{KeyPair, Seed};

    // todo: fix test vectors to actually work
    fn _test_initial_keychain_state() {
        // clear previous data (if any)
        let _ = std::sgxfs::remove(&*SEALED_DATA_PATH);
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
