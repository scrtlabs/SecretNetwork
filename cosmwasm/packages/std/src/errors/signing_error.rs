use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use snafu::Snafu;

#[derive(Debug, Serialize, Deserialize, Snafu, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum SigningError {
    #[snafu(display("Invalid private key format"))]
    InvalidPrivateKeyFormat,
    #[snafu(display("Unknown error: {}", error_code))]
    UnknownErr {
        error_code: u32,
        #[serde(skip)]
        backtrace: Option<snafu::Backtrace>,
    },
}

impl SigningError {
    pub fn unknown_err(error_code: u32) -> Self {
        UnknownErr { error_code }.build()
    }
}

impl PartialEq<SigningError> for SigningError {
    fn eq(&self, rhs: &SigningError) -> bool {
        match self {
            SigningError::InvalidPrivateKeyFormat => {
                matches!(rhs, SigningError::InvalidPrivateKeyFormat)
            }
            SigningError::UnknownErr { error_code, .. } => {
                if let SigningError::UnknownErr {
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
