pub enum Error {
    GenericError,
}

pub fn extract_asn1_value(cert: &[u8], oid: &[u8]) -> Result<Vec<u8>, Error> {
    let mut offset = match cert.windows(oid.len()).position(|window| window == oid) {
        Some(size) => size,
        None => {
            return Err(Error::GenericError);
        }
    };

    offset += 12; // 11 + TAG (0x04)

    if offset + 2 >= cert.len() {
        return Err(Error::GenericError);
    }

    // Obtain Netscape Comment length
    let mut len = cert[offset] as usize;
    if len > 0x80 {
        len = (cert[offset + 1] as usize) * 0x100 + (cert[offset + 2] as usize);
        offset += 2;
    }

    // Obtain Netscape Comment
    offset += 1;

    if offset + len >= cert.len() {
        return Err(Error::GenericError);
    }

    let payload = cert[offset..offset + len].to_vec();

    Ok(payload)
}

#[cfg(all(feature = "SGX_MODE_HW", feature = "production", not(feature = "test")))]
#[allow(dead_code)]
const WHITELIST_FROM_FILE: &str = include_str!("../../whitelist.txt");

#[cfg(all(
    not(all(feature = "SGX_MODE_HW", feature = "production")),
    feature = "test"
))]
const WHITELIST_FROM_FILE: &str = include_str!("fixtures/test_whitelist.txt");

#[cfg(feature = "test")]
pub mod tests {
    use std::io::Read;
    use std::untrusted::fs::File;

    use enclave_ffi_types::NodeAuthResult;

    // #[cfg(feature = "SGX_MODE_HW")]
    // fn tls_ra_cert_der_out_of_date() -> Vec<u8> {
    //     let mut cert = vec![];
    //     let mut f =
    //         File::open("../execute/src/registration/fixtures/attestation_cert_out_of_date.der")
    //             .unwrap();
    //     f.read_to_end(&mut cert).unwrap();
    //
    //     cert
    // }

    fn tls_ra_cert_der_sw_config_needed() -> Vec<u8> {
        let mut cert = vec![];
        let mut f = File::open(
            "../execute/src/registration/fixtures/attestation_cert_sw_config_needed.der",
        )
        .unwrap();
        f.read_to_end(&mut cert).unwrap();

        cert
    }

    #[cfg(feature = "SGX_MODE_HW")]
    fn tls_ra_cert_der_valid() -> Vec<u8> {
        let mut cert = vec![];
        let mut f =
            File::open("../execute/src/registration/fixtures/attestation_cert_hw_v2").unwrap();
        f.read_to_end(&mut cert).unwrap();

        cert
    }

    #[cfg(not(feature = "SGX_MODE_HW"))]
    fn tls_ra_cert_der_valid() -> Vec<u8> {
        let mut cert = vec![];
        let mut f = File::open("../execute/src/registration/fixtures/attestation_cert_sw").unwrap();
        f.read_to_end(&mut cert).unwrap();

        cert
    }

    #[cfg(not(feature = "SGX_MODE_HW"))]
    pub fn test_certificate_invalid_configuration_needed() {}
}
