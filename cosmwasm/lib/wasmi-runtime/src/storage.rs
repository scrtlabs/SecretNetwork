use log::debug;
use sgx_tseal::SgxSealedData;
use sgx_types::marker::ContiguousMemory;
use sgx_types::{sgx_attributes_t, sgx_sealed_data_t, sgx_status_t};
use std::untrusted::fs::File;

pub const SEALING_KEY_SIZE: usize = 32;
pub const SEAL_LOG_SIZE: usize = 2048;

#[derive(Copy, Clone, Default, Debug)]
pub struct SecretKeyStorage {
    pub version: u32,
    pub data: [u8; SEALING_KEY_SIZE],
}

unsafe impl ContiguousMemory for SecretKeyStorage {}

impl SecretKeyStorage {
    /// safe seal
    /// param: the_data : clear text to be sealed
    /// param: sealed_log_out : the output of the sealed data
    /// The flags are from here: https://github.com/intel/linux-sgx/blob/master/common/inc/sgx_attributes.h#L38
    /// additional is a part of AES-GCM that you can authenticate data with the MAC without encrypting it.
    pub fn seal_key(&self, sealed_log_out: &mut [u8; SEAL_LOG_SIZE]) {
        let additional: [u8; 0] = [0_u8; 0];
        let attribute_mask = sgx_attributes_t {
            flags: 0xffff_ffff_ffff_fff3,
            xfrm: 0,
        };
        // todo: change the key policy to MRENCLAVE and create an upgrade mechanism for updating the enclave
        let sealed_data = SgxSealedData::<Self>::seal_data_ex(
            sgx_types::SGX_KEYPOLICY_MRSIGNER, //key policy
            attribute_mask,
            0, //misc mask
            &additional,
            &self,
        )
        .unwrap();
        // to sealed_log ->
        //    let mut sealed_log_arr:[u8;2048] = [0;2048];
        let sealed_log = sealed_log_out.as_mut_ptr();
        let sealed_log_size: usize = 2048;
        to_sealed_log(&sealed_data, sealed_log, sealed_log_size as u32);
    }

    // TODO: Add Error Handling.
    /// unseal key
    /// param: sealed_log_in : the encrypted blob
    /// param: udata : the SecreyKeyStorage (clear text)
    pub fn unseal_key(sealed_log_in: &mut [u8]) -> Option<SecretKeyStorage> {
        let sealed_log_size: usize = SEAL_LOG_SIZE;
        let sealed_log = sealed_log_in.as_mut_ptr();
        let sealed_data = from_sealed_log::<SecretKeyStorage>(sealed_log, sealed_log_size as u32)?;
        let unsealed_result = sealed_data.unseal_data();
        match unsealed_result {
            Ok(unsealed_data) => {
                let udata = unsealed_data.get_decrypt_txt();
                Some(*udata)
            }
            Err(err) => {
                // TODO: Handle this. It can causes panic in Simulation Mode until deleting the file.
                if err == sgx_status_t::SGX_ERROR_MAC_MISMATCH {
                    None
                } else {
                    panic!(err)
                }
            }
        }
    }
}

/// This casts sealed_log from *u8 to *sgx_sealed_data_t which aren't aligned the same way.
fn to_sealed_log<T: Copy + ContiguousMemory>(
    sealed_data: &SgxSealedData<T>,
    sealed_log: *mut u8,
    sealed_log_size: u32,
) -> Option<*mut sgx_sealed_data_t> {
    unsafe {
        sealed_data.to_raw_sealed_data_t(sealed_log as *mut sgx_sealed_data_t, sealed_log_size)
    }
}

/// This casts a *sgx_sealed_data_t to *u8 which aren't aligned the same way.
fn from_sealed_log<'a, T: Copy + ContiguousMemory>(
    sealed_log: *mut u8,
    sealed_log_size: u32,
) -> Option<SgxSealedData<'a, T>> {
    unsafe {
        SgxSealedData::<T>::from_raw_sealed_data_t(
            sealed_log as *mut sgx_sealed_data_t,
            sealed_log_size,
        )
    }
}

/// Save sealed key to file system
pub fn save_sealed_key(path: &str, sealed_key: &[u8]) {
    let opt = File::create(path);
    if opt.is_ok() {
        debug!("Created file => {} ", path);
        let mut file = opt.unwrap();
        let result = file.write_all(&sealed_key);
        if result.is_ok() {
            debug!("success writting to file! ");
        } else {
            debug!("error writting to file! ");
        }
    }
}
