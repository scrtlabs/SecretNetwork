use crate::consts;
use crate::crypto::keys::{KeyPair, Seed};
use crate::crypto::traits::*;
use enclave_ffi_types::CryptoError;
use lazy_static::lazy_static;
use log::*;

pub struct Keychain {
    seed: Seed,
    master_state_key: Seed,
    io_key: KeyPair,
}

lazy_static! {
    pub static ref KEY_MANAGER: Result<Keychain, CryptoError> = {
        let seed = Seed::unseal(consts::SEED_SEALING_PATH).map_err(|err| {
            error!("[Enclave] Error unsealing the seed: {:?}", err);
            CryptoError::ParsingError /* change error type? */
        })?;

        let io_key_bytes = seed.derive_key_from_this(consts::IO_KEY_DERIVE_ORDER);
        let io_key = KeyPair::new_from_slice(&io_key_bytes).map_err(|err| {
            error!("[Enclave] Error creating io_key: {:?}", err);
            CryptoError::ParsingError /* change error type? */
        })?;

        let master_state_key_bytes = seed.derive_key_from_this(consts::STATE_MASTER_KEY_DERIVE_ORDER);
        let master_state_key = Seed::new_from_slice(&io_key_bytes);

        Ok(Keychain {
            seed,
            master_state_key,
            io_key,
        })
    };
}

impl Keychain {
    pub fn get_master_state_key(&self) -> &Seed {
        &self.master_state_key
    }
}
