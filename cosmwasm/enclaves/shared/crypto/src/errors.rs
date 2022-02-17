use derive_more::Display;

#[derive(Debug, Display)]
pub enum CryptoError {
    /// The ECDH process failed.
    DerivingKeyError,
    /// A key was missing.
    MissingKeyError,
    /// The symmetric decryption has failed for some reason.
    DecryptionError,
    /// The ciphertext provided was improper.
    /// e.g. MAC wasn't valid, missing IV etc.
    ImproperEncryption,
    /// The symmetric encryption has failed for some reason.
    EncryptionError,
    /// The signing process has failed for some reason.
    SigningError,
    /// The signature couldn't be parsed correctly.
    ParsingError,
    /// The public key can't be recovered from a message & signature.
    RecoveryError,
    /// A key wasn't valid.
    /// e.g. PrivateKey, PublicKey, SharedSecret.
    KeyError,
    /// The random function had failed generating randomness
    RandomError,
    /// An error related to signature verification
    VerificationError,
}
