use crate::CryptoError;
//use sgx_trts::trts::rsgx_read_rand;
use rand::*;

// todo: check gramine's random impl
pub fn rand_slice(rand: &mut [u8]) -> Result<(), CryptoError> {
    Ok(rand::thread_rng().fill(rand))
    //.map_err(|_e| CryptoError::RandomError {})
}
