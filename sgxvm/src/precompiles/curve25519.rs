use curve25519_dalek::{
    ristretto::{CompressedRistretto, RistrettoPoint},
    scalar::Scalar,
    traits::Identity,
};
use ed25519_dalek::{Signature, Verifier, VerifyingKey};
use std::vec::Vec;

use crate::precompiles::{
    ExitError,
    ExitSucceed,
    LinearCostPrecompile,
    PrecompileFailure,
};

pub struct Ed25519Verify;

impl LinearCostPrecompile for Ed25519Verify {
    const BASE: u64 = 15;
    const WORD: u64 = 3;

    fn raw_execute(input: &[u8], _: u64) -> Result<(ExitSucceed, Vec<u8>), PrecompileFailure> {
        if input.len() < 128 {
            return Err(PrecompileFailure::Error {
                exit_status: ExitError::Other("input must contain 128 bytes".into()),
            });
        };

        let mut i = [0u8; 128];
        i[..128].copy_from_slice(&input[..128]);

        let mut buf = [0u8; 32];

        let msg = &i[0..32];
        let pk = VerifyingKey::try_from(&i[32..64]).map_err(|_| PrecompileFailure::Error {
            exit_status: ExitError::Other("Public key recover failed".into()),
        })?;
        let sig = Signature::try_from(&i[64..128]).map_err(|_| PrecompileFailure::Error {
            exit_status: ExitError::Other("Signature recover failed".into()),
        })?;

        // https://docs.rs/rust-crypto/0.2.36/crypto/ed25519/fn.verify.html
        if pk.verify(msg, &sig).is_ok() {
            buf[31] = 0u8;
        } else {
            buf[31] = 1u8;
        };

        Ok((ExitSucceed::Returned, buf.to_vec()))
    }
}

// Adds at most 10 curve25519 points and returns the CompressedRistretto bytes representation
pub struct Curve25519Add;

impl LinearCostPrecompile for Curve25519Add {
    const BASE: u64 = 60;
    const WORD: u64 = 12;

    fn raw_execute(input: &[u8], _: u64) -> Result<(ExitSucceed, Vec<u8>), PrecompileFailure> {
        if input.len() % 32 != 0 {
            return Err(PrecompileFailure::Error {
                exit_status: ExitError::Other("input must contain multiple of 32 bytes".into()),
            });
        };

        if input.len() > 320 {
            return Err(PrecompileFailure::Error {
                exit_status: ExitError::Other(
                    "input cannot be greater than 320 bytes (10 compressed points)".into(),
                ),
            });
        };

        let mut points = Vec::new();
        let mut temp_buf = <&[u8]>::clone(&input);
        while !temp_buf.is_empty() {
            let mut buf = [0; 32];
            buf.copy_from_slice(&temp_buf[0..32]);
            let point = CompressedRistretto(buf);
            points.push(point);
            temp_buf = &temp_buf[32..];
        }

        let sum = points
            .iter()
            .fold(RistrettoPoint::identity(), |acc, point| {
                let pt = point.decompress().unwrap_or_else(RistrettoPoint::identity);
                acc + pt
            });

        Ok((ExitSucceed::Returned, sum.compress().to_bytes().to_vec()))
    }
}

// Multiplies a scalar field element with an elliptic curve point
pub struct Curve25519ScalarMul;

impl LinearCostPrecompile for Curve25519ScalarMul {
    const BASE: u64 = 60;
    const WORD: u64 = 12;

    fn raw_execute(input: &[u8], _: u64) -> Result<(ExitSucceed, Vec<u8>), PrecompileFailure> {
        if input.len() != 64 {
            return Err(PrecompileFailure::Error {
                exit_status: ExitError::Other(
                    "input must contain 64 bytes (scalar - 32 bytes, point - 32 bytes)".into(),
                ),
            });
        };

        // first 32 bytes is for the scalar value
        let mut scalar_buf = [0; 32];
        scalar_buf.copy_from_slice(&input[0..32]);
        let scalar = Scalar::from_bytes_mod_order(scalar_buf);

        // second 32 bytes is for the compressed ristretto point bytes
        let mut pt_buf = [0; 32];
        pt_buf.copy_from_slice(&input[32..64]);
        let point = CompressedRistretto(pt_buf)
            .decompress()
            .unwrap_or_else(RistrettoPoint::identity);

        let scalar_mul = scalar * point;
        Ok((
            ExitSucceed::Returned,
            scalar_mul.compress().to_bytes().to_vec(),
        ))
    }
}

