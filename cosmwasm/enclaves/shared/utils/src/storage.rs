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
const FILE_NODE_DATA_NODES: usize = 96;
const FILE_NODE_CHILD_NODES: usize = 32;

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
    ) -> Result<(), sgx_status_t> {
        let p_iv: [u8; 12] = [0; 12];

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
    pub data: [FileDataKeyAndMac; FILE_NODE_DATA_NODES],
    pub lower_nodes: [FileDataKeyAndMac; FILE_NODE_CHILD_NODES],
}

pub struct MigrationContext {
    pub m_inp: Vec<u8>,
    pub m_res: Vec<u8>,
}

impl MigrationContext {
    pub fn proceed_from_2_17(
        &mut self,
        s_path: &str,
        should_check_fname: bool,
    ) -> Result<(), sgx_status_t> {
        if self.m_inp.len() < mem::size_of::<FileMd>() {
            warn!("file too small");
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }

        return unsafe { self.proceed_internal(s_path, should_check_fname) };
    }

    unsafe fn proceed_internal(
        &mut self,
        s_path: &str,
        should_check_fname: bool,
    ) -> Result<(), sgx_status_t> {
        let p_md = self.m_inp.as_mut_ptr() as *mut FileMd;

        if (*p_md).plain.update_flag > 0 {
            warn!("file left in recovery mode, unsupported");
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }

        let md_key = self.get_md_key(&*p_md)?;

        let p_md_encr = std::ptr::addr_of_mut!((*p_md).encr);

        md_key.decrypt_once(
            p_md_encr as *const u8,
            mem::size_of::<FileMdEncrypted>() as u32,
            p_md_encr as *mut uint8_t,
        )?;

        if should_check_fname {
            MigrationContext::verify_filename(&*p_md_encr, s_path)?;
        }

        //let ret_size = (*p_md_encr).size as usize;
        let ret_size = std::ptr::read_unaligned(std::ptr::addr_of!((*p_md_encr).size)) as usize;

        if ret_size <= FILE_MD_ENCRYPTED_DATA_SIZE {
            self.allocate_res(ret_size, ret_size, &*p_md_encr);
        } else {
            self.process_all_mht_nodes(ret_size, &mut *p_md_encr)?;
        }

        Ok(())
    }

    unsafe fn get_md_key(&self, md: &FileMd) -> Result<FileDataKeyAndMac, sgx_status_t> {
        let mut key_request = sgx_key_request_t {
            key_name: sgx_types::SGX_KEYSELECT_SEAL,
            key_policy: sgx_types::SGX_KEYPOLICY_MRSIGNER,
            misc_mask: sgx_types::TSEAL_DEFAULT_MISCMASK,
            isv_svn: md.plain.isv_svn,
            ..Default::default()
        };

        key_request.attribute_mask.flags = sgx_types::TSEAL_DEFAULT_FLAGSMASK;
        key_request.attribute_mask.xfrm = 0x0;

        key_request.cpu_svn.svn = md.plain.cpu_svn;
        key_request.key_id.id = md.plain.key_id;

        let mut ret = FileDataKeyAndMac {
            key: sgx_key_128bit_t::default(),
            gmac: md.plain.meta_data_gmac,
        };

        match sgx_get_key(&key_request, &mut ret.key) {
            sgx_status_t::SGX_SUCCESS => {}
            err_code => {
                warn!("gen key failed");
                return Err(err_code);
            }
        }

        Ok(ret)
    }

    unsafe fn verify_filename(md: &FileMdEncrypted, s_path: &str) -> Result<(), sgx_status_t> {
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
            && (md.clean_filename[file_name_len] != 0)
        {
            return Err(sgx_status_t::SGX_ERROR_FILE_NAME_MISMATCH);
        }

        let src_name = slice::from_raw_parts(&raw_path[fname0], file_name_len);
        let dst_name = slice::from_raw_parts(&md.clean_filename[0], file_name_len);

        if src_name != dst_name {
            return Err(sgx_status_t::SGX_ERROR_FILE_NAME_MISMATCH);
        }

        Ok(())
    }

    unsafe fn allocate_res(&mut self, size: usize, size_from_md: usize, md: &FileMdEncrypted) {
        self.m_res.resize(size, 0);
        ptr::copy_nonoverlapping(md.data.as_ptr(), self.m_res.as_mut_ptr(), size_from_md);
    }

