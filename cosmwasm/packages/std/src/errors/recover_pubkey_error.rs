use schemars::JsonSchema;
use snafu::Snafu;
use serde::{Deserialize, Serialize};


#[derive(Debug, Serialize, Deserialize, Snafu, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum RecoverPubkeyError {
    #[snafu(display("Invalid hash format"))]
    InvalidHashFormat,
    #[snafu(display("Invalid signature format"))]
    InvalidSignatureFormat,
    #[snafu(display("Invalid recovery parameter. Supported values: 0 and 1."))]
    InvalidRecoveryParam,
    #[snafu(display("Unknown error: {}", error_code))]
    UnknownErr {
        error_code: u32,
        #[serde(skip)]
        backtrace: Option<snafu::Backtrace>,
    },
}

impl RecoverPubkeyError {
    pub fn unknown_err(error_code: u32) -> Self {
        UnknownErr {
            error_code,
        }.build()
    }
}

impl PartialEq<RecoverPubkeyError> for RecoverPubkeyError {
    fn eq(&self, rhs: &RecoverPubkeyError) -> bool {
        match self {
            RecoverPubkeyError::InvalidHashFormat => {
                matches!(rhs, RecoverPubkeyError::InvalidHashFormat)
            }
            RecoverPubkeyError::InvalidSignatureFormat => {
                matches!(rhs, RecoverPubkeyError::InvalidSignatureFormat)
            }
            RecoverPubkeyError::InvalidRecoveryParam => {
                matches!(rhs, RecoverPubkeyError::InvalidRecoveryParam)
            }
            RecoverPubkeyError::UnknownErr { error_code, .. } => {
                if let RecoverPubkeyError::UnknownErr {
                    error_code: rhs_error_code,
                    ..
                } = rhs
                {
                    error_code == rhs_error_code
                } else {
                    false
                }
            }
        }
    }
}
