// Apache Teaclave (incubating)
// Copyright 2019-2020 The Apache Software Foundation
//
// This product includes software developed at
// The Apache Software Foundation (http://www.apache.org/).
//! Types that contain information about attestation report.
//! The implementation is based on Attestation Service API version 4.
//! https://api.trustedservices.intel.com/documents/sgx-attestation-api-spec.pdf

#[cfg(feature = "sgx")]
extern crate sgx_tse;
#[cfg(feature = "sgx")]
extern crate sgx_types;

pub mod sgx_quote;
pub mod sgx_report;

#[cfg(test)]
mod tests {
    #[test]
    fn it_works() {
        let result = 2 + 2;
        assert_eq!(result, 4);
    }
}