#[cfg(test)]
mod tests {
    use curve25519_dalek::constants;
    use ed25519_dalek::{Signer, SigningKey};

    use super::*;

    #[test]
    fn test_empty_input() -> Result<(), PrecompileFailure> {
        let input: [u8; 0] = [];
        let cost: u64 = 1;

        match Ed25519Verify::raw_execute(&input, cost) {
            Ok((_, _)) => {
                panic!("Test not expected to pass");
            }
            Err(e) => {
                assert_eq!(
                    e,
                    PrecompileFailure::Error {
                        exit_status: ExitError::Other("input must contain 128 bytes".into())
                    }
                );
                Ok(())
            }
        }
    }

    #[test]
    fn test_verify() -> Result<(), PrecompileFailure> {
        #[allow(clippy::zero_prefixed_literal)]
            let secret_key_bytes: [u8; ed25519_dalek::SECRET_KEY_LENGTH] = [
            157, 097, 177, 157, 239, 253, 090, 096, 186, 132, 074, 244, 146, 236, 044, 196, 068,
            073, 197, 105, 123, 050, 105, 025, 112, 059, 172, 003, 028, 174, 127, 096,
        ];

        let keypair = SigningKey::from_bytes(&secret_key_bytes);
        let public_key = keypair.verifying_key();

        let msg: &[u8] = b"abcdefghijklmnopqrstuvwxyz123456";
        assert_eq!(msg.len(), 32);
        let signature = keypair.sign(msg);

        // input is:
        // 1) message (32 bytes)
        // 2) pubkey (32 bytes)
        // 3) signature (64 bytes)
        let mut input: Vec<u8> = Vec::with_capacity(128);
        input.extend_from_slice(msg);
        input.extend_from_slice(&public_key.to_bytes());
        input.extend_from_slice(&signature.to_bytes());
        assert_eq!(input.len(), 128);

        let cost: u64 = 1;

        match Ed25519Verify::raw_execute(&input, cost) {
            Ok((_, output)) => {
                assert_eq!(output.len(), 32);
                assert_eq!(output[0], 0u8);
                assert_eq!(output[1], 0u8);
                assert_eq!(output[2], 0u8);
                assert_eq!(output[31], 0u8);
            }
            Err(e) => {
                return Err(e);
            }
        };

        // try again with a different message
        let msg: &[u8] = b"BAD_MESSAGE_mnopqrstuvwxyz123456";

        let mut input: Vec<u8> = Vec::with_capacity(128);
        input.extend_from_slice(msg);
        input.extend_from_slice(&public_key.to_bytes());
        input.extend_from_slice(&signature.to_bytes());
        assert_eq!(input.len(), 128);

        match Ed25519Verify::raw_execute(&input, cost) {
            Ok((_, output)) => {
                assert_eq!(output.len(), 32);
                assert_eq!(output[0], 0u8);
                assert_eq!(output[1], 0u8);
                assert_eq!(output[2], 0u8);
                assert_eq!(output[31], 1u8); // non-zero indicates error (in our case, 1)
            }
            Err(e) => {
                return Err(e);
            }
        };

        Ok(())
    }

    #[test]
    fn test_sum() -> Result<(), PrecompileFailure> {
        let s1 = Scalar::from(999u64);
        let p1 = constants::RISTRETTO_BASEPOINT_POINT * s1;

        let s2 = Scalar::from(333u64);
        let p2 = constants::RISTRETTO_BASEPOINT_POINT * s2;

        let vec = vec![p1, p2];
        let mut input = vec![];
        input.extend_from_slice(&p1.compress().to_bytes());
        input.extend_from_slice(&p2.compress().to_bytes());

        let sum: RistrettoPoint = vec.iter().sum();
        let cost: u64 = 1;

        match Curve25519Add::raw_execute(&input, cost) {
            Ok((_, out)) => {
                assert_eq!(out, sum.compress().to_bytes());
                Ok(())
            }
            Err(e) => {
                panic!("Test not expected to fail: {:?}", e);
            }
        }
    }

