use enclave_crypto::consts::{IO_KEY_SAVE_PATH, SEED_EXCH_KEY_SAVE_PATH};
use enclave_crypto::KeyPair;
use enclave_utils::storage::rewrite_on_untrusted;
use enclave_utils::Keychain;
use sgx_types::SgxResult;

pub fn write_public_key(kp: &KeyPair, save_path: &str) -> SgxResult<()> {
    rewrite_on_untrusted(base64::encode(kp.get_pubkey()).as_bytes(), save_path)
}

pub fn write_master_pub_keys(key_manager: &Keychain) -> SgxResult<()> {
    let kp = key_manager.seed_exchange_key().unwrap();
    write_public_key(kp, SEED_EXCH_KEY_SAVE_PATH)?;

    let kp = key_manager.get_consensus_io_exchange_keypair().unwrap();
    write_public_key(&kp, IO_KEY_SAVE_PATH)?;

    Ok(())
}
