use crate::results::UnwrapOrSgxErrorUnexpected;
use core::{mem, ptr::null};
use enclave_crypto::{consts::*, AESKey, Kdf, SIVEncryptable};
use lazy_static::lazy_static;
use log::*;
use sgx_types::*;
use std::{
    env,
    io::{Read, Write},
    os::unix::ffi::OsStrExt,
    path, ptr,
    ptr::null_mut,
    sgxfs::SgxFile,
    slice,
    untrusted::{fs, fs::File, path::PathEx},
};

pub fn get_key_from_seed(seed: &[u8]) -> sgx_key_128bit_t {
    let mut key_request = sgx_types::sgx_key_request_t {
        key_name: sgx_types::SGX_KEYSELECT_SEAL,
        key_policy: sgx_types::SGX_KEYPOLICY_MRENCLAVE | sgx_types::SGX_KEYPOLICY_MRSIGNER,
        misc_mask: sgx_types::TSEAL_DEFAULT_MISCMASK,
        ..Default::default()
    };

    if seed.len() > key_request.key_id.id.len() {
        panic!("seed too long: {:?}", seed);
    }

    key_request.key_id.id[..seed.len()].copy_from_slice(seed);

    key_request.attribute_mask.flags = sgx_types::TSEAL_DEFAULT_FLAGSMASK;

    let mut key = sgx_key_128bit_t::default();
    let res = unsafe { sgx_get_key(&key_request, &mut key) };

    if res != sgx_status_t::SGX_SUCCESS {
        panic!("sealing key derive failed: {}", res);
    }

    key
}

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

        unsafe { self.proceed_internal(s_path, should_check_fname) }
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
            md,
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

