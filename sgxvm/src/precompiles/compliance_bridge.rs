extern crate sgx_tstd as std;

use ethabi::{encode, Address, ParamType, Token as AbiToken, Token};
use evm::executor::stack::{PrecompileHandle, PrecompileOutput};
use evm::{ExitError, ExitRevert};
use primitive_types::H160;
use std::prelude::v1::*;
use std::vec::Vec;

use crate::precompiles::{
    ExitSucceed, LinearCostPrecompileWithQuerier, PrecompileFailure, PrecompileResult,
};
use crate::protobuf_generated::ffi;
use crate::{coder, querier, GoQuerier};

// Selector of addVerificationDetails function
const ADD_VERIFICATION_FN_SELECTOR: &str = "e62364ab";
// Selector of hasVerification function
const HAS_VERIFICATION_FN_SELECTOR: &str = "4887fcd8";
// Selector of getVerificationData function
const GET_VERIFICATION_DATA_FN_SELECTOR: &str = "cc8995ec";

/// Precompile for interactions with x/compliance module.
pub struct ComplianceBridge;

impl LinearCostPrecompileWithQuerier for ComplianceBridge {
    const BASE: u64 = 60;
    const WORD: u64 = 150;

    fn execute(querier: *mut GoQuerier, handle: &mut impl PrecompileHandle) -> PrecompileResult {
        let target_gas = handle.gas_limit();
        let cost = crate::precompiles::ensure_linear_cost(
            target_gas,
            handle.input().len() as u64,
            Self::BASE,
            Self::WORD,
        )?;

        handle.record_cost(cost)?;

        let context = handle.context();
        let (exit_status, output) = route(querier, context.caller, handle.input())?;
        Ok(PrecompileOutput {
            exit_status,
            output,
        })
    }
}

