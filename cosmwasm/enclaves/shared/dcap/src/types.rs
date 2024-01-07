// use attestation::sgx_quote::SgxEcdsaQuoteAkType;
// use sgx_types::{
//     sgx_ql_auth_data_t, sgx_ql_certification_data_t, sgx_ql_ecdsa_sig_data_t, sgx_quote3_t,
//     sgx_quote_header_t,
// };
// use std::borrow::Cow;
// use std::mem;
//
// use attestation::sgx_report::SgxEnclaveReport;
//
// // #[cfg_attr(feature = "serde", derive(Serialize, Deserialize))]
// pub struct DcapQuote {
//     header: DcapQuoteHeader,
//     report_body: SgxEnclaveReport,
//     signature: DcapSignatureEcdsaP256,
// }
//
// // #[cfg_attr(feature = "serde", derive(Serialize, Deserialize))]
// pub enum DcapQuoteHeader {
//     V3 {
//         attestation_key_type: SgxEcdsaQuoteAkType,
//         qe3_svn: u16,
//         pce_svn: u16,
//         qe3_vendor_id: Vec<u8>,
//         user_data: Vec<u8>,
//     },
// }
//
// impl DcapQuoteHeader {
//     pub fn from_bytes(bytes: &[u8]) {
//         let attestation_key_type = bytes[0];
//         let qe3_svn = u16::from_le_bytes(bytes[1..2]);
//     }
// }
//
// pub const QE3_VENDOR_ID_INTEL: [u8; 16] = [
//     0x93, 0x9a, 0x72, 0x33, 0xf7, 0x9c, 0x4c, 0xa9, 0x94, 0x0a, 0x0d, 0xb3, 0x95, 0x7f, 0x06, 0x07,
// ];
//
// // pub type QeId<'a> = Cow<'a, [u8]>;
//
// pub struct DcapSignatureEcdsaP256 {
//     signature: [u8; 64],
//     attestation_public_key: [u8; 64],
//     qe3_report: SgxEnclaveReport,
//     qe3_signature: [u8; 64],
//     authentication_data: Vec<u8>,
//     certification_data_type: CertificationDataType,
//     certification_data: Vec<u8>,
// }
//
// impl DcapSignatureEcdsaP256 {
//     fn from_slice(bytes: &[u8]) -> Result<Self> {
//         DcapSignatureEcdsaP256 {}
//     }
// }
//
// #[repr(u16)]
// #[derive(Debug, Copy, Clone, PartialEq, Eq, Hash, FromPrimitive, ToPrimitive)]
// pub enum CertificationDataType {
//     PpidCleartext = 1,
//     PpidEncryptedRsa2048 = 2,
//     PpidEncryptedRsa3072 = 3,
//     PckCertificate = 4,
//     PckCertificateChain = 5,
//     EcdsaSignatureAuxiliaryData = 6,
//     PlatformManifest = 7,
// }
//
// #[derive(Clone, Debug, Hash, PartialEq, Eq)]
// pub struct Qe3CertDataPpid {
//     pub ppid: Vec<u8>,
//     pub cpusvn: Vec<u8>,
//     pub pcesvn: u16,
//     pub pceid: u16,
// }
//
// #[derive(Clone, Debug, Hash, PartialEq, Eq)]
// pub struct Qe3CertDataPckCertChain {
//     pub certs: Vec<Vec<u8>>,
// }
//
// pub type RawQe3CertData = Vec<u8>;
//
// pub type Result<T> = ::std::result::Result<T, ::failure::Error>;
//
// const ECDSA_P256_SIGNATURE_LEN: usize = 64;
// const ECDSA_P256_PUBLIC_KEY_LEN: usize = 64;
// const QE3_VENDOR_ID_LEN: usize = 16;
// const QE3_USER_DATA_LEN: usize = 20;
// const REPORT_BODY_LEN: usize = 384;
// const CPUSVN_LEN: usize = 16;
// const QUOTE_VERSION_3: u16 = 3;
//
// impl DcapQuote {
//     pub fn parse<T: Into<Vec<u8>>>(quote: T) -> Result<DcapQuote> {
//         let mut quote = quote.into();
//         let p_quote3: *const sgx_quote3_t = quote.as_ptr() as *const sgx_quote3_t;
//
//         // let quote_signature = DcapSignatureEcdsaP256 {
//         //     signature: q,
//         //     attestation_public_key: [],
//         //     qe3_report: ,
//         //     qe3_signature: [],
//         //     authentication_data: vec![],
//         //     certification_data_type: CertificationDataType::PpidCleartext,
//         //     certification_data: vec![]
//         // }
//
//         // // copy heading bytes to a sgx_quote3_t type to simplify access
//         let quote3: sgx_quote3_t = unsafe { *p_quote3 };
//
//         let quote_signature_data_vec: Vec<u8> = quote[std::mem::size_of::<sgx_quote3_t>()..].into();
//
//         println!(
//             "quote3 header says signature data len = {}",
//             quote3.signature_data_len
//         );
//         println!(
//             "quote_signature_data len = {}",
//             quote_signature_data_vec.len()
//         );
//
//         assert_eq!(
//             quote3.signature_data_len as usize,
//             quote_signature_data_vec.len()
//         );
//
//         // signature_data has a header of sgx_ql_ecdsa_sig_data_t structure
//         let p_sig_data: *const sgx_ql_ecdsa_sig_data_t = quote_signature_data_vec.as_ptr() as _;
//         // mem copy
//         let sig_data = unsafe { *p_sig_data };
//
//         // sgx_ql_ecdsa_sig_data_t is followed by sgx_ql_auth_data_t
//         // create a new vec for auth_data
//         let auth_certification_data_offset = std::mem::size_of::<sgx_ql_ecdsa_sig_data_t>();
//         let p_auth_data: *const sgx_ql_auth_data_t =
//             (quote_signature_data_vec[auth_certification_data_offset..]).as_ptr() as _;
//         let auth_data_header: sgx_ql_auth_data_t = unsafe { *p_auth_data };
//         println!("auth_data len = {}", auth_data_header.size);
//
//         let auth_data_offset =
//             auth_certification_data_offset + std::mem::size_of::<sgx_ql_auth_data_t>();
//
//         // It should be [0,1,2,3...]
//         // defined at https://github.com/intel/SGXDataCenterAttestationPrimitives/blob/4605fae1c606de4ff1191719433f77f050f1c33c/QuoteGeneration/quote_wrapper/quote/qe_logic.cpp#L1452
//         let auth_data_vec: Vec<u8> = quote_signature_data_vec
//             [auth_data_offset..auth_data_offset + auth_data_header.size as usize]
//             .into();
//         println!("Auth data:\n{:?}", auth_data_vec);
//
//         let temp_cert_data_offset = auth_data_offset + auth_data_header.size as usize;
//         let p_temp_cert_data: *const sgx_ql_certification_data_t =
//             quote_signature_data_vec[temp_cert_data_offset..].as_ptr() as _;
//         let temp_cert_data: sgx_ql_certification_data_t = unsafe { *p_temp_cert_data };
//
//         println!("certification data offset = {}", temp_cert_data_offset);
//         println!("certification data size = {}", temp_cert_data.size);
//
//         let cert_info_offset =
//             temp_cert_data_offset + std::mem::size_of::<sgx_ql_certification_data_t>();
//
//         println!("cert info offset = {}", cert_info_offset);
//         // this should be the last structure
//         assert_eq!(
//             quote_signature_data_vec.len(),
//             cert_info_offset + temp_cert_data.size as usize
//         );
//
//         let tail_content = quote_signature_data_vec[cert_info_offset..].to_vec();
//         let enc_ppid_len = 384;
//         let enc_ppid: &[u8] = &tail_content[0..enc_ppid_len];
//         let pce_id: &[u8] = &tail_content[enc_ppid_len..enc_ppid_len + 2];
//         let cpu_svn: &[u8] = &tail_content[enc_ppid_len + 2..enc_ppid_len + 2 + 16];
//         let pce_isvsvn: &[u8] = &tail_content[enc_ppid_len + 2 + 16..enc_ppid_len + 2 + 18];
//         println!("EncPPID:\n{:02x}", enc_ppid.iter().format(""));
//         println!("PCE_ID:\n{:02x}", pce_id.iter().format(""));
//         println!("TCBr - CPUSVN:\n{:02x}", cpu_svn.iter().format(""));
//         println!("TCBr - PCE_ISVSVN:\n{:02x}", pce_isvsvn.iter().format(""));
//         println!("QE_ID:\n{:02x}", quote3.header.user_data.iter().format(""));
//
//         // Ok(DcapQuote {
//         //     header: DcapQuoteHeader::V3 {
//         //         attestation_key_type,
//         //         qe3_svn,
//         //         pce_svn,
//         //         qe3_vendor_id,
//         //         user_data,
//         //     },
//         //     report_body,
//         //     signature,
//         // })
//     }
// }
