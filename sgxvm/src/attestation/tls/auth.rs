use std::prelude::v1::*;

pub struct ClientAuth {
    outdated_ok: bool,
    is_dcap: bool,
}

impl ClientAuth {
    pub fn new(outdated_ok: bool, is_dcap: bool) -> ClientAuth {
        ClientAuth {
            outdated_ok,
            is_dcap,
        }
    }
}

#[cfg(all(feature = "hardware_mode", not(feature = "mainnet")))]
impl rustls::ClientCertVerifier for ClientAuth {
    fn client_auth_root_subjects(
        &self,
        _sni: Option<&webpki::DNSName>,
    ) -> Option<rustls::DistinguishedNames> {
        Some(rustls::DistinguishedNames::new())
    }

    fn verify_client_cert(
        &self,
        certs: &[rustls::Certificate],
        _sni: Option<&webpki::DNSName>,
    ) -> Result<rustls::ClientCertVerified, rustls::TLSError> {
        if certs.is_empty() {
            println!("[Enclave] No certs provided for Client Auth");
            return Err(rustls::TLSError::NoCertificatesPresented);
        }

        if self.is_dcap {
            crate::attestation::cert::verify_dcap_cert(&certs[0].0).map_err(|err| {
                println!("[Attestastion Server] Cannot verify DCAP cert. Reason: {:?}", err);
                rustls::TLSError::WebPKIError(
                    webpki::Error::ExtensionValueInvalid,
                )
            })?;
            return Ok(rustls::ClientCertVerified::assertion());
        }

        // This call will automatically verify cert is properly signed
        match crate::attestation::cert::verify_ra_cert(&certs[0].0, None) {
            Ok(_) => Ok(rustls::ClientCertVerified::assertion()),
            Err(crate::attestation::types::AuthResult::SwHardeningAndConfigurationNeeded)
            | Err(crate::attestation::types::AuthResult::GroupOutOfDate) => {
                if self.outdated_ok {
                    println!("outdated_ok is set, overriding outdated error");
                    Ok(rustls::ClientCertVerified::assertion())
                } else {
                    Err(rustls::TLSError::WebPKIError(
                        webpki::Error::ExtensionValueInvalid,
                    ))
                }
            }
            Err(_) => Err(rustls::TLSError::WebPKIError(
                webpki::Error::ExtensionValueInvalid,
            )),
        }
    }
}

#[cfg(all(feature = "hardware_mode", feature = "mainnet"))]
impl rustls::ClientCertVerifier for ClientAuth {
    fn client_auth_root_subjects(
        &self,
        _sni: Option<&webpki::DNSName>,
    ) -> Option<rustls::DistinguishedNames> {
        Some(rustls::DistinguishedNames::new())
    }

    fn verify_client_cert(
        &self,
        certs: &[rustls::Certificate],
        _sni: Option<&webpki::DNSName>,
    ) -> Result<rustls::ClientCertVerified, rustls::TLSError> {
        if certs.is_empty() {
            println!("[Enclave] No certs provided for Client Auth");
            return Err(rustls::TLSError::NoCertificatesPresented);
        }

        if self.is_dcap {
            crate::attestation::cert::verify_dcap_cert(&certs[0].0).unwrap();
            return Ok(rustls::ClientCertVerified::assertion());
        }

        match crate::attestation::cert::verify_ra_cert(&certs[0].0, None) {
            Ok(_) => Ok(rustls::ClientCertVerified::assertion()),
            Err(crate::attestation::types::AuthResult::SwHardeningAndConfigurationNeeded) => {
                if self.outdated_ok {
                    println!("outdated_ok is set, overriding outdated error");
                    Ok(rustls::ClientCertVerified::assertion())
                } else {
                    Err(rustls::TLSError::WebPKIError(
                        webpki::Error::ExtensionValueInvalid,
                    ))
                }
            }
            Err(_) => Err(rustls::TLSError::WebPKIError(
                webpki::Error::ExtensionValueInvalid,
            )),
        }
    }
}

pub struct ServerAuth {
    outdated_ok: bool,
    is_dcap: bool,
}

impl ServerAuth {
    pub fn new(outdated_ok: bool, is_dcap: bool) -> ServerAuth {
        ServerAuth {
            outdated_ok,
            is_dcap,
        }
    }
}

#[cfg(all(feature = "hardware_mode", feature = "mainnet"))]
impl rustls::ServerCertVerifier for ServerAuth {
    fn verify_server_cert(
        &self,
        _roots: &rustls::RootCertStore,
        certs: &[rustls::Certificate],
        _hostname: webpki::DNSNameRef,
        _ocsp: &[u8],
    ) -> Result<rustls::ServerCertVerified, rustls::TLSError> {
        if certs.is_empty() {
            println!("[Enclave] No certs provided for Client Auth");
            return Err(rustls::TLSError::NoCertificatesPresented);
        }

        if self.is_dcap {
            crate::attestation::cert::verify_dcap_cert(&certs[0].0).unwrap();
            return Ok(rustls::ServerCertVerified::assertion());
        }

        let res = crate::attestation::cert::verify_ra_cert(&certs[0].0, None);
        match res {
            Ok(_) => Ok(rustls::ServerCertVerified::assertion()),
            Err(crate::attestation::types::AuthResult::SwHardeningAndConfigurationNeeded) => {
                if self.outdated_ok {
                    println!("outdated_ok is set, overriding outdated error");
                    Ok(rustls::ServerCertVerified::assertion())
                } else {
                    Err(rustls::TLSError::WebPKIError(
                        webpki::Error::ExtensionValueInvalid,
                    ))
                }
            }
            Err(_) => Err(rustls::TLSError::WebPKIError(
                webpki::Error::ExtensionValueInvalid,
            )),
        }
    }
}

#[cfg(all(feature = "hardware_mode", not(feature = "mainnet")))]
impl rustls::ServerCertVerifier for ServerAuth {
    fn verify_server_cert(
        &self,
        _roots: &rustls::RootCertStore,
        certs: &[rustls::Certificate],
        _hostname: webpki::DNSNameRef,
        _ocsp: &[u8],
    ) -> Result<rustls::ServerCertVerified, rustls::TLSError> {
        if certs.is_empty() {
            println!("[Enclave] No certs provided for Server Auth");
            return Err(rustls::TLSError::NoCertificatesPresented);
        }

        if self.is_dcap {
            crate::attestation::cert::verify_dcap_cert(&certs[0].0).unwrap();
            return Ok(rustls::ServerCertVerified::assertion());
        }
        
        // This call will automatically verify cert is properly signed
        let res = crate::attestation::cert::verify_ra_cert(&certs[0].0, None);
        match res {
            Ok(_) => Ok(rustls::ServerCertVerified::assertion()),
            Err(crate::attestation::types::AuthResult::SwHardeningAndConfigurationNeeded)
            | Err(crate::attestation::types::AuthResult::GroupOutOfDate) => {
                if self.outdated_ok {
                    println!("outdated_ok is set, overriding outdated error");
                    Ok(rustls::ServerCertVerified::assertion())
                } else {
                    Err(rustls::TLSError::WebPKIError(
                        webpki::Error::ExtensionValueInvalid,
                    ))
                }
            }
            Err(_) => Err(rustls::TLSError::WebPKIError(
                webpki::Error::ExtensionValueInvalid,
            )),
        }
    }
}
