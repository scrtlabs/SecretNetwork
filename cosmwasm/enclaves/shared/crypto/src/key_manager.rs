use crate::consts::*;
use crate::traits::{Kdf, SealedKey};
use crate::CryptoError;
use crate::{AESKey, KeyPair, Seed};
use enclave_ffi_types::EnclaveError;
use lazy_static::lazy_static;
use log::*;
use sgx_types::c_int;
use std::net::SocketAddr;
use std::os::unix::io::IntoRawFd;
use std::{
    io::{Read, Write},
    net::TcpStream,
    str,
    string::String,
    sync::Arc,
};

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
    registration_key: Option<KeyPair>,
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
        let consensus_seed: Option<SeedsHolder<Seed>> = match (
            Seed::unseal(&GENESIS_CONSENSUS_SEED_SEALING_PATH.as_str()),
            Seed::unseal(&CURRENT_CONSENSUS_SEED_SEALING_PATH.as_str()),
        ) {
            (Ok(genesis), Ok(current)) => Some(SeedsHolder { genesis, current }),
            _ => None,
        };

        let registration_key = Self::unseal_registration_key();

        let mut x = Keychain {
            consensus_seed_id: 1,
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

    fn unseal_registration_key() -> Option<KeyPair> {
        match KeyPair::unseal(&REGISTRATION_KEY_SEALING_PATH.as_str()) {
            Ok(k) => Some(k),
            _ => None,
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

    pub fn reseal_registration_key(&mut self) -> Result<(), EnclaveError> {
        match Self::unseal_registration_key() {
            Some(kp) => {
                if let Err(_e) = std::sgxfs::remove(&*REGISTRATION_KEY_SEALING_PATH) {
                    error!("Failed to reseal registration key - error code 0xC11");
                    return Err(EnclaveError::FailedSeal);
                };
                if let Err(_e) = kp.seal(&REGISTRATION_KEY_SEALING_PATH.as_str()) {
                    error!("Failed to reseal registration key - error code 0xC12");
                    return Err(EnclaveError::FailedSeal);
                }
                Ok(())
            }
            None => Ok(()),
        }
    }

    pub fn set_registration_key(&mut self, kp: KeyPair) -> Result<(), EnclaveError> {
        if let Err(e) = kp.seal(&REGISTRATION_KEY_SEALING_PATH.as_str()) {
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
        debug!(
            "Sealing genesis consensus seed in {}",
            *GENESIS_CONSENSUS_SEED_SEALING_PATH
        );
        if let Err(e) = genesis.seal(&GENESIS_CONSENSUS_SEED_SEALING_PATH.as_str()) {
            error!("Error sealing genesis consensus_seed - error code 0xC14");
            return Err(e);
        }

        debug!(
            "Sealing current consensus seed in {}",
            *CURRENT_CONSENSUS_SEED_SEALING_PATH
        );
        if let Err(e) = current.seal(&CURRENT_CONSENSUS_SEED_SEALING_PATH.as_str()) {
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

        Ok(())
    }

    pub fn make_client_config() -> rustls::ClientConfig {
        let mut config = rustls::ClientConfig::new();

        config
            .root_store
            .add_server_trust_anchors(&webpki_roots::TLS_SERVER_ROOTS);

        config
    }

    fn create_socket_to_service(host_name: &str) -> Result<c_int, CryptoError> {
        use std::net::ToSocketAddrs;

        let mut addr: Option<SocketAddr> = None;

        let addrs = (host_name, 3000).to_socket_addrs().map_err(|err| {
            trace!("Error while trying to convert to socket addrs {:?}", err);
            CryptoError::SocketCreationError
        })?;

        for a in addrs {
            if let SocketAddr::V4(_) = a {
                addr = Some(a);
            }
        }

        if addr.is_none() {
            trace!("Failed to resolve the IPv4 address of the service");
            return Err(CryptoError::IPv4LookupError);
        }

        let sock = TcpStream::connect(&addr.unwrap()).map_err(|err| {
            trace!(
                "Error while trying to connect to service with addr: {:?}, err: {:?}",
                addr,
                err
            );
            CryptoError::SocketCreationError
        })?;

        return Ok(sock.into_raw_fd());
    }

    fn get_challange_from_service(fd: c_int, host_name: &str) -> Result<Vec<u8>, CryptoError> {
        pub const CHALLANGE_ENDPOINT: &str = "/authenticate";

        let req = format!("GET {} HTTP/1.1\r\nHOST: {}\r\nContent-Length:{}\r\nContent-Type: application/json\r\nConnection: close\r\n\r\n{}",
        CHALLANGE_ENDPOINT,
        host_name,
        encoded_json.len(),
        encoded_json);

        trace!("{}", req);
        let config = Keychain::make_client_config();
        let dns_name = webpki::DNSNameRef::try_from_ascii_str(host_name).unwrap();
        let mut sess = rustls::ClientSession::new(&Arc::new(config), dns_name);
        let mut sock = TcpStream::new(fd).unwrap();
        let mut tls = rustls::Stream::new(&mut sess, &mut sock);

        let _result = tls.write(req.as_bytes());
        let mut plaintext = Vec::new();

        info!("write complete");

        tls.read_to_end(&mut plaintext).unwrap();
        info!("read_to_end complete");
        let resp_string = String::from_utf8(plaintext.clone()).unwrap();
        info!("resp string {}", resp_string);

        Ok(plaintext)
    }

    fn try_get_consensus_seed_from_service(id: u16) -> Result<Seed, CryptoError> {
        #[cfg(feature = "production")]
        pub const SEED_SERVICE_DNS: &'static str = "sss.scrtlabs.com";
        #[cfg(not(feature = "production"))]
        pub const SEED_SERVICE_DNS: &'static str = "sssd.scrtlabs.com";
        let socket = Keychain::create_socket_to_service(SEED_SERVICE_DNS)?;
        Keychain::get_challange_from_service(socket, SEED_SERVICE_DNS);
        let s: [u8; 32] = [
            0, id as u8, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21,
            22, 23, 24, 25, 26, 27, 28, 29, 30, 31,
        ];
        let mut seed = Seed::default();
        seed.as_mut().copy_from_slice(&s);
        Ok(seed)
    }

    // Retreiving consensus seed from SingularitySeedService
    // id - The desired seed id
    // retries - The amount of times to retry upon failure. 0 means infinite
    pub fn get_next_consensus_seed_from_service(
        &mut self,
        retries: u8,
        genesis_seed: Seed,
    ) -> Result<Seed, CryptoError> {
        let mut opt_seed: Result<Seed, CryptoError> = Err(CryptoError::DecryptionError);

        match retries {
            0 => {
                trace!("Looping consensus seed lookup forever");
                loop {
                    if let Ok(seed) =
                        Keychain::try_get_consensus_seed_from_service(self.consensus_seed_id)
                    {
                        opt_seed = Ok(seed);
                        break;
                    }
                }
            }
            _ => {
                for try_id in 1..retries + 1 {
                    trace!("Looping consensus seed lookup {}/{}", try_id, retries);
                    match Keychain::try_get_consensus_seed_from_service(self.consensus_seed_id) {
                        Ok(seed) => {
                            opt_seed = Ok(seed);
                            break;
                        }
                        Err(e) => opt_seed = Err(e),
                    }
                }
            }
        };

        if let Err(e) = opt_seed {
            return Err(e);
        }

        let mut seed = opt_seed?;
        trace!(
            "LIORRR Genesis seed is {:?} service seed is {:?}",
            genesis_seed.as_slice(),
            seed.as_slice()
        );

        // XOR the seed with the genesis seed
        let mut seed_vec = seed.as_mut().to_vec();
        seed_vec
            .iter_mut()
            .zip(genesis_seed.as_slice().to_vec().iter())
            .for_each(|(x1, x2)| *x1 ^= *x2);

        seed.as_mut().copy_from_slice(seed_vec.as_slice());

        trace!("LIORRR New seed is {:?}", seed.as_slice());

        trace!("Successfully fetched consensus seed from service");
        self.consensus_seed_id += 1;
        Ok(seed)
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
        let _ = std::sgxfs::remove(&*GENESIS_CONSENSUS_SEED_SEALING_PATH);
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
