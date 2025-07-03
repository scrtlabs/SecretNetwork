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
use sha2::{Digest, Sha256};
use std::io::{Read, Write};
use std::sgxfs::SgxFile;
use std::sync::SgxMutex;
use tendermint::validator::Set;
use tendermint_proto::v0_38::types::ValidatorSet as RawValidatorSet;
use tendermint_proto::Protobuf;
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
pub struct KeychainMutableData {
    pub height: u64,
    pub validator_set_serialized: Vec<u8>,
    pub next_mr_enclave: Option<sgx_measurement_t>,
    pub last_evidence_seed: Option<Seed>,
}

impl KeychainMutableData {
    pub fn decode_validator_set_ex(ser: &[u8]) -> Option<Set> {
        match <Set as Protobuf<RawValidatorSet>>::decode(ser) {
            Ok(val) => Some(val),
            Err(e) => {
                error!("error decoding validator set: {:?}", e);
                None
            }
        }
    }

    pub fn decode_validator_set(&self) -> Option<Set> {
        KeychainMutableData::decode_validator_set_ex(self.validator_set_serialized.as_slice())
    }
}

pub struct Keychain {
    consensus_seed_id: u16,
    consensus_seed: Option<SeedsHolder<Seed>>,
    consensus_state_ikm: Option<SeedsHolder<AESKey>>,
    consensus_seed_exchange_keypair: Option<SeedsHolder<KeyPair>>,
    consensus_io_exchange_keypair: Option<KeyPair>,
    consensus_callback_secret: Option<AESKey>,
    pub random_encryption_key: Option<AESKey>,
    pub initial_randomness_seed: Option<AESKey>,
    registration_key: Option<KeyPair>,
    admin_proof_secret: Option<AESKey>,
    contract_key_proof_secret: Option<AESKey>,
    pub extra_data: SgxMutex<KeychainMutableData>,
}

#[derive(Clone, Copy, Default)]
pub struct SeedsHolder<T> {
    pub genesis: T,
    pub current: T,
}

lazy_static! {
    static ref SEALING_KDK: sgx_key_128bit_t = get_key_from_seed("seal.kdk".as_bytes());
    pub static ref SEALED_DATA_PATH: String = make_sgx_secret_path(&SEALED_FILE_UNITED);
    pub static ref KEY_MANAGER: Keychain = Keychain::new();
}

const KEYCHAIN_DATA_VER: u32 = 1;

#[allow(clippy::new_without_default)]
impl Keychain {
    pub fn serialize(&self, writer: &mut dyn Write) -> std::io::Result<()> {
        writer.write_all(&KEYCHAIN_DATA_VER.to_le_bytes())?;

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

        let extra = self.extra_data.lock().unwrap();

        writer.write_all(&extra.height.to_le_bytes())?;
        let val_size = extra.validator_set_serialized.len() as u64;
        writer.write_all(&val_size.to_le_bytes())?;
        writer.write_all(&extra.validator_set_serialized)?;

        let ex_flag: u8 = (if extra.next_mr_enclave.is_some() {
            1_u8
        } else {
            0_u8
        }) | (if extra.last_evidence_seed.is_some() {
            2_u8
        } else {
            0_u8
        });

        writer.write_all(&[ex_flag])?;

        if let Some(val) = extra.next_mr_enclave {
            writer.write_all(&val.m)?;
        }

        if let Some(val) = extra.last_evidence_seed {
            writer.write_all(val.as_slice())?;
        }

        Ok(())
    }

    fn read_u32(reader: &mut dyn Read) -> std::io::Result<u32> {
        let mut buf = [0u8; 4];
        reader.read_exact(&mut buf)?;
        Ok(u32::from_le_bytes(buf))
    }

    fn read_u64(reader: &mut dyn Read) -> std::io::Result<u64> {
        let mut buf = [0u8; 8];
        reader.read_exact(&mut buf)?;
        Ok(u64::from_le_bytes(buf))
    }

    pub fn deserialize(&mut self, reader: &mut dyn Read) -> std::io::Result<()> {
        let ver = Self::read_u32(reader)?;
        if KEYCHAIN_DATA_VER != ver {
            return Err(std::io::Error::new(
                std::io::ErrorKind::Other,
                "unsupported ver",
            ));
        }

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

        let mut extra = self.extra_data.lock().unwrap();

        extra.height = Self::read_u64(reader)?;

        let val_size = Self::read_u64(reader)?;

        extra.validator_set_serialized = vec![0u8; val_size as usize];
        reader.read_exact(&mut extra.validator_set_serialized)?;

        reader.read_exact(&mut flag_bytes)?;

        if (flag_bytes[0] & 1_u8) != 0 {
            let mut val = sgx_measurement_t::default();
            reader.read_exact(&mut val.m)?;
            extra.next_mr_enclave = Some(val);
        } else {
            extra.next_mr_enclave = None;
        }

        if (flag_bytes[0] & 2_u8) != 0 {
            let mut buf = Ed25519PrivateKey::default();
            reader.read_exact(buf.get_mut())?;
            extra.last_evidence_seed = Some(Seed::from(buf));
        } else {
            extra.last_evidence_seed = None;
        }

        Ok(())
    }

