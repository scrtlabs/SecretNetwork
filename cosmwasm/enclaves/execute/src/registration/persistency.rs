use super::attestation::create_attestation_certificate;
use enclave_crypto::consts::{
    IO_CERT_SAVE_PATH, IO_KEY_SAVE_PATH, SEED_EXCH_CERT_SAVE_PATH, SEED_EXCH_KEY_SAVE_PATH,
    SIGNATURE_TYPE,
};
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

pub fn write_cert(cert: &[u8], save_path: &str) -> SgxResult<()> {
    if let Err(status) = rewrite_on_untrusted(cert, save_path) {
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

pub fn write_master_pub_keys(key_manager: &Keychain, api_key: &[u8]) -> SgxResult<()> {
    let kp = key_manager.seed_exchange_key().unwrap();
    write_public_key(&kp.current, SEED_EXCH_KEY_SAVE_PATH)?;
    write_cert_for_public_key(&kp.genesis, SEED_EXCH_CERT_SAVE_PATH, api_key_slice)?;

    let kp = key_manager.get_consensus_io_exchange_keypair().unwrap();
    write_public_key(&kp.current, IO_KEY_SAVE_PATH)?;
    write_cert_for_public_key(&kp.genesis, SEED_EXCH_CERT_SAVE_PATH, api_key_slice)?;

    Ok(())
}

pub fn write_cert_for_public_key(kp: &KeyPair, save_path: &str, api_key: &[u8]) -> SgxResult<()> {
    let (_, cert) = match create_attestation_certificate(kp, SIGNATURE_TYPE, api_key, None) {
        Err(e) => {
            error!("Error in create_attestation_certificate: {:?}", e);
            return Err(e);
        }
        Ok(res) => res,
    };

    if let Err(status) = write_cert(cert.as_slice(), save_path) {
        return Err(status);
    }

    Ok(())
}
