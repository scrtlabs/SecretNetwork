use crate::crypto::keys::*;
struct Keychain {
    seed: [u8; SEED_SIZE],
    master_state_key: StateKey,
    io_key: KeyPair,
}

static key_manager: Option(Keychain) = None;

impl Keychain {
    pub fn get() -> Result<Keychain, CryptoError> {
        match key_manager {
            Some(x) => x,
            None => {
                /**
                 * TODO:
                 * 1. unseal `seed`
                 * 2. CSPRNG.init(seed)
                 * 3. CSPRNG.next(256 bits) => master_state_key
                 * 4. CSPRNG.next(256 bits) => sk_io
                 **/
                let seed = [1_u8; SEED_SIZE];
                let master_state_key = [2_u8; SYMMETRIC_KEY_SIZE];
                let io_key = [3_u8; SECRET_KEY_SIZE];

                key_manager = Some(Keychain {
                    seed: seed,
                    master_state_key: master_state_key,
                    io_key: KeyPair::new_from_slice(&io_key)?, // TODO handle the error in here in order to not need to handle it outside
                });
                key_manager
            }
        }
    }
}
