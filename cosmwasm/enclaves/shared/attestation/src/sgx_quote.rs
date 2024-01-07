use enclave_ffi_types::NodeAuthResult;
use log::error;

/// SGX Quote structure version
#[derive(Debug, PartialEq)]
#[allow(dead_code)]
pub enum SgxQuoteVersion {
    /// EPID quote version
    V1(SgxEpidQuoteSigType),
    /// EPID quote version
    V2(SgxEpidQuoteSigType),
    /// ECDSA quote version
    V3(SgxEcdsaQuoteAkType),
}

/// Intel EPID attestation signature type
#[derive(Debug, PartialEq)]
#[allow(dead_code)]
pub enum SgxEpidQuoteSigType {
    Unlinkable,
    Linkable,
}

/// ECDSA attestation key type
#[derive(Debug, PartialEq)]
pub enum SgxEcdsaQuoteAkType {
    /// ECDSA-256-with-P-256 curve
    P256_256,
    /// ECDSA-384-with-P-384 curve
    P384_384,
}

/// SGX Quote status
#[derive(PartialEq, Debug)]
pub enum SgxQuoteStatus {
    /// EPID signature of the ISV enclave QUOTE was verified correctly and the
    /// TCB level of the SGX platform is up-to-date.
    OK,
    /// EPID signature of the ISV enclave QUOTE was invalid. The content of the
    /// QUOTE is not trustworthy.
    ///
    /// For DCAP, the signature over the application report is invalid.
    SignatureInvalid,
    /// The EPID group has been revoked. When this value is returned, the
    /// revocation Reason field of the Attestation Verification Report will
    /// contain revocation reason code for this EPID group as reported in the
    /// EPID Group CRL. The content of the QUOTE is not trustworthy.
    GroupRevoked,
    /// The EPID private key used to sign the QUOTE has been revoked by
    /// signature. The content of the QUOTE is not trustworthy.
    SignatureRevoked,
    /// The EPID private key used to sign the QUOTE has been directly revoked
    /// (not by signature). The content of the QUOTE is not trustworthy.
    ///
    /// For DCAP, the attestation key or platform has been revoked.
    KeyRevoked,
    /// SigRL version in ISV enclave QUOTE does not match the most recent
    /// version of the SigRL. In rare situations, after SP retrieved the SigRL
    /// from IAS and provided it to the platform, a newer version of the SigRL
    /// is madeavailable. As a result, the Attestation Verification Report will
    /// indicate SIGRL_VERSION_MISMATCH. SP can retrieve the most recent version
    /// of SigRL from the IAS and request the platform to perform remote
    /// attestation again with the most recent version of SigRL. If the platform
    /// keeps failing to provide a valid QUOTE matching with the most recent
    /// version of the SigRL, the content of the QUOTE is not trustworthy.
    SigrlVersionMismatch,
    /// The EPID signature of the ISV enclave QUOTE has been verified correctly,
    /// but the TCB level of SGX platform is outdated (for further details see
    /// Advisory IDs). The platform has not been identified as compromised and
    /// thus it is not revoked. It is up to the Service Provider to decide
    /// whether or not to trust the content of the QUOTE, andwhether or not to
    /// trust the platform performing the attestation to protect specific
    /// sensitive information.
    GroupOutOfDate,
    /// The EPID signature of the ISV enclave QUOTE has been verified correctly,
    /// but additional configuration of SGX platform may be needed(for further
    /// details see Advisory IDs). The platform has not been identified as
    /// compromised and thus it is not revoked. It is up to the Service Provider
    /// to decide whether or not to trust the content of the QUOTE, and whether
    /// or not to trust the platform performing the attestation to protect
    /// specific sensitive information.
    ///
    /// For DCAP, The Quote verification passed and the platform is patched to
    /// the latest TCB level but additional configuration of the SGX
    /// platform may be needed.
    ConfigurationNeeded,
    /// The EPID signature of the ISV enclave QUOTE has been verified correctly
    /// but due to certain issues affecting the platform, additional SW
    /// Hardening in the attesting SGX enclaves may be needed.The relying party
    /// should evaluate the potential risk of an attack leveraging the relevant
    /// issues on the attesting enclave, and whether the attesting enclave
    /// employs adequate software hardening to mitigate the risk.
    SwHardeningNeeded,
    /// The EPID signature of the ISV enclave QUOTE has been verified correctly
    /// but additional configuration for the platform and SW Hardening in the
    /// attesting SGX enclaves may be needed. The platform has not been
    /// identified as compromised and thus it is not revoked. It is up to the
    /// Service Provider to decide whether or not to trust the content of the
    /// QUOTE. The relying party should also evaluate the potential risk of an
    /// attack leveraging the relevant issues on the attestation enclave, and
    /// whether the attesting enclave employs adequate software hardening to
    /// mitigate the risk.
    ConfigurationAndSwHardeningNeeded,
    /// DCAP specific quote status. The Quote is good but TCB level of the
    /// platform is out of date. The platform needs patching to be at the latest
    /// TCB level.
    OutOfDate,
    /// DCAP specific quote status. The Quote is good but the TCB level of the
    /// platform is out of date and additional configuration of the SGX Platform
    /// at its current patching level may be needed. The platform needs patching
    /// to be at the latest TCB level.
    OutOfDateConfigurationNeeded,
    /// Other unknown bad status.
    UnknownBadStatus,
}

