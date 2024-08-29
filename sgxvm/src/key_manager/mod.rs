use lazy_static::lazy_static;
use sgx_tstd::ffi::OsString;
use sgx_tstd::{env, sgxfs::SgxFile};
use sgx_types::{sgx_status_t, SgxResult};
use std::io::{Read, Write};
use std::string::String;
use std::vec::Vec;

use crate::key_manager::epoch_manager::EpochManager;
use crate::key_manager::keys::{StateEncryptionKey, TransactionEncryptionKey};

pub mod consts;
pub mod epoch_manager;
pub mod keys;
pub mod utils;

pub const SEED_SIZE: usize = 32;
pub const KEYMANAGER_FILENAME: &str = ".keymanager";
pub const PUBLIC_KEY_SIZE: usize = 32;
pub const PRIVATE_KEY_SIZE: usize = 32;

lazy_static! {
    pub static ref UNSEALED_KEY_MANAGER: Option<KeyManager> = KeyManager::unseal().ok();
    pub static ref KEYMANAGER_HOME: OsString =
        env::var_os("KEYMANAGER_HOME").unwrap_or_else(get_default_keymanager_home);
}

/// Handles initialization of first node in network. This node works as 
/// attestation server, which will share epoch keys with other nodes.
/// If `reset_flag` was set to `true`, it will rewrite existing seed file
pub fn init_enclave_inner(reset_flag: i32) -> sgx_status_t {
    // Check if master key exists
    let sealed_file_exists = match KeyManager::exists() {
        Ok(exists) => exists,
        Err(err) => {
            return err;
        }
    };

    // If sealed file does not exist or reset flag was set, create key manager with random keys and seal it
    if !sealed_file_exists || reset_flag != 0 {
        // Generate random master key
        let key_manager = match KeyManager::random() {
            Ok(manager) => manager,
            Err(err) => return err,
        };

        // Seal key manager state
        match key_manager.seal() {
            Ok(_) => return sgx_status_t::SGX_SUCCESS,
            Err(err) => return err,
        };
    }

    sgx_status_t::SGX_SUCCESS
}

/// KeyManager handles keys sealing/unsealing and derivation.
pub struct KeyManager {
    epoch_manager: EpochManager,
}

impl KeyManager {
    pub fn list_epochs(&self) -> Vec<(u16, u64, Vec<u8>)> {
        self.epoch_manager.list_epochs()
    }

    pub fn get_state_key_by_epoch(&self, epoch: u16) -> Option<StateEncryptionKey> {
        match self.epoch_manager.get_epoch(epoch) {
            Some(epoch) => Some(epoch.get_state_key()),
            None => None
        }
    }

    pub fn get_state_key_by_block(&self, block_number: u64) -> Option<(u16, StateEncryptionKey)> {
        match self.epoch_manager.get_current_epoch(block_number) {
            Some(epoch) => Some((epoch.epoch_number, epoch.get_state_key())),
            None => None
        }
    }

    pub fn get_tx_key_by_epoch(&self, epoch: u16) -> Option<TransactionEncryptionKey> {
        match self.epoch_manager.get_epoch(epoch) {
            Some(epoch) => Some(epoch.get_tx_key()),
            None => None
        }
    }

    pub fn get_tx_key_by_block(&self, block_number: u64) -> Option<(u16, TransactionEncryptionKey)> {
        match self.epoch_manager.get_current_epoch(block_number) {
            Some(epoch) => Some((epoch.epoch_number, epoch.get_tx_key())),
            None => None
        }
    }

    /// Checks if file with sealed master key exists
    pub fn exists() -> SgxResult<bool> {
        match SgxFile::open(format!("{}/{}", KEYMANAGER_HOME.to_str().unwrap(), KEYMANAGER_FILENAME)) {
            Ok(_) => Ok(true),
            Err(ref err) if err.kind() == std::io::ErrorKind::NotFound => Ok(false),
            Err(err) => {
                println!(
                    "[KeyManager] Cannot check if sealed file exists. Reason: {:?}",
                    err
                );
                Err(sgx_status_t::SGX_ERROR_UNEXPECTED)
            }
        }
    }

    /// Seals Key Manager to protected file, so it will be accessible only for enclave.
    /// For now, enclaves with same MRSIGNER will be able to recover that file, but
    /// we'll use MRENCLAVE since Upgradeability Protocol will be implemented
    pub fn seal(&self) -> SgxResult<()> {
        let mut sealed_file = KeyManager::create_sealed_file()?;

        let encoded = self.epoch_manager.serialize()?;
        if let Err(err) = sealed_file.write(encoded.as_bytes()) {
            println!(
                "[KeyManager] Cannot write serialized epoch manager. Reason: {:?}",
                err
            );
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }

        Ok(())
    }

