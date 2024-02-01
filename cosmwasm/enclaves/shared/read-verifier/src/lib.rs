pub mod ecalls;
pub mod read_proofs;

#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;
extern crate sgx_types;

use crate::read_proofs::ReadProofer;
use lazy_static::lazy_static;
use std::sync::SgxMutex;

lazy_static! {
    pub static ref READ_PROOFER: SgxMutex<ReadProofer> = SgxMutex::new(ReadProofer::default());
}
