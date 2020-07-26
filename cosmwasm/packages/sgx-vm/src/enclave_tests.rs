// I reexport `sgx_types` here so that `go-cosmwasm` doesn't need to specify it as a dev-dependency.
pub use sgx_types;
use sgx_types::{sgx_enclave_id_t, sgx_status_t};

pub use crate::enclave::get_enclave;

#[link(name = "rust_cosmwasm_enclave.signed")]
extern "C" {
    pub fn ecall_run_tests(eid: sgx_enclave_id_t) -> sgx_status_t;
}
