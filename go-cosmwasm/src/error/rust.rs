use errno::{set_errno, Errno};

use cosmwasm_sgx_vm::VmError;
use snafu::Snafu;

use crate::memory::Buffer;

#[derive(Debug, Snafu)]
pub enum Error {
    #[snafu(display("Null/Empty argument: {}", name))]
    EmptyArg {
        name: String,
        #[cfg(feature = "backtraces")]
        backtrace: snafu::Backtrace,
    },
    /// Whenever UTF-8 bytes cannot be decoded into a unicode string, e.g. in String::from_utf8 or str::from_utf8.
    #[snafu(display("Cannot decode UTF8 bytes into string: {}", msg))]
    InvalidUtf8 {
        msg: String,
        backtrace: snafu::Backtrace,
    },
    #[snafu(display("Ran out of gas"))]
    OutOfGas {
        #[cfg(feature = "backtraces")]
        backtrace: snafu::Backtrace,
    },
    #[snafu(display("Caught Panic"))]
    Panic {
        #[cfg(feature = "backtraces")]
        backtrace: snafu::Backtrace,
    },
    #[snafu(display("Execution error: {}", msg))]
    VmErr {
        msg: String,
        #[cfg(feature = "backtraces")]
        backtrace: snafu::Backtrace,
    },
    #[snafu(display("{}", msg))]
    GoCwEnclaveError {
        msg: String,
        #[cfg(feature = "backtraces")]
        backtrace: snafu::Backtrace,
    },
}

impl Error {
    pub fn empty_arg<T: Into<String>>(name: T) -> Self {
        EmptyArg { name: name.into() }.build()
    }

    pub fn invalid_utf8<S: ToString>(msg: S) -> Self {
        InvalidUtf8 {
            msg: msg.to_string(),
        }
        .build()
    }

    pub fn panic() -> Self {
        Panic {}.build()
    }

    pub fn vm_err<S: ToString>(msg: S) -> Self {
        VmErr {
            msg: msg.to_string(),
        }
        .build()
    }

    pub fn enclave_err<S: ToString>(msg: S) -> Self {
        GoCwEnclaveError {
            msg: msg.to_string(),
        }
        .build()
    }

    pub fn out_of_gas() -> Self {
        OutOfGas {}.build()
    }
}

impl From<VmError> for Error {
    fn from(source: VmError) -> Self {
        match source {
            VmError::GasDepletion => Error::out_of_gas(),
            _ => Error::vm_err(source),
        }
    }
}

impl From<std::str::Utf8Error> for Error {
    fn from(source: std::str::Utf8Error) -> Self {
        Error::invalid_utf8(source)
    }
}

impl From<std::string::FromUtf8Error> for Error {
    fn from(source: std::string::FromUtf8Error) -> Self {
        Error::invalid_utf8(source)
    }
}

/// cbindgen:prefix-with-name
#[repr(i32)]
enum ErrnoValue {
    Success = 0,
    Other = 1,
    OutOfGas = 2,
}

pub fn clear_error() {
    set_errno(Errno(ErrnoValue::Success as i32));
}

pub fn set_error(err: Error, errout: Option<&mut Buffer>) {
    let msg = err.to_string();
    if let Some(mb) = errout {
        *mb = Buffer::from_vec(msg.into_bytes());
    }
    let errno = match err {
        Error::OutOfGas { .. } => ErrnoValue::OutOfGas,
        _ => ErrnoValue::Other,
    } as i32;
    set_errno(Errno(errno));
}

/// If `result` is Ok, this returns the binary representation of the Ok value and clears the error in `errout`.
/// Otherwise it returns an empty vector and writes the error to `errout`.
pub fn handle_c_error<T>(result: Result<T, Error>, errout: Option<&mut Buffer>) -> Vec<u8>
where
    T: Into<Vec<u8>>,
{
    match result {
        Ok(value) => {
            clear_error();
            value.into()
        }
        Err(error) => {
            set_error(error, errout);
            Vec::new()
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use cosmwasm_sgx_vm::FfiError;
    use std::str;

    #[test]
    fn empty_arg_works() {
        let error = Error::empty_arg("gas");
        match error {
            Error::EmptyArg { name, .. } => {
                assert_eq!(name, "gas");
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn invalid_utf8_works_for_strings() {
        let error = Error::invalid_utf8("my text");
        match error {
            Error::InvalidUtf8 { msg, .. } => {
                assert_eq!(msg, "my text");
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn invalid_utf8_works_for_errors() {
        let original = String::from_utf8(vec![0x80]).unwrap_err();
        let error = Error::invalid_utf8(original);
        match error {
            Error::InvalidUtf8 { msg, .. } => {
                assert_eq!(msg, "invalid utf-8 sequence of 1 bytes from index 0");
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn panic_works() {
        let error = Error::panic();
        match error {
            Error::Panic { .. } => {}
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn vm_err_works_for_strings() {
        let error = Error::vm_err("my text");
        match error {
            Error::VmErr { msg, .. } => {
                assert_eq!(msg, "my text");
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn vm_err_works_for_errors() {
        // No public interface exists to generate a VmError directly
        let original: VmError = FfiError::out_of_gas().into();
        let error = Error::vm_err(original);
        match error {
            Error::VmErr { msg, .. } => {
                assert_eq!(msg, "Ran out of gas during contract execution");
            }
            _ => panic!("expect different error"),
        }
    }

    // Tests of `impl From<X> for Error` converters

    #[test]
    fn from_std_str_utf8error_works() {
        let error: Error = str::from_utf8(b"Hello \xF0\x90\x80World")
            .unwrap_err()
            .into();
        match error {
            Error::InvalidUtf8 { msg, .. } => {
                assert_eq!(msg, "invalid utf-8 sequence of 3 bytes from index 6")
            }
            _ => panic!("expect different error"),
        }
    }

    #[test]
    fn from_std_string_fromutf8error_works() {
        let error: Error = String::from_utf8(b"Hello \xF0\x90\x80World".to_vec())
            .unwrap_err()
            .into();
        match error {
            Error::InvalidUtf8 { msg, .. } => {
                assert_eq!(msg, "invalid utf-8 sequence of 3 bytes from index 6")
            }
            _ => panic!("expect different error"),
        }
    }
}
