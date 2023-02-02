#[cfg(feature = "dcap")]
pub mod dcap;

#[cfg(not(feature = "dcap"))]
pub mod dcap_mock;

pub mod epid;
mod net;

use sgx_types::{sgx_enclave_id_t, sgx_status_t};

extern "C" {
    pub fn ecall_generate_authentication_material(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        api_key: *const u8,
        api_key_len: u32,
        auth_type: u8,
    ) -> sgx_status_t;
}