    /// Unseals key manager from protected file. If file was not found or unaccessible,
    /// will return SGX_ERROR_UNEXPECTED
    pub fn unseal() -> SgxResult<Self> {
        // Unseal file with key manager
        let sealed_file_path = format!("{}/{}", KEYMANAGER_HOME.to_str().unwrap(), KEYMANAGER_FILENAME);
        let mut sealed_file = SgxFile::open(sealed_file_path).map_err(|err| {
            println!(
                "[KeyManager] Cannot open file with key manager. Reason: {:?}",
                err
            );
            sgx_status_t::SGX_ERROR_UNEXPECTED
        })?;

        let mut sealed_file_content: Vec<u8> = Vec::default();
        let sealed_file_content_len = sealed_file.read_to_end(&mut sealed_file_content)
            .map_err(|err| {
                println!("[KeyManager] Cannot read sealed file. Reason: {:?}", err);
                sgx_status_t::SGX_ERROR_UNEXPECTED
            })?;

        if sealed_file_content_len < SEED_SIZE {
            println!("[KeyManager] Corrupted sealed file. Invalid len. Expected >= 32, Got: {:?}", sealed_file_content_len);
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }

        let epoch_manager = match sealed_file_content_len {
            SEED_SIZE => {
                // Read from plain seed
                let legacy_seed: [u8; SEED_SIZE] = sealed_file_content.try_into().map_err(|err| {
                    println!("[KeyManager] Cannot convert legacy seed to appropriate format. Reason: {:?}", err);
                    sgx_status_t::SGX_ERROR_UNEXPECTED
                })?;
                EpochManager::from_seed(legacy_seed)
            },
            _ => {
                // Deserialize epoch manager
                let serialized_epoch_manager = String::from_utf8(sealed_file_content).map_err(|err| {
                    println!(
                        "[KeyManager] Cannot read serialized epoch manager. Reason: {:?}",
                        err
                    );
                    sgx_status_t::SGX_ERROR_UNEXPECTED
                })?;
                EpochManager::deserialize(&serialized_epoch_manager)?
            }
        };

        Ok(Self {
            epoch_manager,
        })
    }

    /// Creates new KeyManager with signle random epoch key
    pub fn random() -> SgxResult<Self> {
        let random_epoch_manager = EpochManager::random_with_single_epoch()?;

        Ok(Self {
            epoch_manager: random_epoch_manager,
        })
    }

    #[cfg(feature = "attestation_server")]
    /// Creates new epoch with provided starting block
    pub fn add_new_epoch(&self, starting_block: u64) -> SgxResult<()> {
        let updated_epoch_manager = self.epoch_manager.add_new_epoch(starting_block)?;
        let serialized_epoch_manager = updated_epoch_manager.serialize()?;

        let mut sealed_file = KeyManager::create_sealed_file()?;

        if let Err(err) = sealed_file.write(serialized_epoch_manager.as_bytes()) {
            println!(
                "[KeyManager] Cannot write serialized epoch manager. Reason: {:?}",
                err
            );
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }

        Ok(())
    }

    #[cfg(feature = "attestation_server")]
    pub fn remove_latest_epoch(&self) -> SgxResult<()> {
        let updated_epoch_manager = self.epoch_manager.remove_latest_epoch()?;
        let serialized_epoch_manager = updated_epoch_manager.serialize()?;

        let mut sealed_file = KeyManager::create_sealed_file()?;

        if let Err(err) = sealed_file.write(serialized_epoch_manager.as_bytes()) {
            println!(
                "[KeyManager] Cannot write serialized epoch manager. Reason: {:?}",
                err
            );
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }

        Ok(())
    }

    #[cfg(feature = "attestation_server")]
    /// Encrypts epoch data using shared key
    pub fn encrypt_epoch_data(
        &self,
        reg_key: &keys::RegistrationKey,
        public_key: Vec<u8>,
    ) -> SgxResult<Vec<u8>> {
        self.epoch_manager.encrypt(reg_key, public_key)
    }

    /// Recovers encrypted epoch data, obtained from attestation server
    pub fn decrypt_epoch_data(
        reg_key: &keys::RegistrationKey,
        public_key: Vec<u8>,
        encrypted_epoch_data: Vec<u8>,
    ) -> SgxResult<Self> {
        let epoch_manager = EpochManager::decrypt(reg_key, public_key, encrypted_epoch_data)?;

        Ok(Self {
            epoch_manager,
        })
    }

    /// Return x25519 public key for transaction encryption. Public key is based on block number
    pub fn get_public_key(&self, block_number: u64) -> SgxResult<Vec<u8>> {
        match self.epoch_manager.get_current_epoch(block_number) {
            Some(epoch) => Ok(epoch.get_tx_key().public_key()),
            None => {
                println!("[KeyManager] Cannot obtain tx encryption key");
                Err(sgx_status_t::SGX_ERROR_UNEXPECTED)
            }
        }
    }

    fn create_sealed_file() -> SgxResult<SgxFile> {
        let keymanager_home_path = match KEYMANAGER_HOME.to_str() {
            Some(path) => path,
            None => {
                println!("[KeyManager] Cannot get KEYMANAGER_HOME env");
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }
        };

        let sealed_file_path = format!("{}/{}", keymanager_home_path, KEYMANAGER_FILENAME);
        println!(
            "[KeyManager] Creating file for key manager. Location: {:?}",
            sealed_file_path
        );
        let sealed_file = SgxFile::create(sealed_file_path).map_err(|err| {
            println!(
                "[KeyManager] Cannot create file for key manager. Reason: {:?}",
                err
            );
            sgx_status_t::SGX_ERROR_UNEXPECTED
        })?;
        println!("[KeyManager] File created");

        Ok(sealed_file)
    }
}

/// Tries to return path to $HOME/.swisstronik-enclave directory.
/// If it cannot find home directory, panics with error
fn get_default_keymanager_home() -> OsString {
    let home_dir = env::home_dir().expect("[KeyManager] Cannot find home directory");
    let default_keymanager_home = home_dir
        .to_str()
        .expect("[KeyManager] Cannot decode home directory path");
    OsString::from(format!("{}/.swisstronik-enclave", default_keymanager_home))
}
