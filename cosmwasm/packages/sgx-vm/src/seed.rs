use enclave_ffi_types::{HealthCheckResult, PUBLIC_KEY_SIZE};
use sgx_types::*;

use log::info;

use crate::enclave::{ecall_submit_validator_set_evidence, ENCLAVE_DOORBELL};

extern "C" {
    pub fn ecall_init_node(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        master_key: *const u8,
        master_key_len: u32,
        encrypted_seed: *const u8,
        encrypted_seed_len: u32,
    ) -> sgx_status_t;

    pub fn ecall_init_bootstrap(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        public_key: &mut [u8; 32],
    ) -> sgx_status_t;

    pub fn ecall_key_gen(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        public_key: &mut [u8; 32],
    ) -> sgx_status_t;

    pub fn ecall_migration_op(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        opcode: u32,
    ) -> sgx_status_t;

    pub fn ecall_get_network_pubkey(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        i_seed: u32,
        p_node: *mut u8,
        p_io: *mut u8,
        p_seeds: *mut u32,
    ) -> sgx_status_t;

    pub fn ecall_rotate_store(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        p_kv: *mut u8,
        n_kv: u32,
    ) -> sgx_status_t;

    pub fn ecall_onchain_approve_upgrade(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        msg: *const u8,
        msg_len: u32,
    ) -> sgx_types::sgx_status_t;

    pub fn ecall_onchain_approve_machine_id(
        eid: sgx_enclave_id_t,
        retval: *mut sgx_status_t,
        p_id: *const u8,
        n_id: u32,
        p_proof: *mut u8,
        is_on_chain: bool,
    ) -> sgx_types::sgx_status_t;

    /// Trigger a query method in a wasm contract
    pub fn ecall_health_check(
        eid: sgx_enclave_id_t,
        retval: *mut HealthCheckResult,
    ) -> sgx_status_t;
}

pub fn untrusted_health_check() -> SgxResult<HealthCheckResult> {
    //info!("Initializing enclave..");

    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    //debug!("Initialized enclave successfully!");

    let eid = enclave.geteid();
    let mut ret = HealthCheckResult::default();

    let status = unsafe { ecall_health_check(eid, &mut ret) };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    Ok(ret)
}

pub fn untrusted_init_node(master_key: &[u8], encrypted_seed: &[u8]) -> SgxResult<()> {
    info!("Initializing enclave..");

    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    info!("Initialized enclave successfully!");

    let eid = enclave.geteid();
    let mut ret = sgx_status_t::SGX_SUCCESS;

    let status = unsafe {
        ecall_init_node(
            eid,
            &mut ret,
            master_key.as_ptr(),
            master_key.len() as u32,
            encrypted_seed.as_ptr(),
            encrypted_seed.len() as u32,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if ret != sgx_status_t::SGX_SUCCESS {
        return Err(ret);
    }

    Ok(())
}

pub fn untrusted_submit_validator_set_evidence(evidence: [u8; 32]) -> SgxResult<()> {
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    let eid = enclave.geteid();
    let mut ret = sgx_status_t::SGX_SUCCESS;
    let status = unsafe { ecall_submit_validator_set_evidence(eid, &mut ret, evidence.as_ptr()) };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if ret != sgx_status_t::SGX_SUCCESS {
        return Err(ret);
    }

    Ok(())
}

pub fn untrusted_rotate_store(p_buf: *mut u8, n_buf: u32) -> SgxResult<()> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    //info!("Initialized enclave successfully!");

    let eid = enclave.geteid();
    let mut ret = sgx_status_t::SGX_SUCCESS;
    let status = unsafe { ecall_rotate_store(eid, &mut ret, p_buf, n_buf) };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if ret != sgx_status_t::SGX_SUCCESS {
        return Err(ret);
    }

    // println!("untrusted_rotate_store res: {}", unsafe {
    //     hex::encode(std::slice::from_raw_parts(p_buf, n_buf as usize))
    // });

    Ok(())
}

pub fn untrusted_migration_op(opcode: u32) -> SgxResult<()> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    //info!("Initialized enclave successfully!");

    let eid = enclave.geteid();
    let mut ret = sgx_status_t::SGX_SUCCESS;
    let status = unsafe { ecall_migration_op(eid, &mut ret, opcode) };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if ret != sgx_status_t::SGX_SUCCESS {
        return Err(ret);
    }

    Ok(())
}

