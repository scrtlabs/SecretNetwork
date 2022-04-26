use schemars::JsonSchema;
use snafu::Snafu;
use serde::{Deserialize, Serialize};


#[derive(Debug, Serialize, Deserialize, Snafu, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum VerificationError {
    #[snafu(display("Batch error"))]
    BatchErr,
    #[snafu(display("Generic error"))]
    GenericErr,
    #[snafu(display("Invalid hash format"))]
    InvalidHashFormat,
    #[snafu(display("Invalid signature format"))]
    InvalidSignatureFormat,
    #[snafu(display("Invalid public key format"))]
    InvalidPubkeyFormat,
    #[snafu(display("Invalid recovery parameter. Supported values: 0 and 1."))]
    InvalidRecoveryParam,
    #[snafu(display("Unknown error: {}", error_code))]
    UnknownErr {
        error_code: u32,
        #[serde(skip)]
        backtrace: Option<snafu::Backtrace>,
    },
}

impl VerificationError {
    pub fn unknown_err(error_code: u32) -> Self {
        UnknownErr {
            error_code,
        }.build()
    }
}

impl PartialEq<VerificationError> for VerificationError {
    fn eq(&self, rhs: &VerificationError) -> bool {
        match self {
            VerificationError::BatchErr => matches!(rhs, VerificationError::BatchErr),
            VerificationError::GenericErr => matches!(rhs, VerificationError::GenericErr),
            VerificationError::InvalidHashFormat => {
                matches!(rhs, VerificationError::InvalidHashFormat)
            }
            VerificationError::InvalidPubkeyFormat => {
                matches!(rhs, VerificationError::InvalidPubkeyFormat)
            }
            VerificationError::InvalidSignatureFormat => {
                matches!(rhs, VerificationError::InvalidSignatureFormat)
            }
            VerificationError::InvalidRecoveryParam => {
                matches!(rhs, VerificationError::InvalidRecoveryParam)
            }
            VerificationError::UnknownErr { error_code, .. } => {
                if let VerificationError::UnknownErr {
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
