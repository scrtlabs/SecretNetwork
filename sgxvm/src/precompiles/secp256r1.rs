extern crate sgx_tstd as std;

use std::vec::Vec;
use core::cmp::min;
use p256::ecdsa::{signature::hazmat::PrehashVerifier, Signature, VerifyingKey};

use crate::precompiles::{
    ExitError,
    ExitSucceed,
    LinearCostPrecompile,
    PrecompileFailure,
};

pub struct P256Verify;

impl LinearCostPrecompile for P256Verify {
    const BASE: u64 = 3450;
    const WORD: u64 = 0;

    fn raw_execute(i: &[u8], target_gas: u64) -> Result<(ExitSucceed, Vec<u8>), PrecompileFailure> {
        if i.len() < 160 {
            return Err(PrecompileFailure::Error {
                exit_status: ExitError::Other("input must contain 160 bytes".into()),
            });
        };
        const P256VERIFY_BASE: u64 = 3_450;

        if P256VERIFY_BASE > target_gas {
            return Err(PrecompileFailure::Error {
                exit_status: ExitError::OutOfGas,
            });
        }
        let mut input = [0u8; 160];
        input[..min(i.len(), 160)].copy_from_slice(&i[..min(i.len(), 160)]);

        // msg signed (msg is already the hash of the original message)
        let msg: [u8; 32] = input[..32].try_into().unwrap();
        // r, s: signature
        let sig: [u8; 64] = input[32..96].try_into().unwrap();
        // x, y: public key
        let pk: [u8; 64] = input[96..160].try_into().unwrap();
        // append 0x04 to the public key: uncompressed form
        let mut uncompressed_pk = [0u8; 65];
        uncompressed_pk[0] = 0x04;
        uncompressed_pk[1..].copy_from_slice(&pk);

        let public_key = VerifyingKey::from_sec1_bytes(&uncompressed_pk).map_err(|_| PrecompileFailure::Error {
            exit_status: ExitError::Other("Public key recover failed".into()),
        })?;
        let signature = Signature::from_slice(&sig).map_err(|_| PrecompileFailure::Error {
            exit_status: ExitError::Other("Signature recover failed".into()),
        })?;


        let mut buf = [0u8; 32];

        // verify
        if public_key.verify_prehash(&msg, &signature).is_ok() {
            buf[31] = 1u8;
        } else {
            buf[31] = 0u8;
        }
        Ok((ExitSucceed::Returned, buf.to_vec()))

    }
}
#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_empty_input() -> Result<(), PrecompileFailure> {
        let input: [u8; 0] = [];
        let cost: u64 = 1;

        match P256Verify::raw_execute(&input, cost) {
            Ok((_, _)) => {
                panic!("Test not expected to pass");
            }
            Err(e) => {
                assert_eq!(
                    e,
                    PrecompileFailure::Error {
                        exit_status: ExitError::Other("input must contain 160 bytes".into())
                    }
                );
                Ok(())
            }
        }
    }
    #[test]
    fn proper_sig_verify() {
        let input = hex::decode("4cee90eb86eaa050036147a12d49004b6b9c72bd725d39d4785011fe190f0b4da73bd4903f0ce3b639bbbf6e8e80d16931ff4bcf5993d58468e8fb19086e8cac36dbcd03009df8c59286b162af3bd7fcc0450c9aa81be5d10d312af6c66b1d604aebd3099c618202fcfe16ae7770b0c49ab5eadf74b754204a3bb6060e44eff37618b065f9832de4ca6ca971a7a1adc826d0f7c00181a5fb2ddf79ae00b4e10e").unwrap();
        let target_gas = 3_500u64;
        let (success, res) = P256Verify::raw_execute(&input, target_gas).unwrap();
        assert_eq!(success, ExitSucceed::Returned);
        assert_eq!(res.len(), 32);
        assert_eq!(res[0], 0u8);
        assert_eq!(res[1], 0u8);
        assert_eq!(res[2], 0u8);
        assert_eq!(res[31], 1u8);
    }
}