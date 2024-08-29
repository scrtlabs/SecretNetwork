use std::string::String;
use std::string::ToString;
use thiserror_no_std::Error;

#[derive(Error, Debug, Clone)]
#[allow(dead_code)]
pub enum RustError {
    #[error("Cannot decode UTF8 bytes into string: {}", msg)]
    InvalidUtf8 { msg: String },
    #[error("Encryption error: {}", msg)]
    EncryptionError { msg: String },
    #[error("Decryption error: {}", msg)]
    DecryptionError { msg: String },
    #[error("Enclave error: {}", msg)]
    EnclaveError { msg: String },
    #[error("Cannot perform ECDH: {}", msg)]
    ECDHError { msg: String },
    #[error("Cannot decode protobuf: {}", msg)]
    ProtobufError { msg: String },
}

impl RustError {
    pub fn invalid_utf8<S: ToString>(msg: S) -> Self {
        RustError::InvalidUtf8 {
            msg: msg.to_string(),
        }
    }

    pub fn encryption_err<S: ToString>(msg: S) -> Self {
        RustError::EncryptionError {
            msg: msg.to_string(),
        }
    }

    pub fn decryption_err<S: ToString>(msg: S) -> Self {
        RustError::DecryptionError {
            msg: msg.to_string(),
        }
    }

    pub fn enclave_err<S: ToString>(msg: S) -> Self {
        RustError::EnclaveError {
            msg: msg.to_string(),
        }
    }

    pub fn ecdh_err<S: ToString>(msg: S) -> Self {
        RustError::ECDHError {
            msg: msg.to_string(),
        }
    }

    pub fn protobuf_error<S: ToString>(msg: S) -> Self {
        RustError::ProtobufError {
            msg: msg.to_string(),
        }
    }
}

impl From<std::str::Utf8Error> for RustError {
    fn from(source: std::str::Utf8Error) -> Self {
        RustError::invalid_utf8(source)
    }
}

impl From<std::string::FromUtf8Error> for RustError {
    fn from(source: std::string::FromUtf8Error) -> Self {
        RustError::invalid_utf8(source)
    }
}

impl From<protobuf::ProtobufError> for RustError {
    fn from(source: protobuf::ProtobufError) -> Self {
        RustError::protobuf_error(source)
    }
}
