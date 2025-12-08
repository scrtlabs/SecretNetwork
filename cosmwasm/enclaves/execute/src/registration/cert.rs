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

#[cfg(not(feature = "epid_whitelist_disabled"))]
pub fn check_epid_gid_is_whitelisted(epid_gid: &u32) -> bool {
    let decoded = base64::decode(WHITELIST_FROM_FILE.trim()).unwrap(); //will never fail since data is constant
    decoded.as_chunks::<4>().0.iter().any(|&arr| {
        if epid_gid == &u32::from_be_bytes(arr) {
            return true;
        }
        false
    })
}

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

    #[cfg(not(feature = "epid_whitelist_disabled"))]
    pub fn test_epid_whitelist() {
        // check that we parse this correctly
        let res = crate::registration::cert::check_epid_gid_is_whitelisted(&(0xc12 as u32));
        assert_eq!(res, true);

        // check that 2nd number works
        let res = crate::registration::cert::check_epid_gid_is_whitelisted(&(0x6942 as u32));
        assert_eq!(res, true);

        // check all kinds of failures that should return false
        let res = crate::registration::cert::check_epid_gid_is_whitelisted(&(0x0 as u32));
        assert_eq!(res, false);

        let res = crate::registration::cert::check_epid_gid_is_whitelisted(&(0x120c as u32));
        assert_eq!(res, false);

        let res = crate::registration::cert::check_epid_gid_is_whitelisted(&(0xc120000 as u32));
        assert_eq!(res, false);

        let res = crate::registration::cert::check_epid_gid_is_whitelisted(&(0x1242 as u32));
        assert_eq!(res, false);
    }
}
