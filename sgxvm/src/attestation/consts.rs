use sgx_types::sgx_quote_sign_type_t;

#[cfg(feature = "production")]
pub const MRSIGNER: [u8; 32] = [117, 49, 209, 78, 24, 190, 212, 85, 43, 93, 191, 167, 37, 107, 46, 128, 149, 149, 220, 14, 137, 80, 163, 203, 250, 68, 219, 58, 99, 217, 42, 59];
#[cfg(not(feature = "production"))]
pub const MRSIGNER: [u8; 32] = [131, 215, 25, 231, 125, 234, 202, 20, 112, 246, 186, 246, 42, 77, 119, 67, 3, 200, 153, 219, 105, 2, 15, 156, 112, 238, 29, 252, 8, 199, 206, 158];

#[cfg(feature = "production")]
pub const MIN_REQUIRED_SVN: u16 = 1;
#[cfg(not(feature = "production"))]
pub const MIN_REQUIRED_SVN: u16 = 0;

pub const DEV_HOSTNAME: &str = "api.trustedservices.intel.com";
pub const SIGRL_SUFFIX: &str = "/sgx/dev/attestation/v5/sigrl/";
pub const REPORT_SUFFIX: &str = "/sgx/dev/attestation/v5/report";
pub const CERTEXPIRYDAYS: i64 = 90i64;

pub const PUBLIC_KEY_SIZE: usize = 32;
pub const ENCRYPTED_KEY_SIZE: usize = 78;

pub const QUOTE_SIGNATURE_TYPE: sgx_quote_sign_type_t = sgx_quote_sign_type_t::SGX_LINKABLE_SIGNATURE; 
pub const MIN_REQUIRED_TCB: u64 = 16;