    pub fn save(&self) {
        let path: &str = &SEALED_DATA_PATH;
        let mut file = SgxFile::create_ex(path, &SEALING_KDK).unwrap();

        self.serialize(&mut file).unwrap();
    }

    fn load(&mut self) -> bool {
        let path: &str = &SEALED_DATA_PATH;
        match SgxFile::open_ex(path, &SEALING_KDK) {
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

    pub fn encrypt_hash_ex(seed: &Seed, hv: [u8; 32], height: u64) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(seed.as_slice());
        hasher.update(hv);
        hasher.update(height.to_le_bytes());

        let mut ret: [u8; 32] = [0_u8; 32];
        ret.copy_from_slice(&hasher.finalize());
        ret
    }

    pub fn encrypt_hash(&self, hv: [u8; 32], height: u64) -> [u8; 32] {
        Self::encrypt_hash_ex(&self.consensus_seed.unwrap().current, hv, height)
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
            //#[cfg(feature = "random")]
            initial_randomness_seed: None,
            //#[cfg(feature = "random")]
            random_encryption_key: None,
            admin_proof_secret: None,
            contract_key_proof_secret: None,
            extra_data: SgxMutex::new(KeychainMutableData {
                height: 0,
                validator_set_serialized: Vec::new(),
                next_mr_enclave: None,
                last_evidence_seed: None,
            }),
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
                trace!("Network seeds imported from legacy");
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
            let mut extra = self.extra_data.lock().unwrap();
            extra.height = res.height;
            extra.validator_set_serialized = res.validator_set;
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
            Ok(key) => {
                self.registration_key = Some(key);
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

    pub fn set_consensus_seed_exchange_keypair(&mut self, genesis: KeyPair, current: KeyPair) {
        self.consensus_seed_exchange_keypair = Some(SeedsHolder { genesis, current })
    }

    pub fn set_consensus_io_exchange_keypair(&mut self, current: KeyPair) {
        self.consensus_io_exchange_keypair = Some(current)
    }

    pub fn set_consensus_state_ikm(&mut self, genesis: AESKey, current: AESKey) {
        self.consensus_state_ikm = Some(SeedsHolder { genesis, current });
    }

    pub fn set_consensus_callback_secret(&mut self, current: AESKey) {
        self.consensus_callback_secret = Some(current);
    }

    pub fn delete_consensus_seed(&mut self) {
        self.consensus_seed = None;
    }

    pub fn set_consensus_seed(&mut self, genesis: Seed, current: Seed) {
        self.consensus_seed = Some(SeedsHolder { genesis, current });
        trace!("Consensus seeds set");
    }

    pub fn generate_consensus_ikm_key(seed: &Seed) -> AESKey {
        seed.derive_key_from_this(&CONSENSUS_STATE_IKM_DERIVE_ORDER.to_be_bytes())
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

        let consensus_seed_exchange_keypair_current_bytes = self
            .consensus_seed
            .unwrap()
            .current
            .derive_key_from_this(&CONSENSUS_SEED_EXCHANGE_KEYPAIR_DERIVE_ORDER.to_be_bytes());
        let consensus_seed_exchange_keypair_current =
            KeyPair::from(consensus_seed_exchange_keypair_current_bytes);

        self.set_consensus_seed_exchange_keypair(
            consensus_seed_exchange_keypair_genesis,
            consensus_seed_exchange_keypair_current,
        );

        // consensus_io_exchange_keypair

        let consensus_io_exchange_keypair_current_bytes = self
            .consensus_seed
            .unwrap()
            .current
            .derive_key_from_this(&CONSENSUS_IO_EXCHANGE_KEYPAIR_DERIVE_ORDER.to_be_bytes());
        let consensus_io_exchange_keypair_current =
            KeyPair::from(consensus_io_exchange_keypair_current_bytes);

        self.set_consensus_io_exchange_keypair(consensus_io_exchange_keypair_current);

        // consensus_state_ikm

        let consensus_state_ikm_genesis =
            Self::generate_consensus_ikm_key(&self.consensus_seed.unwrap().genesis);

        let consensus_state_ikm_current =
            Self::generate_consensus_ikm_key(&self.consensus_seed.unwrap().current);

        self.set_consensus_state_ikm(consensus_state_ikm_genesis, consensus_state_ikm_current);

        // consensus_state_ikm

        let consensus_callback_secret_current = self
            .consensus_seed
            .unwrap()
            .current
            .derive_key_from_this(&CONSENSUS_CALLBACK_SECRET_DERIVE_ORDER.to_be_bytes());

        self.set_consensus_callback_secret(consensus_callback_secret_current);

        //#[cfg(feature = "random")]
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
        }

        let admin_proof_secret = self
            .consensus_seed
            .unwrap()
            .current
            .derive_key_from_this(&ADMIN_PROOF_SECRET_DERIVE_ORDER.to_be_bytes());

        self.admin_proof_secret = Some(admin_proof_secret);

        let contract_key_proof_secret = self
            .consensus_seed
            .unwrap()
            .current
            .derive_key_from_this(&CONTRACT_KEY_PROOF_SECRET_DERIVE_ORDER.to_be_bytes());

        self.contract_key_proof_secret = Some(contract_key_proof_secret);

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
