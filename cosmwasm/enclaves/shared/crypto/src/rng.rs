use crate::CryptoError;
use sgx_trts::trts::rsgx_read_rand;

pub fn rand_slice(rand: &mut [u8]) -> Result<(), CryptoError> {
    rsgx_read_rand(rand).map_err(|_e| CryptoError::RandomError {})
}