    unsafe fn process_all_mht_nodes(
        &mut self,
        ret_size: usize,
        md: &mut FileMdEncrypted,
    ) -> Result<(), sgx_status_t> {
        let node_size = mem::size_of::<FileMd>(); // 4K

        let mut data_nodes = (ret_size - FILE_MD_ENCRYPTED_DATA_SIZE + node_size - 1) / node_size;
        let mht_nodes = (data_nodes + FILE_NODE_DATA_NODES - 1) / FILE_NODE_DATA_NODES;

        let size_required = mem::size_of::<FileMd>() + node_size * (data_nodes + mht_nodes);
        if self.m_inp.len() < size_required {
            warn!("file too short");
            return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
        }

        // allocate with padding
        self.allocate_res(
            FILE_MD_ENCRYPTED_DATA_SIZE + node_size * data_nodes,
            FILE_MD_ENCRYPTED_DATA_SIZE,
            &md,
        );

        let mut offs_dst = FILE_MD_ENCRYPTED_DATA_SIZE;
        let mut offs_src = mem::size_of::<FileMd>();

        let mut p_mk_0 = std::ptr::addr_of_mut!(md.root_mht);
        let mut p_mk_1 = p_mk_0.add(1);

        loop {
            let chunk = if data_nodes <= FILE_NODE_DATA_NODES {
                data_nodes
            } else {
                FILE_NODE_DATA_NODES
            };

            let p_mht_node =
                self.process_mht_node(&(*p_mk_0), &mut offs_src, &mut offs_dst, chunk)?;

            if chunk == data_nodes {
                break;
            }

            ptr::copy_nonoverlapping(
                (*p_mht_node).lower_nodes.as_ptr(),
                p_mk_1,
                FILE_NODE_CHILD_NODES,
            );

            p_mk_0 = p_mk_0.add(1);
            p_mk_1 = p_mk_1.add(FILE_NODE_CHILD_NODES);

            data_nodes -= chunk;
        }

        self.m_res.resize(ret_size, 0); // truncate the padding

        Ok(())
    }

    unsafe fn process_mht_node(
        &mut self,
        parent_km: &FileDataKeyAndMac,
        offs_src: &mut usize,
        offs_dst: &mut usize,
        num_nodes: usize,
    ) -> Result<*const FileMhtNode, sgx_status_t> {
        // Decode mht node
        let node_size = mem::size_of::<FileMd>(); // 4K

        let p_mht_node = self.m_inp.as_mut_ptr().add(*offs_src) as *mut FileMhtNode;
        *offs_src += mem::size_of::<FileMhtNode>();

        parent_km.decrypt_once(
            p_mht_node as *const u8,
            mem::size_of::<FileMhtNode>() as u32,
            p_mht_node as *mut uint8_t,
        )?;

        for i_node in 0..num_nodes {
            let km = &(*p_mht_node).data[i_node];

            km.decrypt_once(
                self.m_inp.as_ptr().add(*offs_src),
                node_size as u32,
                self.m_res.as_mut_ptr().add(*offs_dst),
            )?;

            *offs_src += node_size;
            *offs_dst += node_size;
        }

        Ok(p_mht_node)
    }
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

    let mut mctx = MigrationContext {
        m_inp: Vec::new(),
        m_res: Vec::new(),
    };

    if let Err(e) = file.read_to_end(&mut mctx.m_inp) {
        warn!("Failed to read file: {}", e);
        return Err(sgx_status_t::SGX_ERROR_UNEXPECTED);
    }

    mctx.proceed_from_2_17(s_path, should_check_fname)?;

    Ok(mctx.m_res)
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


/*
pub fn test_migration_once(size: usize) {
    println!("Testing SGX migration, size={}", size);

    let mut data: Vec<u8> = Vec::new();
    data.reserve(size);

    for i in 0..size {
        data.push(((i * 17) % 251) as u8);
    }

    {
        let mut f = match SgxFile::create("large_file") {
            Ok(file) => file,
            Err(_) => {
                return;
            }
        };

        if f.write_all(&data).is_err() {
            return;
        }
    }

    let data2 = match unseal_file_from_2_17("large_file", true) {
        Ok(d) => d,
        Err(e) => {
            println!("Unseal failed: {}", e);
            return;
        }
    };

    if data.as_slice() == data2.as_slice() {
        println!("match");
    } else {
        println!("MIS-match");
    }
}

pub fn test_migration() {
    test_migration_once(0);
    test_migration_once(19);
    test_migration_once(500);
    test_migration_once(3072);
    test_migration_once(3073); // indirect
    test_migration_once(20000);
    test_migration_once(3072 + 96 * 4096 - 100);
    test_migration_once(3072 + 96 * 4096);
    test_migration_once(3072 + 96 * 4096 + 100); // 2nd-order
    test_migration_once(3072 + 96 * 4096 * 33); // max 2nd-order
    test_migration_once(50000000); // huge
}
*/