fn route(
    querier: *mut GoQuerier,
    caller: H160,
    data: &[u8],
) -> Result<(ExitSucceed, Vec<u8>), PrecompileFailure> {
    if data.len() <= 4 {
        return Err(PrecompileFailure::Revert {
            exit_status: ExitRevert::Reverted,
            output: encode(&[AbiToken::String("cannot decode input".into())]),
        });
    }

    let input_signature = hex::encode(data[..4].to_vec());
    match input_signature.as_str() {
        HAS_VERIFICATION_FN_SELECTOR => {
            let has_verification_params = vec![
                ParamType::Address,
                ParamType::Uint(32),
                ParamType::Uint(32),
                ParamType::Array(Box::new(ParamType::Address)),
            ];

            let decoded_params = match decode_input(has_verification_params, &data[4..]) {
                Ok(params) => params,
                Err(_) => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&vec![AbiToken::String(
                            "failed to decode input parameters".into(),
                        )]),
                    });
                }
            };

            let user_address = match decoded_params[0].clone().into_address() {
                Some(addr) => addr,
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid user address".into(),
                        )]),
                    });
                }
            };

            let verification_type = match decoded_params[1].clone().into_uint() {
                Some(vtype) => vtype.as_u32(),
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid verification type".into(),
                        )]),
                    });
                }
            };

            let expiration_timestamp = match decoded_params[2].clone().into_uint() {
                Some(timestamp) => timestamp.as_u32(),
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid expiration timestamp".into(),
                        )]),
                    });
                }
            };

            let allowed_issuers = match decoded_params[3].clone().into_array() {
                Some(array) => array,
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid allowed issuers array".into(),
                        )]),
                    });
                }
            };

            // Decode allowed issuers
            let allowed_issuers: Result<Vec<Address>, _> = allowed_issuers
                .into_iter()
                .map(|issuer| match issuer.into_address() {
                    Some(address) => Ok(address),
                    None => Err(()),
                })
                .collect();

            let allowed_issuers = match allowed_issuers {
                Ok(issuers) => issuers,
                Err(_) => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "one or more invalid issuer addresses".into(),
                        )]),
                    });
                }
            };

            let encoded_request = coder::encode_has_verification_request(
                user_address,
                verification_type,
                expiration_timestamp,
                allowed_issuers,
            );

            match querier::make_request(querier, encoded_request) {
                Some(result) => {
                    let has_verification = protobuf::parse_from_bytes::<
                        ffi::QueryHasVerificationResponse,
                    >(result.as_slice())
                    .map_err(|_| PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "cannot decode protobuf response".into(),
                        )]),
                    })?;

                    let tokens = vec![AbiToken::Bool(has_verification.hasVerification)];

                    let encoded_response = encode(&tokens);
                    Ok((ExitSucceed::Returned, encoded_response.to_vec()))
                }
                None => {
                    Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "call to hasVerification function to x/compliance failed".into(),
                        )]),
                    })
                }
            }
        }
        ADD_VERIFICATION_FN_SELECTOR => {
            let verification_params = vec![
                ParamType::Address,
                ParamType::String,
                ParamType::Uint(32),
                ParamType::Uint(32),
                ParamType::Uint(32),
                ParamType::Bytes,
                ParamType::String,
                ParamType::String,
                ParamType::Uint(32),
            ];

            let decoded_params = match decode_input(verification_params, &data[4..]) {
                Ok(params) => params,
                Err(_) => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "failed to decode input parameters".into(),
                        )]),
                    });
                }
            };

            let user_address = match decoded_params[0].clone().into_address() {
                Some(addr) => addr,
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid user address".into(),
                        )]),
                    });
                }
            };

            let origin_chain = match decoded_params[1].clone().into_string() {
                Some(chain) => chain,
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid origin chain".into(),
                        )]),
                    });
                }
            };

            let verification_type = match decoded_params[2].clone().into_uint() {
                Some(vtype) => vtype.as_u32(),
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid verification type".into(),
                        )]),
                    });
                }
            };

            let issuance_timestamp = match decoded_params[3].clone().into_uint() {
                Some(timestamp) => timestamp.as_u32(),
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid issuance timestamp".into(),
                        )]),
                    });
                }
            };

            let expiration_timestamp = match decoded_params[4].clone().into_uint() {
                Some(timestamp) => timestamp.as_u32(),
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid expiration timestamp".into(),
                        )]),
                    });
                }
            };

            let proof_data = match decoded_params[5].clone().into_bytes() {
                Some(data) => data,
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid proof data".into(),
                        )]),
                    });
                }
            };

            let schema = match decoded_params[6].clone().into_string() {
                Some(schema) => schema,
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid schema".into(),
                        )]),
                    });
                }
            };

            let issuer_verification_id = match decoded_params[7].clone().into_string() {
                Some(id) => id,
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid issuer verification ID".into(),
                        )]),
                    });
                }
            };

            let version = match decoded_params[8].clone().into_uint() {
                Some(ver) => ver.as_u32(),
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid version".into(),
                        )]),
                    });
                }
            };

            let encoded_request = coder::encode_add_verification_details_request(
                user_address,
                caller,
                origin_chain,
                verification_type,
                issuance_timestamp,
                expiration_timestamp,
                proof_data,
                schema,
                issuer_verification_id,
                version,
            );

            match querier::make_request(querier, encoded_request) {
                Some(result) => {
                    let added_verification = protobuf::parse_from_bytes::<
                        ffi::QueryAddVerificationDetailsResponse,
                    >(result.as_slice())
                    .map_err(|_| PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "cannot parse protobuf response".into(),
                        )]),
                    })?;

                    let token = vec![AbiToken::Bytes(
                        added_verification.verificationId.into(),
                    )];
                    let encoded_response = encode(&token);

                    Ok((ExitSucceed::Returned, encoded_response.to_vec()))
                }
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "call to addVerificationDetails to x/compliance failed".into(),
                        )]),
                    });
                }
            }
        }
        GET_VERIFICATION_DATA_FN_SELECTOR => {
            let get_verification_data_params = vec![ParamType::Address, ParamType::Address];
            let decoded_params = match decode_input(get_verification_data_params, &data[4..]) {
                Ok(params) => params,
                Err(_) => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "failed to decode input parameters".into(),
                        )]),
                    });
                }
            };

            let user_address = match decoded_params[0].clone().into_address() {
                Some(addr) => addr,
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid user address".into(),
                        )]),
                    });
                }
            };

            let issuer_address = match decoded_params[1].clone().into_address() {
                Some(addr) => addr,
                None => {
                    return Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "invalid issuer address".into(),
                        )]),
                    });
                }
            };

            let encoded_request = coder::encode_get_verification_data(user_address, issuer_address);

            match querier::make_request(querier, encoded_request) {
                Some(result) => {
                    let get_verification_data = protobuf::parse_from_bytes::<ffi::QueryGetVerificationDataResponse>(result.as_slice())
                        .map_err(|_| PrecompileFailure::Revert {
                            exit_status: ExitRevert::Reverted,
                            output: encode(&[AbiToken::String(
                                "cannot decode protobuf response".into(),
                            )]),
                        })?;

                    let data = get_verification_data
                        .data
                        .into_iter()
                        .flat_map(|log| {
                            let issuer_address = Address::from_slice(&log.issuerAddress);
                            let tokens = vec![AbiToken::Tuple(vec![
                                AbiToken::Uint(log.verificationType.into()),
                                AbiToken::Bytes(log.verificationID.clone().into()),
                                AbiToken::Address(issuer_address.clone()),
                                AbiToken::String(log.originChain.clone()),
                                AbiToken::Uint(log.issuanceTimestamp.into()),
                                AbiToken::Uint(log.expirationTimestamp.into()),
                                AbiToken::Bytes(log.originalData.clone().into()),
                                AbiToken::String(log.schema.clone()),
                                AbiToken::String(log.issuerVerificationId.clone()),
                                AbiToken::Uint(log.version.into()),
                            ])];

                            tokens.into_iter()
                        })
                        .collect::<Vec<AbiToken>>();

                    let encoded_response = encode(&[AbiToken::Array(data)]);
                    Ok((ExitSucceed::Returned, encoded_response.to_vec()))
                }
                None => {
                    Err(PrecompileFailure::Revert {
                        exit_status: ExitRevert::Reverted,
                        output: encode(&[AbiToken::String(
                            "call to getVerificationData to x/compliance failed".into(),
                        )]),
                    })
                }
            }
        }
        _ => Err(PrecompileFailure::Revert {
            exit_status: ExitRevert::Reverted,
            output: encode(&vec![AbiToken::String("incorrect request".into())]),
        }),
    }
}

fn decode_input(
    param_types: Vec<ParamType>,
    input: &[u8],
) -> Result<Vec<Token>, PrecompileFailure> {
    let decoded_params =
        ethabi::decode(&param_types, input).map_err(|err| PrecompileFailure::Revert {
            exit_status: ExitRevert::Reverted,
            output: encode(&[AbiToken::String(
                format!("cannot decode params: {:?}", err).into(),
            )]),
        })?;

    if decoded_params.len() != param_types.len() {
        return Err(PrecompileFailure::Revert {
            exit_status: ExitRevert::Reverted,
            output: encode(&[AbiToken::String(
                "incorrect decoded params len".into(),
            )]),
        });
    }

    Ok(decoded_params)
}
