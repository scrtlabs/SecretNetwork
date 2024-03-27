use crate::results::UnwrapOrSgxErrorUnexpected;

use core::mem;
use core::ptr::null;
use log::{error, info};
use std::io::{Read, Write};
use std::path::Path;
use std::sgxfs::SgxFile;
use std::slice;

use sgx_types::*;
use std::untrusted::fs;
use std::untrusted::fs::File;

pub const SCRT_SGX_STORAGE_ENV_VAR: &str = "SCRT_SGX_STORAGE";
pub const DEFAULT_SGX_SECRET_PATH: &str = "/opt/secret/.sgx_secrets/";

pub fn write_to_untrusted(bytes: &[u8], filepath: &str) -> SgxResult<()> {
    let mut f = File::create(filepath)
        .sgx_error_with_log(&format!("Creating file '{}' failed", filepath))?;
    f.write_all(bytes)
        .sgx_error_with_log("Writing File failed!")
}

pub fn seal(data: &[u8], filepath: &str) -> SgxResult<()> {
    let mut file = SgxFile::create(filepath)
        .sgx_error_with_log(&format!("Creating sealed file '{}' failed", filepath))?;

    file.write_all(data)
        .sgx_error_with_log("Writing sealed file failed!")
}

pub fn unseal(filepath: &str) -> SgxResult<Vec<u8>> {
    let mut file = SgxFile::open(filepath)
        .sgx_error_with_log(&format!("Opening sealed file '{}' failed", filepath))?;

    let mut output = vec![];
    file.read_to_end(&mut output)
        .sgx_error_with_log(&format!("Reading sealed file '{}' failed", filepath))?;

    Ok(output)
}

pub fn rewrite_on_untrusted(bytes: &[u8], filepath: &str) -> SgxResult<()> {
    let is_path_exists = fs::try_exists(filepath).unwrap_or(false);

    if is_path_exists {
        fs::remove_file(filepath)
            .sgx_error_with_log(&format!("Removing existing file '{}' failed", filepath))?;
    }

    write_to_untrusted(bytes, filepath)
}

//////////////
#[repr(packed)]
pub struct FileMdPlain {
    pub file_id: u64,
    pub major_version: u8,
    pub minor_version: u8,

    pub key_id: [u8; 32],
    pub cpu_svn: [u8; 16],
    pub isv_svn: u16,
    pub use_user_kdk_key: u8,
    pub attribute_mask_flags: u64,
    pub attribute_mask_xfrm: u64,
    pub meta_data_gmac: [u8; 16],
    pub update_flag: u8,
}

const FILE_MD_ENCRYPTED_DATA_SIZE: usize = 3072;
const FILE_MD_ENCRYPTED_FILENAME_SIZE: usize = 260;

#[repr(packed)]
pub struct FileMdEncrypted {
    pub clean_filename: [u8; FILE_MD_ENCRYPTED_FILENAME_SIZE],
    pub size: u64,

    // that was deleted in 2.18
    pub mc_uuid: [u8; 16],
    pub mc_value: u32,

    pub mht_key: [u8; 16],
    pub mht_gmac: [u8; 16],

    pub data: [u8; FILE_MD_ENCRYPTED_DATA_SIZE],
}

#[repr(packed)]
pub struct FileMd {
    pub plain: FileMdPlain,
    pub encr: FileMdEncrypted,
    pub padding: [u8; 610],
}

