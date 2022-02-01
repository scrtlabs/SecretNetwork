use derive_more::Display;

#[derive(Debug, Display)]
pub enum SignatureError {
    VerificationError,
    ParsingError,
}