pub fn untrusted_get_network_pubkey(
    i_seed: u32,
) -> SgxResult<(u32, [u8; PUBLIC_KEY_SIZE], [u8; PUBLIC_KEY_SIZE])> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    let mut pk_node = [0u8; PUBLIC_KEY_SIZE];
    let mut pk_io = [0u8; PUBLIC_KEY_SIZE];
    let mut seeds: u32 = 0;

    let eid = enclave.geteid();
    let mut ret = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_get_network_pubkey(
            eid,
            &mut ret,
            i_seed,
            pk_node.as_mut_ptr(),
            pk_io.as_mut_ptr(),
            &mut seeds,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if ret != sgx_status_t::SGX_SUCCESS {
        return Err(ret);
    }

    Ok((seeds, pk_node, pk_io))
}

pub fn untrusted_approve_upgrade(msg_slice: &[u8]) -> SgxResult<()> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    //info!("Initialized enclave successfully!");

    let eid = enclave.geteid();
    let mut ret = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_onchain_approve_upgrade(eid, &mut ret, msg_slice.as_ptr(), msg_slice.len() as u32)
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if ret != sgx_status_t::SGX_SUCCESS {
        return Err(ret);
    }

    Ok(())
}

pub fn untrusted_approve_machine_id(
    machine_id: &[u8],
    proof: *mut u8,
    is_on_chain: bool,
) -> SgxResult<()> {
    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    //info!("Initialized enclave successfully!");

    let eid = enclave.geteid();
    let mut ret = sgx_status_t::SGX_SUCCESS;
    let status = unsafe {
        ecall_onchain_approve_machine_id(
            eid,
            &mut ret,
            machine_id.as_ptr(),
            machine_id.len() as u32,
            proof,
            is_on_chain,
        )
    };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if ret != sgx_status_t::SGX_SUCCESS {
        return Err(ret);
    }

    Ok(())
}

pub fn untrusted_key_gen() -> SgxResult<[u8; 32]> {
    info!("Initializing enclave..");

    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    info!("Initialized enclave successfully!");

    let eid = enclave.geteid();
    let mut retval = sgx_status_t::SGX_SUCCESS;
    let mut public_key = [0u8; 32];
    // let status = unsafe { ecall_get_encrypted_seed(eid, &mut retval, cert, cert_len, & mut seed) };
    let status = unsafe { ecall_key_gen(eid, &mut retval, &mut public_key) };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        return Err(retval);
    }

    Ok(public_key)
}

pub fn untrusted_init_bootstrap() -> SgxResult<[u8; 32]> {
    info!("Hello from just before initializing - untrusted_init_bootstrap");

    // Bind the token to a local variable to ensure its
    // destructor runs in the end of the function
    let enclave_access_token = ENCLAVE_DOORBELL
        .get_access(1) // This can never be recursive
        .ok_or(sgx_status_t::SGX_ERROR_BUSY)?;
    let enclave = (*enclave_access_token)?;

    info!("Hello from just after initializing - untrusted_init_bootstrap");

    let eid = enclave.geteid();
    let mut retval = sgx_status_t::SGX_SUCCESS;
    let mut public_key = [0u8; 32];
    // let status = unsafe { ecall_get_encrypted_seed(eid, &mut retval, cert, cert_len, & mut seed) };
    let status = unsafe { ecall_init_bootstrap(eid, &mut retval, &mut public_key) };

    if status != sgx_status_t::SGX_SUCCESS {
        return Err(status);
    }

    if retval != sgx_status_t::SGX_SUCCESS {
        return Err(retval);
    }

    Ok(public_key)
}
