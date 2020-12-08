/// This macro coerces all `?` marks in its scope to convert to the requested error type, and returns a Result
/// of that error type.
///
/// This macro takes two arguments: an error type (`E`), and and expression or code block.
/// It then wraps the code block in a closure that returns `Result<T, E>` such that T is the type of the
/// expression or last expression in the code block.
///
/// This is useful when you have a scope in a bigger function in which `?`'s should convert to a type that is different
/// from the error type of the containing function.
///
/// # Example
///
/// ```rust
/// struct Error1;
/// struct Error2;
/// struct Error3;
///
/// impl From<Error1> for Error2 {
///     fn from (_: Error1) -> Error2 {
///         $T2
///     }
/// }
///
/// impl From<Error2> for Error3 {
///     fn from (_: Error2) -> Error3 {
///         $T2
///     }
/// }
///
/// fn err1() -> Result<(), Error1> { Err(Error1) }
/// fn err2() -> Result<(), Error2> { Err(Error2) }
/// fn err3() -> Result<(), Error3> { Err(Error3) }
///
/// fn example() -> Result<(), Error3> {
///     err3()?; // just works
///     err2()?; // from!(Error2, Error3);
///     // uses both conversions
///     coalesce!(Error2, {
///         err1()?;
///         err2()?;
///         Ok(())
///     })?;
///     Ok(())
/// }
/// ```
#[macro_export]
macro_rules! coalesce {
    ($error_type: ty, $body: expr) => {{
        #[allow(unused_mut)]
        let mut wrapper = || -> std::result::Result<_, $error_type> { $body };
        wrapper()
    }};
}

#[macro_export]
macro_rules! validate_const_ptr {
    ($ptr:expr, $ptr_len:expr) => {{
        if let Err(_e) = {
            if $ptr.is_null() || $ptr_len == 0 {
                ::log::warn!("Tried to access an empty pointer - ptr.is_null()");
                Err(::sgx_types::sgx_status_t::SGX_ERROR_UNEXPECTED)
            } else {
                ::sgx_trts::trts::rsgx_lfence();
                Ok(())
            }
        } {
            ::log::error!("Tried to access data outside enclave memory!");
            return $crate::results::result_init_success_to_initresult(Err(
                ::enclave_ffi_types::EnclaveError::FailedFunctionCall,
            ));
        }
    }};
}