pub fn unseal_file_from_2_17(
    s_path: &str,
    should_check_fname: bool,
) -> Result<Vec<u8>, sgx_status_t> {
    let mut file = match File::open(s_path) {
        Ok(f) => f,
        Err(_) => {
            return Err(/*e*/ sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    };

    let mut bytes = Vec::new();
    if file.read_to_end(&mut bytes).is_err() {
        return Err(/*e*/ sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    if bytes.len() < mem::size_of::<FileMd>() {
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    unsafe {
        let p_md = bytes.as_mut_ptr() as *const FileMd;

        let mut key_request = sgx_key_request_t {
            key_name: sgx_types::SGX_KEYSELECT_SEAL,
            key_policy: sgx_types::SGX_KEYPOLICY_MRSIGNER,
            misc_mask: sgx_types::TSEAL_DEFAULT_MISCMASK,
            isv_svn: (*p_md).plain.isv_svn,
            ..Default::default()
        };

        key_request.attribute_mask.flags = sgx_types::TSEAL_DEFAULT_FLAGSMASK;
        key_request.attribute_mask.xfrm = 0x0;

        key_request.cpu_svn.svn = (*p_md).plain.cpu_svn;
        key_request.key_id.id = (*p_md).plain.key_id;

        let mut cur_key: sgx_key_128bit_t = sgx_key_128bit_t::default();

        let mut st = sgx_get_key(&key_request, &mut cur_key);
        if sgx_status_t::SGX_SUCCESS != st {
            return Err(st);
        }

        let /* mut */ md_decr: FileMdEncrypted = FileMdEncrypted {
            clean_filename: [0; FILE_MD_ENCRYPTED_FILENAME_SIZE],
            size: 0,
            mc_uuid: [0; 16],
            mc_value: 0,
            mht_key: [0; 16],
            mht_gmac: [0; 16],

            data: [0; 3072],
        };

        let p_iv: [u8; 12] = [0; 12];

        st = sgx_rijndael128GCM_decrypt(
            &cur_key,
            std::ptr::addr_of!((*p_md).encr) as *const u8,
            mem::size_of::<FileMdEncrypted>() as u32,
            std::ptr::addr_of!(md_decr) as *mut uint8_t,
            p_iv.as_ptr() as *const u8,
            12,
            null(),
            0,
            &(*p_md).plain.meta_data_gmac,
        );

        if sgx_status_t::SGX_SUCCESS != st {
            return Err(st);
        }

        let ret_size = std::ptr::read_unaligned(std::ptr::addr_of!(md_decr.size)) as usize;
        if ret_size > FILE_MD_ENCRYPTED_DATA_SIZE {
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }

        bytes.resize(ret_size, 0);
        bytes.copy_from_slice(slice::from_raw_parts(md_decr.data.as_ptr(), ret_size));

        if should_check_fname {
            let raw_path = s_path.as_bytes();

            let mut fname0: usize = 0;
            for (i, ch) in raw_path.iter().enumerate() {
                if *ch == b'/' as u8 {
                    fname0 = i + 1;
                }
            }

            let file_name_len = raw_path.len() - fname0;

            if file_name_len > FILE_MD_ENCRYPTED_FILENAME_SIZE {
                return Err(sgx_status_t::SGX_ERROR_FILE_NAME_MISMATCH);
            }

            if (file_name_len < FILE_MD_ENCRYPTED_FILENAME_SIZE)
                && (md_decr.clean_filename[file_name_len] != 0)
            {
                return Err(sgx_status_t::SGX_ERROR_FILE_NAME_MISMATCH);
            }

            let src_name = slice::from_raw_parts(&raw_path[fname0], file_name_len);
            let dst_name = slice::from_raw_parts(&md_decr.clean_filename[0], file_name_len);

            if src_name != dst_name {
                return Err(sgx_status_t::SGX_ERROR_FILE_NAME_MISMATCH);
            }
        }
    };

    Ok(bytes)
}

pub fn migrate_file_from_2_17_safe(
    s_path: &str,
    should_check_fname: bool,
) -> Result<(), sgx_status_t> {
    if Path::new(s_path).exists() {
        let data = match unseal_file_from_2_17(s_path, should_check_fname) {
            Ok(x) => x,
            Err(e) => {
                error!("Couldn't unseal file {}, {}", s_path, e);
                return Err(e);
            }
        };

        let s_path_bkp = s_path.to_string() + ".bkp";
        if let Err(e) = fs::copy(s_path, &s_path_bkp) {
            error!("Couldn't backup {} into {}, {}", s_path, s_path_bkp, e);
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }

        if let Err(e) = seal(data.as_slice(), s_path) {
            error!("Couldn't RE-seal file {}, {}", s_path, e);
            return Err(e);
        }

        info!("File {} successfully RE-sealed", s_path);
    } else {
        info!("File {} doesn't exist, skipping", s_path);
    }

    Ok(())
}
