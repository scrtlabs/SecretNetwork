use enclave_crypto::consts::{IO_KEY_SAVE_PATH, SEED_EXCH_KEY_SAVE_PATH};
use enclave_crypto::{KeyPair, Keychain};
use enclave_utils::storage::rewrite_on_untrusted;
use sgx_types::SgxResult;

pub fn write_public_key(kp: &KeyPair, save_path: &str) -> SgxResult<()> {
    if let Err(status) =
        rewrite_on_untrusted(base64::encode(&kp.get_pubkey()).as_bytes(), save_path)
    {
        return Err(status);
    }

    Ok(())
}

pub fn write_seed(seed: &[u8], save_path: &str) -> SgxResult<()> {
    if let Err(status) = rewrite_on_untrusted(base64::encode(seed).as_bytes(), save_path) {
        return Err(status);
    }

    Ok(())
}

pub fn write_master_pub_keys(key_manager: &Keychain) -> SgxResult<()> {
    let kp = key_manager.seed_exchange_key().unwrap();
    write_public_key(&kp.current, SEED_EXCH_KEY_SAVE_PATH)?;

    let kp = key_manager.get_consensus_io_exchange_keypair().unwrap();
    write_public_key(&kp.current, IO_KEY_SAVE_PATH)?;

    Ok(())
}