fn migrate_file_from_2_17_safe(
    file_name: &str,
    should_check_fname: bool,
) -> Result<(), sgx_status_t> {
    let str_path = make_sgx_secret_path(file_name);
    let s_path = str_path.as_str();

    if path::Path::new(s_path).exists() {
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

pub fn migrate_all_from_2_17() -> sgx_types::sgx_status_t {
    if let Err(e) = migrate_file_from_2_17_safe(SEALED_FILE_REGISTRATION_KEY, true) {
        return e;
    }
    if let Err(e) = migrate_file_from_2_17_safe(SEALED_FILE_ENCRYPTED_SEED_KEY_GENESIS, true) {
        return e;
    }
    if let Err(e) = migrate_file_from_2_17_safe(SEALED_FILE_ENCRYPTED_SEED_KEY_CURRENT, true) {
        return e;
    }
    if let Err(e) = migrate_file_from_2_17_safe(SEALED_FILE_REK, true) {
        return e;
    }
    if let Err(e) = migrate_file_from_2_17_safe(SEALED_FILE_IRS, true) {
        return e;
    }
    if let Err(e) = migrate_file_from_2_17_safe(SEALED_FILE_VALIDATOR_SET, true) {
        return e;
    }
    if let Err(e) = migrate_file_from_2_17_safe(SEALED_FILE_TX_BYTES, true) {
        return e;
    }
    sgx_status_t::SGX_SUCCESS
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

/////////////////
// Contract storage recoding
fn get_next_chunk(p_buf: *mut u8, n_buf: usize, offset: *mut usize) -> SgxResult<(*mut u8, usize)> {
    unsafe {
        // Ensure we have at least 4 bytes left for the chunk size
        if *offset + 4 > n_buf {
            warn!("Buffer underflow: Not enough bytes for next chunk size");
            return Err(sgx_status_t::SGX_ERROR_INVALID_PARAMETER);
        }

        // Read chunk size (first 4 bytes as a native-endian u32)
        let size_bytes = ptr::slice_from_raw_parts_mut(p_buf.add(*offset), 4);
        let chunk_size = u32::from_ne_bytes(*(size_bytes as *const [u8; 4])) as usize;
        *offset += 4;

        // Ensure the chunk fits within the buffer
        if *offset + chunk_size > n_buf {
            warn!("Buffer underflow: Declared chunk size exceeds remaining buffer");
            return Err(sgx_status_t::SGX_ERROR_INVALID_PARAMETER);
        }

        // Return a raw pointer to the chunk memory
        let chunk_ptr = p_buf.add(*offset);
        *offset += chunk_size;

        Ok((chunk_ptr, chunk_size))
    }
}

fn get_next_chunk64(p_buf: *mut u8, n_buf: usize, offset: *mut usize) -> (*mut u8, usize) {
    unsafe {
        // Ensure we have at least 8 bytes left for the chunk size
        if *offset + 8 > n_buf {
            return (null_mut(), 0);
        }

        // Read chunk size (first 8 bytes as a native-endian u32)
        let size_bytes = ptr::slice_from_raw_parts_mut(p_buf.add(*offset), 8);
        let chunk_size = u64::from_ne_bytes(*(size_bytes as *const [u8; 8])) as usize;
        *offset += 8;

        // Ensure the chunk fits within the buffer
        if *offset + chunk_size > n_buf {
            return (null_mut(), 0);
        }

        // Return a raw pointer to the chunk memory
        let chunk_ptr = p_buf.add(*offset);
        *offset += chunk_size;

        (chunk_ptr, chunk_size)
    }
}

#[repr(C, packed)]
#[derive(Debug)]
struct KeyVer2 {
    magic_prefix: u8, // must be 3
    contract_key: [u8; 20],
    magic_1: u64,     // must be 6
    magic_2: [u8; 6], // must be b"secret"
    seed_version: u16,
    encoding_version: u32,
    data_size: u64,
}

/// Rotates the store buffer, replacing the current IKM with a new one.
///
/// # Safety
///
/// - `p_buf` must be valid for reads and writes for `n_buf` bytes.
/// - The caller must ensure that `p_buf` is properly aligned.
/// - The function assumes that `ikm_current` and `ikm_new` point to valid keys.
/// - `num_recoded` must be a valid, mutable pointer.
///
/// Calling this with invalid pointers or buffer lengths may lead to undefined behavior.
pub unsafe fn rotate_store(
    p_buf: *mut u8,
    n_buf: usize,
    ikm_current: &AESKey,
    ikm_next: &AESKey,
    num_total: &mut u32,
    num_recoded: &mut u32,
) -> SgxResult<()> {
    //println!("******* rotate_store **********");

    let mut offset: usize = 0;

    let (og_key_p, og_key_n) = get_next_chunk(p_buf, n_buf, &mut offset)?;
    let og_key = slice::from_raw_parts(og_key_p, og_key_n);
    //println!("og_key = {}", hex::encode(og_key));

    let symm_key_current = ikm_current.derive_key_from_this(og_key);
    let symm_key_next = ikm_next.derive_key_from_this(og_key);

    while offset < n_buf {
        *num_total += 1;
        let (key_p, key_n) = get_next_chunk(p_buf, n_buf, &mut offset)?;
        let (val_p, val_n) = get_next_chunk(p_buf, n_buf, &mut offset)?;

        // println!(" key = {}", hex::encode(slice::from_raw_parts(key_p, key_n)));
        // println!(" val_2 = {}", hex::encode(slice::from_raw_parts(val_p, val_n)));

        if key_n >= std::mem::size_of::<KeyVer2>() {
            let key_parsed: *mut KeyVer2 = key_p as *mut KeyVer2;
            if (key_n - std::mem::size_of::<KeyVer2>() == (*key_parsed).data_size as usize)
                && ((*key_parsed).magic_prefix == 3)
                && ((*key_parsed).magic_1 == 6)
            {
                //println!(" contract_key: {}", hex::encode((*key_parsed).contract_key));

                let mut offs_val: usize = 0;
                let (salt_p, salt_n) = get_next_chunk64(val_p, val_n, &mut offs_val);
                let (data_p, data_n) = get_next_chunk64(val_p, val_n, &mut offs_val);

                if (data_n > 0) && (salt_n > 0) {
                    let encryption_salt = slice::from_raw_parts(salt_p, salt_n);
                    let encrypted_val2 = slice::from_raw_parts_mut(data_p, data_n);

                    // println!("   salt = {}", hex::encode(encryption_salt));
                    // println!("   data_2 = {}", hex::encode(&encrypted_val2));

                    let encrypted_key = slice::from_raw_parts(
                        key_p.add(std::mem::size_of::<KeyVer2>()),
                        key_n - std::mem::size_of::<KeyVer2>(),
                    );
                    // println!("   trying encrypted_key = {}", hex::encode(encrypted_key));

                    let result = symm_key_current
                        .decrypt_siv(encrypted_val2, Some(&[encrypted_key, encryption_salt]));
                    if let Ok(data_plain) = result {
                        // println!("   data_plain = {}", hex::encode(&data_plain));

                        // re-encode it with the new key. Leave the salt intact
                        if let Ok(encrypted_val3) = symm_key_next
                            .encrypt_siv(&data_plain, Some(&[encrypted_key, encryption_salt]))
                        {
                            // println!("   data_3 = {}", hex::encode(&encrypted_val3));
                            if encrypted_val3.len() == encrypted_val2.len() {
                                encrypted_val2.copy_from_slice(&encrypted_val3);
                                *num_recoded += 1;
                            }
                        }
                    }
                }
            }
        }
    }

    Ok(())
}
