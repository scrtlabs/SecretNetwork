use crate::results::UnwrapOrSgxErrorUnexpected;

use core::mem;
use core::ptr::null;
use log::*;
use log::{error, info};
use std::io::{Read, Write};
use std::path::Path;
use std::ptr;
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
const FILE_MD_ENCRYPTED_DATA_NODES: usize = 96;

#[repr(packed)]
pub struct FileDataKeyAndMac {
    pub key: [u8; 16],
    pub gmac: [u8; 16],
}

impl FileDataKeyAndMac {
    unsafe fn decrypt_once(
        &self,
        p_src: *const uint8_t,
        src_len: uint32_t,
        p_dst: *mut uint8_t,
        p_iv: &[u8; 12],
    ) -> Result<(), sgx_status_t> {
        let res = sgx_rijndael128GCM_decrypt(
            &self.key,
            p_src,
            src_len,
            p_dst,
            p_iv.as_ptr(),
            12,
            null(),
            0,
            &self.gmac,
        );

        if sgx_status_t::SGX_SUCCESS != res {
            return Err(res);
        }

        Ok(())
    }
}

#[repr(packed)]
pub struct FileMdEncrypted {
    pub clean_filename: [u8; FILE_MD_ENCRYPTED_FILENAME_SIZE],
    pub size: u64,

    // that was deleted in 2.18
    pub mc_uuid: [u8; 16],
    pub mc_value: u32,

    pub root_mht: FileDataKeyAndMac,

    pub data: [u8; FILE_MD_ENCRYPTED_DATA_SIZE],
}

#[repr(packed)]
pub struct FileMd {
    pub plain: FileMdPlain,
    pub encr: FileMdEncrypted,
    pub padding: [u8; 610],
}

#[repr(packed)]
pub struct FileMhtNode {
    pub data: [FileDataKeyAndMac; FILE_MD_ENCRYPTED_DATA_NODES],
    pub lower_nodes: [FileDataKeyAndMac; 32],
}

pub fn unseal_file_from_2_17(
    s_path: &str,
    should_check_fname: bool,
) -> Result<Vec<u8>, sgx_status_t> {
    let mut file = match File::open(s_path) {
        Ok(f) => f,
        Err(e) => {
            warn!("Failed to open file: {}", e);
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }
    };

    let mut bytes = Vec::new();
    if let Err(e) = file.read_to_end(&mut bytes) {
        warn!("Failed to read file: {}", e);
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    if bytes.len() < mem::size_of::<FileMd>() {
        warn!("file too small");
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    unsafe {
        let p_md = bytes.as_mut_ptr() as *const FileMd;

        if (*p_md).plain.update_flag > 0 {
            warn!("file left in recovery mode, unsupported");
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED); // we don't support recovery
        }

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

        let mut cur_key_mac = FileDataKeyAndMac {
            key: sgx_key_128bit_t::default(),
            gmac: (*p_md).plain.meta_data_gmac,
        };

        match sgx_get_key(&key_request, &mut cur_key_mac.key) {
            sgx_status_t::SGX_SUCCESS => {}
            err_code => {
                warn!("gen key failed");
                return Err(err_code);
            }
        }

        let /* mut */ md_decr: FileMdEncrypted = FileMdEncrypted {
            clean_filename: [0; FILE_MD_ENCRYPTED_FILENAME_SIZE],
            size: 0,
            mc_uuid: [0; 16],
            mc_value: 0,
            root_mht: FileDataKeyAndMac{
                key: [0; 16],
                gmac: [0; 16],
            },
            data: [0; FILE_MD_ENCRYPTED_DATA_SIZE],
        };

        let p_iv: [u8; 12] = [0; 12];

        cur_key_mac.decrypt_once(
            std::ptr::addr_of!((*p_md).encr) as *const u8,
            mem::size_of::<FileMdEncrypted>() as u32,
            std::ptr::addr_of!(md_decr) as *mut uint8_t,
            &p_iv,
        )?;

        if should_check_fname {
            let raw_path = s_path.as_bytes();

            let mut fname0: usize = 0;
            for (i, ch) in raw_path.iter().enumerate() {
                if *ch == b'/' {
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

        let ret_size = std::ptr::read_unaligned(std::ptr::addr_of!(md_decr.size)) as usize;

        if ret_size <= FILE_MD_ENCRYPTED_DATA_SIZE {
            bytes.resize(ret_size, 0);
            bytes.copy_from_slice(slice::from_raw_parts(md_decr.data.as_ptr(), ret_size));
        } else {
            let node_size = mem::size_of::<FileMd>(); // 4K

            let num_nodes = (ret_size - FILE_MD_ENCRYPTED_DATA_SIZE + node_size - 1) / node_size;
            if num_nodes > FILE_MD_ENCRYPTED_DATA_NODES {
                warn!("too many nodes, indirect files not supported");
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }

            let size_required = mem::size_of::<FileMd>() + node_size * (num_nodes + 1);
            if bytes.len() < size_required {
                warn!("file too short");
                return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
            }

            let mut bytes_src = bytes;
            bytes = Vec::new();
            bytes.resize(FILE_MD_ENCRYPTED_DATA_SIZE + node_size * num_nodes, 7); // allocate with padding

            let mut offs_dst = FILE_MD_ENCRYPTED_DATA_SIZE;

            ptr::copy_nonoverlapping(md_decr.data.as_ptr(), bytes.as_mut_ptr(), offs_dst);

            // Decode mht node
            let p_mht_node =
                bytes_src.as_mut_ptr().add(mem::size_of::<FileMd>()) as *mut FileMhtNode;

            md_decr.root_mht.decrypt_once(
                p_mht_node as *const u8,
                mem::size_of::<FileMhtNode>() as u32,
                p_mht_node as *mut uint8_t,
                &p_iv,
            )?;

            let mut offs_src = mem::size_of::<FileMd>() + node_size;

            for i_node in 0..num_nodes {
                let keys = &(*p_mht_node).data[i_node];

                keys.decrypt_once(
                    bytes_src.as_ptr().add(offs_src),
                    node_size as u32,
                    bytes.as_mut_ptr().add(offs_dst),
                    &p_iv,
                )?;

                offs_src += node_size;
                offs_dst += node_size;
            }

            bytes.resize(ret_size, 0); // truncate the padding
        }
    };

    Ok(bytes)
}

pub fn migrate_file_from_2_17_safe(
    s_path: &str,
    should_check_fname: bool,
) -> Result<(), sgx_status_t> {
    if Path::new(s_path).exists() {
        if SgxFile::open(s_path).is_ok() {
            info!("File {} is already converted", s_path);
        } else {
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
        }
    } else {
        info!("File {} doesn't exist, skipping", s_path);
    }

    Ok(())
}
