use derive_more::Display;

#[derive(Debug, Display)]
pub enum CryptoError {
    /// The ECDH process failed.
    DerivingKeyError = 1,
    /// A key was missing.
    MissingKeyError = 2,
    /// The symmetric decryption has failed for some reason.
    DecryptionError = 3,
    /// The ciphertext provided was improper.
    /// e.g. MAC wasn't valid, missing IV etc.
    ImproperEncryption = 4,
    /// The symmetric encryption has failed for some reason.
    EncryptionError = 5,
    /// The signing process has failed for some reason.
    SigningError = 6,
    /// The signature couldn't be parsed correctly.
    ParsingError = 7,
    /// The public key can't be recovered from a message & signature.
    RecoveryError = 8,
    /// A key wasn't valid.
    /// e.g. PrivateKey, PublicKey, SharedSecret.
    KeyError = 9,
    /// The random function had failed generating randomness
    RandomError = 10,
    /// An error related to signature verification
    VerificationError = 11,
    SocketCreationError = 12,
    IPv4LookupError = 13,
}

#[derive(Debug, Display)]
pub enum WasmApiCryptoError {
    InvalidHashFormat = 3,
    InvalidSignatureFormat = 4,
    InvalidPubkeyFormat = 5,
    InvalidRecoveryParam = 6,
    BatchErr = 7,
    GenericErr = 10,
    InvalidPrivateKeyFormat = 1000, // Assaf: 1000 to not collide with CosmWasm someday
}