impl From<&SgxQuoteStatus> for NodeAuthResult {
    fn from(status: &SgxQuoteStatus) -> Self {
        match status {
            SgxQuoteStatus::ConfigurationAndSwHardeningNeeded => {
                NodeAuthResult::SwHardeningAndConfigurationNeeded
            }
            SgxQuoteStatus::ConfigurationNeeded => NodeAuthResult::ConfigurationNeeded,
            SgxQuoteStatus::GroupOutOfDate => NodeAuthResult::GroupOutOfDate,
            SgxQuoteStatus::KeyRevoked => NodeAuthResult::KeyRevoked,
            SgxQuoteStatus::SigrlVersionMismatch => NodeAuthResult::SigrlVersionMismatch,
            SgxQuoteStatus::SignatureRevoked => NodeAuthResult::SignatureRevoked,
            SgxQuoteStatus::GroupRevoked => NodeAuthResult::GroupRevoked,
            _ => NodeAuthResult::BadQuoteStatus,
        }
    }
}

impl From<&str> for SgxQuoteStatus {
    /// Convert from str status from the report to enum.
    fn from(status: &str) -> Self {
        match status {
            "OK" => SgxQuoteStatus::OK,
            "SIGNATURE_INVALID" => SgxQuoteStatus::SignatureInvalid,
            "GROUP_REVOKED" => SgxQuoteStatus::GroupRevoked,
            "SIGNATURE_REVOKED" => SgxQuoteStatus::SignatureRevoked,
            "KEY_REVOKED" => SgxQuoteStatus::KeyRevoked,
            "SIGRL_VERSION_MISMATCH" => SgxQuoteStatus::SigrlVersionMismatch,
            "GROUP_OUT_OF_DATE" => SgxQuoteStatus::GroupOutOfDate,
            "OUT_OF_DATE" => SgxQuoteStatus::OutOfDate,
            "OUT_OF_DATE_CONFIGURATION_NEEDED" => SgxQuoteStatus::OutOfDateConfigurationNeeded,
            "CONFIGURATION_NEEDED" => SgxQuoteStatus::ConfigurationNeeded,
            "SW_HARDENING_NEEDED" => SgxQuoteStatus::SwHardeningNeeded,
            "CONFIGURATION_AND_SW_HARDENING_NEEDED" => {
                SgxQuoteStatus::ConfigurationAndSwHardeningNeeded
            }
            _ => SgxQuoteStatus::UnknownBadStatus,
        }
    }
}

type SignatureAlgorithms = &'static [&'static webpki::SignatureAlgorithm];
pub static SUPPORTED_SIG_ALGS: SignatureAlgorithms = &[
    &webpki::ECDSA_P256_SHA256,
    &webpki::ECDSA_P256_SHA384,
    &webpki::ECDSA_P384_SHA256,
    &webpki::ECDSA_P384_SHA384,
    &webpki::RSA_PSS_2048_8192_SHA256_LEGACY_KEY,
    &webpki::RSA_PSS_2048_8192_SHA384_LEGACY_KEY,
    &webpki::RSA_PSS_2048_8192_SHA512_LEGACY_KEY,
    &webpki::RSA_PKCS1_2048_8192_SHA256,
    &webpki::RSA_PKCS1_2048_8192_SHA384,
    &webpki::RSA_PKCS1_2048_8192_SHA512,
    &webpki::RSA_PKCS1_3072_8192_SHA384,
];

pub fn check_advisories(
    quote_status: &SgxQuoteStatus,
    advisories: &crate::sgx_report::AdvisoryIDs,
) -> Result<(), NodeAuthResult> {
    // this checks if there are any vulnerabilities that are not on in the whitelisted list
    let vulnerable = advisories.vulnerable();
    if vulnerable.is_empty() {
        Ok(())
    } else {
        error!("Platform is updated but requires further BIOS configuration");
        error!(
            "The following vulnerabilities must be mitigated: {:?}",
            vulnerable
        );
        Err(NodeAuthResult::from(quote_status))
    }
}