    #[test]
    fn test_empty() -> Result<(), PrecompileFailure> {
        // Test that sum works for the empty iterator
        let input = vec![];

        let cost: u64 = 1;

        match Curve25519Add::raw_execute(&input, cost) {
            Ok((_, out)) => {
                assert_eq!(out, RistrettoPoint::identity().compress().to_bytes());
                Ok(())
            }
            Err(e) => {
                panic!("Test not expected to fail: {:?}", e);
            }
        }
    }

    #[test]
    fn test_scalar_mul() -> Result<(), PrecompileFailure> {
        let s1 = Scalar::from(999u64);
        let s2 = Scalar::from(333u64);
        let p1 = constants::RISTRETTO_BASEPOINT_POINT * s1;
        let p2 = constants::RISTRETTO_BASEPOINT_POINT * s2;

        let mut input = vec![];
        input.extend_from_slice(&s1.to_bytes());
        input.extend_from_slice(&constants::RISTRETTO_BASEPOINT_POINT.compress().to_bytes());

        let cost: u64 = 1;

        match Curve25519ScalarMul::raw_execute(&input, cost) {
            Ok((_, out)) => {
                assert_eq!(out, p1.compress().to_bytes());
                assert_ne!(out, p2.compress().to_bytes());
                Ok(())
            }
            Err(e) => {
                panic!("Test not expected to fail: {:?}", e);
            }
        }
    }

    #[test]
    fn test_scalar_mul_empty_error() -> Result<(), PrecompileFailure> {
        let input = vec![];

        let cost: u64 = 1;

        match Curve25519ScalarMul::raw_execute(&input, cost) {
            Ok((_, _out)) => {
                panic!("Test not expected to work");
            }
            Err(e) => {
                assert_eq!(
                    e,
                    PrecompileFailure::Error {
                        exit_status: ExitError::Other(
                            "input must contain 64 bytes (scalar - 32 bytes, point - 32 bytes)"
                                .into()
                        )
                    }
                );
                Ok(())
            }
        }
    }

    #[test]
    fn test_point_addition_bad_length() -> Result<(), PrecompileFailure> {
        let input: Vec<u8> = [0u8; 33].to_vec();

        let cost: u64 = 1;

        match Curve25519Add::raw_execute(&input, cost) {
            Ok((_, _out)) => {
                panic!("Test not expected to work");
            }
            Err(e) => {
                assert_eq!(
                    e,
                    PrecompileFailure::Error {
                        exit_status: ExitError::Other(
                            "input must contain multiple of 32 bytes".into()
                        )
                    }
                );
                Ok(())
            }
        }
    }

    #[test]
    fn test_point_addition_too_many_points() -> Result<(), PrecompileFailure> {
        let mut input = vec![];
        input.extend_from_slice(&constants::RISTRETTO_BASEPOINT_POINT.compress().to_bytes()); // 1
        input.extend_from_slice(&constants::RISTRETTO_BASEPOINT_POINT.compress().to_bytes()); // 2
        input.extend_from_slice(&constants::RISTRETTO_BASEPOINT_POINT.compress().to_bytes()); // 3
        input.extend_from_slice(&constants::RISTRETTO_BASEPOINT_POINT.compress().to_bytes()); // 4
        input.extend_from_slice(&constants::RISTRETTO_BASEPOINT_POINT.compress().to_bytes()); // 5
        input.extend_from_slice(&constants::RISTRETTO_BASEPOINT_POINT.compress().to_bytes()); // 6
        input.extend_from_slice(&constants::RISTRETTO_BASEPOINT_POINT.compress().to_bytes()); // 7
        input.extend_from_slice(&constants::RISTRETTO_BASEPOINT_POINT.compress().to_bytes()); // 8
        input.extend_from_slice(&constants::RISTRETTO_BASEPOINT_POINT.compress().to_bytes()); // 9
        input.extend_from_slice(&constants::RISTRETTO_BASEPOINT_POINT.compress().to_bytes()); // 10
        input.extend_from_slice(&constants::RISTRETTO_BASEPOINT_POINT.compress().to_bytes()); // 11

        let cost: u64 = 1;

        match Curve25519Add::raw_execute(&input, cost) {
            Ok((_, _out)) => {
                panic!("Test not expected to work");
            }
            Err(e) => {
                assert_eq!(
                    e,
                    PrecompileFailure::Error {
                        exit_status: ExitError::Other(
                            "input cannot be greater than 320 bytes (10 compressed points)".into()
                        )
                    }
                );
                Ok(())
            }
        }
    }
}