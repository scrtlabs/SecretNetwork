use core::time;
use mem::MaybeUninit;
use std::{mem, thread, vec};

use cosmwasm_std::{
    attr, coins, entry_point, to_binary, BankMsg, Binary, CosmosMsg, Deps, DepsMut, Empty, Env,
    Event, MessageInfo, QueryRequest, Reply, ReplyOn, Response, StdError, StdResult, Storage,
    SubMsg, SubMsgResult, WasmMsg, WasmQuery,
};
use cosmwasm_storage::PrefixedStorage;
use secp256k1::Secp256k1;

use crate::msg::{ExecuteMsg, ExternalMessages, InstantiateMsg, QueryMsg, QueryRes};
use crate::state::{count, count_read, expiration, expiration_read, PREFIX_TEST, TEST_KEY};

#[entry_point]
pub fn instantiate(
    deps: DepsMut,
    env: Env,
    info: MessageInfo,
    msg: InstantiateMsg,
) -> StdResult<Response> {
    match msg {
        InstantiateMsg::WasmMsg { ty } => wasm_msg(ty),
        InstantiateMsg::Counter { counter, expires } => {
            if counter == 0 {
                return Err(StdError::generic_err("got wrong counter on init"));
            }

            count(deps.storage).save(&counter)?;
            let expires = env.block.height + expires;
            expiration(deps.storage).save(&expires)?;
            let mut resp = Response::default();
            resp.data = Some(env.contract.address.as_bytes().into());
            Ok(resp)
        }

        InstantiateMsg::AddAttributes {} => Ok(Response::new()
            .add_attribute("attr1", "🦄")
            .add_attribute("attr2", "🌈")),
        InstantiateMsg::AddAttributesWithSubmessage { id } => Ok(Response::new()
            .add_submessage(SubMsg {
                id,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash: env.contract.code_hash,
                    contract_addr: env.contract.address.into_string(),
                    msg: Binary::from(r#"{"add_more_attributes":{}}"#.as_bytes().to_vec()),
                    funds: vec![],
                })
                .into(),
                reply_on: match id {
                    0 => ReplyOn::Never,
                    _ => ReplyOn::Always,
                },
                gas_limit: None,
            })
            .add_attribute("attr1", "🦄")
            .add_attribute("attr2", "🌈")),

        InstantiateMsg::AddPlaintextAttributes {} => Ok(Response::new()
            .add_attribute_plaintext("attr1", "🦄")
            .add_attribute_plaintext("attr2", "🌈")),
        InstantiateMsg::AddPlaintextAttributesWithSubmessage { id } => Ok(Response::new()
            .add_submessage(SubMsg {
                id,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash: env.contract.code_hash,
                    contract_addr: env.contract.address.into_string(),
                    msg: Binary::from(
                        r#"{"add_more_plaintext_attributes":{}}"#.as_bytes().to_vec(),
                    ),
                    funds: vec![],
                })
                .into(),
                reply_on: match id {
                    0 => ReplyOn::Never,
                    _ => ReplyOn::Always,
                },
                gas_limit: None,
            })
            .add_attribute_plaintext("attr1", "🦄")
            .add_attribute_plaintext("attr2", "🌈")),

        InstantiateMsg::AddEvents {} => Ok(Response::new()
            .add_event(
                Event::new("cyber1".to_string())
                    .add_attribute("attr1", "🦄")
                    .add_attribute("attr2", "🌈"),
            )
            .add_event(
                Event::new("cyber2".to_string())
                    .add_attribute("attr3", "🐙")
                    .add_attribute("attr4", "🦄"),
            )),
        InstantiateMsg::AddEventsWithSubmessage { id } => Ok(Response::new()
            .add_submessage(SubMsg {
                id,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash: env.contract.code_hash,
                    contract_addr: env.contract.address.into_string(),
                    msg: Binary::from(r#"{"add_more_events":{}}"#.as_bytes().to_vec()),
                    funds: vec![],
                })
                .into(),
                reply_on: match id {
                    0 => ReplyOn::Never,
                    _ => ReplyOn::Always,
                },
                gas_limit: None,
            })
            .add_event(
                Event::new("cyber1".to_string())
                    .add_attribute("attr1", "🦄")
                    .add_attribute("attr2", "🌈"),
            )
            .add_event(
                Event::new("cyber2".to_string())
                    .add_attribute("attr3", "🐙")
                    .add_attribute("attr4", "🦄"),
            )),
        InstantiateMsg::AddMixedAttributesAndEvents {} => Ok(Response::new()
            .add_event(
                Event::new("cyber1".to_string())
                    .add_attribute_plaintext("attr1", "🦄")
                    .add_attribute_plaintext("attr2", "🌈"),
            )
            .add_attribute_plaintext("attr3", "🐙")
            .add_attribute_plaintext("attr4", "🦄")),
        InstantiateMsg::AddMixedAttributesAndEventsWithSubmessage { id } => Ok(Response::new()
            .add_submessage(SubMsg {
                id,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash: env.contract.code_hash,
                    contract_addr: env.contract.address.into_string(),
                    msg: Binary::from(
                        r#"{"add_more_mixed_attributes_and_events":{}}"#.as_bytes().to_vec(),
                    ),
                    funds: vec![],
                })
                .into(),
                reply_on: match id {
                    0 => ReplyOn::Never,
                    _ => ReplyOn::Always,
                },
                gas_limit: None,
            })
            .add_event(
                Event::new("cyber1".to_string())
                    .add_attribute("attr1", "🦄")
                    .add_attribute("attr2", "🌈"),
            )
            .add_attribute("attr3", "🐙")
            .add_attribute_plaintext("attr4", "🦄")),

        InstantiateMsg::MeasureGasForSubmessage { id } => {
            let msg = match id {
                0 => Binary::from(r#"{"gas_meter":{}}"#.as_bytes().to_vec()),
                _ => Binary::from(r#"{"gas_meter_proxy":{}}"#.as_bytes().to_vec()),
            };

            Ok(Response::new().add_submessage(SubMsg {
                id,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash: env.contract.code_hash,
                    contract_addr: env.contract.address.into_string(),
                    msg,
                    funds: vec![],
                })
                .into(),
                reply_on: match id {
                    0 => ReplyOn::Never,
                    _ => ReplyOn::Always,
                },
                gas_limit: None,
            }))
        }
        // These were ported from the v0.10 test-contract:
        InstantiateMsg::Nop {} => Ok(Response::new().add_attribute("init", "🌈")),
        InstantiateMsg::Callback {
            contract_addr,
            code_hash,
        } => Ok(Response::new()
            .add_message(CosmosMsg::Wasm(WasmMsg::Execute {
                code_hash,
                contract_addr: contract_addr.clone(),
                msg: Binary::from(r#"{"c":{"x":0,"y":13}}"#.as_bytes().to_vec()),
                funds: vec![],
            }))
            .add_attribute("init with a callback", "🦄")),
        InstantiateMsg::CallbackContractError {
            contract_addr,
            code_hash,
        } => Ok(Response::new()
            .add_message(CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: contract_addr.clone(),
                code_hash,
                msg: Binary::from(r#"{"contract_error":{"error_type":"generic_err"}}"#.as_bytes()),
                funds: vec![],
            }))
            .add_attribute("init with a callback with contract error", "🤷‍♀️")),
        InstantiateMsg::ContractError { error_type } => Err(map_string_to_error(error_type)),
        InstantiateMsg::NoLogs {} => Ok(Response::new()),
        InstantiateMsg::CallbackToInit { code_id, code_hash } => Ok(Response::new()
            .add_message(CosmosMsg::Wasm(WasmMsg::Instantiate {
                code_id,
                msg: Binary::from(r#"{"nop":{}}"#.as_bytes().to_vec()),
                code_hash,
                funds: vec![],
                label: String::from("fi"),
            }))
            .add_attribute("instantiating a new contract from init!", "🐙")),
        InstantiateMsg::CallbackBadParams {
            contract_addr,
            code_hash,
        } => Ok(
            Response::new().add_message(CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: contract_addr.clone(),
                code_hash,
                msg: Binary::from(r#"{"c":{"x":"banana","y":3}}"#.as_bytes().to_vec()),
                funds: vec![],
            })),
        ),
        InstantiateMsg::Panic {} => panic!("panic in init"),
        InstantiateMsg::SendExternalQueryDepthCounter {
            to,
            depth,
            code_hash,
        } => Ok(Response::new().add_attribute(
            format!(
                "{}",
                send_external_query_depth_counter(deps.as_ref(), to, depth, code_hash)
            ),
            "",
        )),
        InstantiateMsg::SendExternalQueryRecursionLimit {
            to,
            depth,
            code_hash,
        } => Ok(Response::new().add_attribute(
            "message",
            send_external_query_recursion_limit(deps.as_ref(), to, depth, code_hash)?,
        )),
        InstantiateMsg::CallToInit {
            code_id,
            code_hash,
            label,
            msg,
        } => Ok(Response::new()
            .add_message(CosmosMsg::Wasm(WasmMsg::Instantiate {
                code_id,
                code_hash,
                msg: Binary(msg.as_bytes().into()),
                funds: vec![],
                label,
            }))
            .add_attribute("a", "a")),
        InstantiateMsg::CallToExec {
            addr,
            code_hash,
            msg,
        } => Ok(Response::new()
            .add_message(CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: addr,
                code_hash,
                msg: Binary(msg.as_bytes().into()),
                funds: vec![],
            }))
            .add_attribute("b", "b")),
        InstantiateMsg::CallToQuery {
            addr,
            code_hash,
            msg,
        } => {
            let answer: u32 = deps
                .querier
                .query(&QueryRequest::Wasm(WasmQuery::Smart {
                    contract_addr: addr,
                    code_hash,
                    msg: Binary::from(msg.as_bytes().to_vec()),
                }))
                .map_err(|err| {
                    StdError::generic_err(format!("Got an error from query: {:?}", err))
                })?;

            Ok(Response::new().add_attribute("c", format!("{}", answer)))
        }
        InstantiateMsg::BankMsgSend { to, amount } => {
            Ok(Response::new().add_message(CosmosMsg::Bank(BankMsg::Send {
                to_address: to,
                amount,
            })))
        }
        InstantiateMsg::BankMsgBurn { amount } => {
            Ok(Response::new().add_message(CosmosMsg::Bank(BankMsg::Burn { amount })))
        }
        InstantiateMsg::CosmosMsgCustom {} => {
            Ok(Response::new().add_message(CosmosMsg::Custom(Empty {})))
        }
        InstantiateMsg::SendMultipleFundsToInitCallback {
            coins,
            code_id,
            code_hash,
        } => Ok(
            Response::new().add_message(CosmosMsg::Wasm(WasmMsg::Instantiate {
                code_id,
                code_hash,
                msg: Binary("{\"nop\":{}}".as_bytes().to_vec()),
                funds: coins,
                label: "init test".to_string(),
            })),
        ),
        InstantiateMsg::SendMultipleFundsToExecCallback {
            coins,
            to,
            code_hash,
        } => Ok(
            Response::new().add_message(CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: to,
                code_hash,
                msg: Binary("{\"no_data\":{}}".as_bytes().to_vec()),
                funds: coins,
            })),
        ),
        InstantiateMsg::GetEnv {} => Ok(Response::new()
            .add_attribute("env", serde_json_wasm::to_string(&env).unwrap())
            .add_attribute("info", serde_json_wasm::to_string(&info).unwrap())),
        InstantiateMsg::TestRemoveDb {} => {
            let mut store = PrefixedStorage::new(deps.storage, PREFIX_TEST);
            // save something
            store.set(
                TEST_KEY,
                &bincode2::serialize(&true)
                    .map_err(|_| StdError::generic_err("serialization error"))?,
            );

            store.remove(TEST_KEY);
            // test if it was removed
            if store.get(TEST_KEY).is_some() {
                return Err(StdError::generic_err("This key should have been removed!"));
            }

            Ok(Response::new())
        }
    }
}

pub fn wasm_msg(ty: String) -> StdResult<Response> {
    if ty == "success" {
        return Ok(Response::default());
    } else if ty == "err" {
        return Err(StdError::generic_err("custom error"));
    } else if ty == "panic" {
        panic!()
    }

    return Err(StdError::generic_err("custom error"));
}

#[entry_point]
pub fn execute(deps: DepsMut, env: Env, info: MessageInfo, msg: ExecuteMsg) -> StdResult<Response> {
    match msg {
        ExecuteMsg::IncrementTimes { times } => {
            let mut res = Response::default();
            for _ in 0..times {
                res = res.add_message(CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash: env.contract.code_hash.clone(),
                    contract_addr: env.contract.address.clone().into_string(),
                    msg: Binary::from("{\"increment\":{\"addition\":1}}".as_bytes().to_vec()),
                    funds: vec![],
                }));
            }
            Ok(res)
        }
        ExecuteMsg::LastMsgMarkerNop {} => {
            // also tests using finalize like this
            Ok(Response::new().add_message(CosmosMsg::FinalizeTx(Empty {})))
        }
        ExecuteMsg::LastMsgMarker {} => {
            let increment_msg = SubMsg {
                id: 0,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash: env.contract.code_hash,
                    contract_addr: env.contract.address.into_string(),
                    msg: Binary::from("{\"increment\":{\"addition\":1}}".as_bytes().to_vec()),
                    funds: vec![],
                })
                .into(),
                reply_on: ReplyOn::Never,
                gas_limit: None,
            };

            let bank_msg = SubMsg {
                id: 0,
                msg: CosmosMsg::Bank(BankMsg::Send {
                    to_address: "".to_string(),
                    amount: coins(1u128, "ust"),
                })
                .into(),
                reply_on: ReplyOn::Never,
                gas_limit: None,
            };
            Ok(Response::new()
                .add_submessages(vec![
                    increment_msg.clone(),
                    increment_msg.clone(),
                    increment_msg.clone(),
                    increment_msg.clone(),
                ])
                // also tests using finalize like this
                .add_message(CosmosMsg::finalize_tx())
                .add_submessages(vec![increment_msg.clone(), bank_msg.clone()]))
        }
        ExecuteMsg::WasmMsg { ty } => wasm_msg(ty),
        ExecuteMsg::Increment { addition } => increment(env, deps, addition),
        ExecuteMsg::SendFundsWithErrorWithReply {} => Ok(Response::new()
            .add_submessage(SubMsg {
                id: 8000,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash: env.contract.code_hash,
                    contract_addr: env.contract.address.into_string(),
                    msg: Binary::from(r#"{"add_more_attributes":{}}"#.as_bytes().to_vec()),
                    funds: coins(100, "lior"),
                })
                .into(),
                reply_on: ReplyOn::Always,
                gas_limit: None,
            })
            .add_attribute("attr1", "🦄")
            .add_attribute("attr2", "🌈")),
        ExecuteMsg::SendFundsWithReply {} => Ok(Response::new()
            .add_submessage(SubMsg {
                id: 8001,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash: env.contract.code_hash,
                    contract_addr: env.contract.address.into_string(),
                    msg: Binary::from(r#"{"add_more_attributes":{}}"#.as_bytes().to_vec()),
                    funds: coins(100, "denom"),
                })
                .into(),
                reply_on: ReplyOn::Always,
                gas_limit: None,
            })
            .add_attribute("attr1", "🦄")
            .add_attribute("attr2", "🌈")),
        ExecuteMsg::AddAttributes {} => Ok(Response::new()
            .add_attribute("attr1", "🦄")
            .add_attribute("attr2", "🌈")),
        ExecuteMsg::AddAttributesWithSubmessage { id } => Ok(Response::new()
            .add_submessage(SubMsg {
                id,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash: env.contract.code_hash,
                    contract_addr: env.contract.address.into_string(),
                    msg: Binary::from(r#"{"add_more_attributes":{}}"#.as_bytes().to_vec()),
                    funds: vec![],
                })
                .into(),
                reply_on: match id {
                    0 => ReplyOn::Never,
                    _ => ReplyOn::Always,
                },
                gas_limit: None,
            })
            .add_attribute("attr1", "🦄")
            .add_attribute("attr2", "🌈")),
        ExecuteMsg::AddMoreAttributes {} => Ok(Response::new()
            .add_attribute("attr3", "🍉")
            .add_attribute("attr4", "🥝")),
        ExecuteMsg::AddPlaintextAttributes {} => Ok(Response::new()
            .add_attribute_plaintext("attr1", "🦄")
            .add_attribute_plaintext("attr2", "🌈")),
        ExecuteMsg::AddPlaintextAttributesWithSubmessage { id } => Ok(Response::new()
            .add_submessage(SubMsg {
                id,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash: env.contract.code_hash,
                    contract_addr: env.contract.address.into_string(),
                    msg: Binary::from(
                        r#"{"add_more_plaintext_attributes":{}}"#.as_bytes().to_vec(),
                    ),
                    funds: vec![],
                })
                .into(),
                reply_on: match id {
                    0 => ReplyOn::Never,
                    _ => ReplyOn::Always,
                },
                gas_limit: None,
            })
            .add_attribute_plaintext("attr1", "🦄")
            .add_attribute_plaintext("attr2", "🌈")),
        ExecuteMsg::AddMorePlaintextAttributes {} => Ok(Response::new()
            .add_attribute_plaintext("attr3", "🍉")
            .add_attribute_plaintext("attr4", "🥝")),

        ExecuteMsg::AddEvents {} => Ok(Response::new()
            .add_event(
                Event::new("cyber1".to_string())
                    .add_attribute("attr1", "🦄")
                    .add_attribute("attr2", "🌈"),
            )
            .add_event(
                Event::new("cyber2".to_string())
                    .add_attribute("attr3", "🐙")
                    .add_attribute("attr4", "🦄"),
            )),
        ExecuteMsg::AddEventsWithSubmessage { id } => Ok(Response::new()
            .add_submessage(SubMsg {
                id,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash: env.contract.code_hash,
                    contract_addr: env.contract.address.into_string(),
                    msg: Binary::from(r#"{"add_more_events":{}}"#.as_bytes().to_vec()),
                    funds: vec![],
                })
                .into(),
                reply_on: match id {
                    0 => ReplyOn::Never,
                    _ => ReplyOn::Always,
                },
                gas_limit: None,
            })
            .add_event(
                Event::new("cyber1".to_string())
                    .add_attribute("attr1", "🦄")
                    .add_attribute("attr2", "🌈"),
            )
            .add_event(
                Event::new("cyber2".to_string())
                    .add_attribute("attr3", "🐙")
                    .add_attribute("attr4", "🦄"),
            )),
        ExecuteMsg::AddMoreEvents {} => Ok(Response::new()
            .add_event(
                Event::new("cyber3".to_string())
                    .add_attribute("attr1", "🤯")
                    .add_attribute("attr2", "🤟"),
            )
            .add_event(
                Event::new("cyber4".to_string())
                    .add_attribute("attr3", "😅")
                    .add_attribute("attr4", "🦄"),
            )),

        ExecuteMsg::AddMixedAttributesAndEvents {} => Ok(Response::new()
            .add_event(
                Event::new("cyber1".to_string())
                    .add_attribute_plaintext("attr1", "🦄")
                    .add_attribute_plaintext("attr2", "🌈"),
            )
            .add_attribute_plaintext("attr3", "🐙")
            .add_attribute_plaintext("attr4", "🦄")),
        ExecuteMsg::AddMixedAttributesAndEventsWithSubmessage { id } => Ok(Response::new()
            .add_submessage(SubMsg {
                id,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash: env.contract.code_hash,
                    contract_addr: env.contract.address.into_string(),
                    msg: Binary::from(
                        r#"{"add_more_mixed_attributes_and_events":{}}"#.as_bytes().to_vec(),
                    ),
                    funds: vec![],
                })
                .into(),
                reply_on: match id {
                    0 => ReplyOn::Never,
                    _ => ReplyOn::Always,
                },
                gas_limit: None,
            })
            .add_event(
                Event::new("cyber1".to_string())
                    .add_attribute("attr1", "🦄")
                    .add_attribute("attr2", "🌈"),
            )
            .add_attribute("attr3", "🐙")
            .add_attribute_plaintext("attr4", "🦄")),
        ExecuteMsg::AddMoreMixedAttributesAndEvents {} => Ok(Response::new()
            .add_event(
                Event::new("cyber2".to_string())
                    .add_attribute("attr5", "🐙")
                    .add_attribute("attr6", "🦄"),
            )
            .add_attribute("attr7", "😅")
            .add_attribute_plaintext("attr8", "🦄")),
        ExecuteMsg::AddAttributesFromV010 {
            addr,
            code_hash,
            id,
        } => Ok(Response::new()
            .add_submessage(SubMsg {
                id,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    code_hash,
                    contract_addr: addr,
                    msg: Binary::from(r#"{"add_more_attributes":{}}"#.as_bytes().to_vec()),
                    funds: vec![],
                })
                .into(),
                reply_on: match id {
                    0 => ReplyOn::Never,
                    _ => ReplyOn::Always,
                },
                gas_limit: None,
            })
            .add_event(
                Event::new("cyber1".to_string())
                    .add_attribute("attr1", "🦄")
                    .add_attribute("attr2", "🌈"),
            )
            .add_attribute("attr3", "🐙")
            .add_attribute_plaintext("attr4", "🦄")),
        ExecuteMsg::GasMeter {} => {
            // busy work
            let mut v = vec![0; 65536];
            let mut x = 0;
            loop {
                x += (x + 1) % 65536;
                v[x] = 65536 - x;
            }
        }
        ExecuteMsg::GasMeterProxy {} => Ok(Response::default()),
        ExecuteMsg::TransferMoney { amount } => transfer_money(deps, amount),
        ExecuteMsg::RecursiveReply {} => recursive_reply(env, deps),
        ExecuteMsg::RecursiveReplyFail {} => recursive_reply_fail(env, deps),
        ExecuteMsg::InitNewContract {} => init_new_contract(env, deps),
        ExecuteMsg::InitNewContractWithError {} => init_new_contract_with_error(env, deps),
        ExecuteMsg::SubMsgLoop { iter } => sub_msg_loop(env, deps, iter),
        ExecuteMsg::SubMsgLoopIner { iter } => sub_msg_loop_iner(env, deps, iter),
        ExecuteMsg::MultipleSubMessages {} => send_multiple_sub_messages(env, deps),
        ExecuteMsg::MultipleSubMessagesNoReply {} => send_multiple_sub_messages_no_reply(env, deps),
        ExecuteMsg::QuickError {} => {
            count(deps.storage).save(&123456)?;
            Err(StdError::generic_err("error in execute"))
        }
        ExecuteMsg::MultipleSubMessagesNoReplyWithError {} => {
            send_multiple_sub_messages_no_reply_with_error(env, deps)
        }
        ExecuteMsg::MultipleSubMessagesNoReplyWithPanic {} => {
            send_multiple_sub_messages_no_reply_with_panic(env, deps)
        }
        ExecuteMsg::MultipleSubMessagesWithReplyWithError {} => {
            send_multiple_sub_messages_with_reply_with_error(env, deps)
        }
        ExecuteMsg::MultipleSubMessagesWithReplyWithPanic {} => {
            send_multiple_sub_messages_with_reply_with_panic(env, deps)
        }
        ExecuteMsg::InitV10 {
            counter,
            code_id,
            code_hash,
        } => {
            let mut resp = Response::default();

            let msg =
                "{\"init_from_v1\":{\"counter\":".to_string() + counter.to_string().as_str() + "}}";
            resp.messages.push(SubMsg {
                id: 1700,
                msg: CosmosMsg::Wasm(WasmMsg::Instantiate {
                    code_hash,
                    msg: Binary::from(msg.as_bytes().to_vec()),
                    funds: vec![],
                    label: "new2231231".to_string(),
                    code_id,
                }),
                gas_limit: Some(10000000_u64),
                reply_on: ReplyOn::Always,
            });

            Ok(resp)
        }
        ExecuteMsg::ExecV10 { address, code_hash } => {
            let mut resp = Response::default();

            resp.messages.push(SubMsg {
                id: 1800,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    msg: Binary::from("{\"execute_from_v1\":{\"counter\":20}}".as_bytes().to_vec()),
                    contract_addr: address,
                    code_hash,
                    funds: vec![],
                }),
                gas_limit: Some(10000000_u64),
                reply_on: ReplyOn::Always,
            });

            Ok(resp)
        }
        ExecuteMsg::AddAttributeStep1 {} => Ok(Response::new()
            .add_submessage(SubMsg {
                id: 8451,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    msg: Binary("{\"add_attribute_step2\":{}}".as_bytes().to_vec()),
                    contract_addr: env.contract.address.into_string(),
                    code_hash: env.contract.code_hash,
                    funds: vec![],
                }),
                gas_limit: Some(10000000_u64),
                reply_on: ReplyOn::Success,
            })
            .add_attribute_plaintext("attr1", "🦄")
            .set_data(to_binary("step1")?)),
        ExecuteMsg::AddAttributeStep2 {} => Ok(Response::new()
            .add_message(CosmosMsg::Wasm(WasmMsg::Execute {
                msg: Binary("{\"add_attribute_step3\":{}}".as_bytes().to_vec()),
                contract_addr: env.contract.address.into_string(),
                code_hash: env.contract.code_hash,
                funds: vec![],
            }))
            .add_attribute_plaintext("attr2", "🦄")
            .set_data(to_binary("step2")?)),
        ExecuteMsg::AddAttributeStep3 {} => Ok(Response::new()
            .add_message(CosmosMsg::Wasm(WasmMsg::Execute {
                msg: Binary("{\"add_attribute_step4\":{}}".as_bytes().to_vec()),
                contract_addr: env.contract.address.into_string(),
                code_hash: env.contract.code_hash,
                funds: vec![],
            }))
            .add_attribute_plaintext("attr3", "🦄")
            .set_data(to_binary("step3")?)),
        ExecuteMsg::AddAttributeStep4 {} => Ok(Response::new()
            .add_attribute_plaintext("attr4", "🦄")
            .set_data(to_binary("step4")?)),
        ExecuteMsg::InitV10NoReply {
            counter,
            code_id,
            code_hash,
        } => {
            let mut resp = Response::default();

            let msg =
                "{\"init_from_v1\":{\"counter\":".to_string() + counter.to_string().as_str() + "}}";
            resp.messages.push(SubMsg {
                id: 8888,
                msg: CosmosMsg::Wasm(WasmMsg::Instantiate {
                    code_hash,
                    msg: Binary::from(msg.as_bytes().to_vec()),
                    funds: vec![],
                    label: "new2231231".to_string(),
                    code_id,
                }),
                gas_limit: Some(10000000_u64),
                reply_on: ReplyOn::Never,
            });

            Ok(resp)
        }
        ExecuteMsg::ExecV10NoReply { address, code_hash } => {
            let mut resp = Response::default();

            resp.messages.push(SubMsg {
                id: 8889,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    msg: Binary::from("{\"execute_from_v1\":{\"counter\":20}}".as_bytes().to_vec()),
                    contract_addr: address,
                    code_hash,
                    funds: vec![],
                }),
                gas_limit: Some(10000000_u64),
                reply_on: ReplyOn::Never,
            });

            Ok(resp)
        }
        ExecuteMsg::QueryV10 { address, code_hash } => {
            let response = deps.querier.query(&QueryRequest::Wasm(WasmQuery::Smart {
                code_hash,
                contract_addr: address,
                msg: to_binary(&ExternalMessages::GetCountFromV1 {})?,
            }))?;

            match response {
                QueryRes::Get { count } => {
                    let mut resp = Response::default();
                    resp.data = Some((count as u32).to_be_bytes().into());
                    return Ok(resp);
                }
            }
        }

        ExecuteMsg::InitV10WithError { code_id, code_hash } => {
            let mut resp = Response::default();

            let msg = "{\"init_from_v1_with_error\":{}";
            resp.messages.push(SubMsg {
                id: 2000,
                msg: CosmosMsg::Wasm(WasmMsg::Instantiate {
                    code_hash,
                    msg: Binary::from(msg.as_bytes().to_vec()),
                    funds: vec![],
                    label: "new2231231".to_string(),
                    code_id,
                }),
                gas_limit: Some(10000000_u64),
                reply_on: ReplyOn::Always,
            });

            Ok(resp)
        }
        ExecuteMsg::ExecV10WithError { address, code_hash } => {
            let mut resp = Response::default();

            resp.messages.push(SubMsg {
                id: 2100,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    msg: Binary::from("{\"execute_from_v1_with_error\":{}}".as_bytes().to_vec()),
                    contract_addr: address,
                    code_hash,
                    funds: vec![],
                }),
                gas_limit: Some(10000000_u64),
                reply_on: ReplyOn::Always,
            });

            Ok(resp)
        }
        ExecuteMsg::InitV10NoReplyWithError { code_id, code_hash } => {
            let mut resp = Response::default();

            let msg = "{\"init_from_v1_with_error\":{}}";
            resp.messages.push(SubMsg {
                id: 8890,
                msg: CosmosMsg::Wasm(WasmMsg::Instantiate {
                    code_hash,
                    msg: Binary::from(msg.as_bytes().to_vec()),
                    funds: vec![],
                    label: "new2231231".to_string(),
                    code_id,
                }),
                gas_limit: Some(10000000_u64),
                reply_on: ReplyOn::Never,
            });

            Ok(resp)
        }
        ExecuteMsg::ExecV10NoReplyWithError { address, code_hash } => {
            let mut resp = Response::default();

            resp.messages.push(SubMsg {
                id: 8891,
                msg: CosmosMsg::Wasm(WasmMsg::Execute {
                    msg: Binary::from("{\"execute_from_v1_with_error\":{}}".as_bytes().to_vec()),
                    contract_addr: address,
                    code_hash,
                    funds: vec![],
                }),
                gas_limit: Some(10000000_u64),
                reply_on: ReplyOn::Never,
            });

            Ok(resp)
        }
        ExecuteMsg::QueryV10WithError { address, code_hash } => {
            deps.querier.query(&QueryRequest::Wasm(WasmQuery::Smart {
                code_hash,
                contract_addr: address,
                msg: to_binary(&ExternalMessages::QueryFromV1WithError {})?,
            }))?;

            // shouldn't be reachable
            Ok(Response::default())
        }
        // These were ported from the v0.10 test-contract:
        ExecuteMsg::A {
            contract_addr,
            code_hash,
            x,
            y,
        } => Ok(a(deps, env, contract_addr, code_hash, x, y)),
        ExecuteMsg::B {
            contract_addr,
            code_hash,
            x,
            y,
        } => Ok(b(deps, env, contract_addr, code_hash, x, y)),
        ExecuteMsg::C { x, y } => Ok(c(deps, env, x, y)),
        ExecuteMsg::UnicodeData {} => Ok(unicode_data(deps, env)),
        ExecuteMsg::EmptyLogKeyValue {} => Ok(empty_log_key_value(deps, env)),
        ExecuteMsg::EmptyData {} => Ok(empty_data(deps, env)),
        ExecuteMsg::NoData {} => Ok(no_data(deps, env)),
        ExecuteMsg::ContractError { error_type } => Err(map_string_to_error(error_type)),
        ExecuteMsg::NoLogs {} => Ok(Response::default()),
        ExecuteMsg::CallbackToInit { code_id, code_hash } => {
            Ok(exec_callback_to_init(deps, env, code_id, code_hash))
        }
        ExecuteMsg::CallbackBadParams {
            contract_addr,
            code_hash,
        } => Ok(exec_callback_bad_params(contract_addr, code_hash)),
        ExecuteMsg::CallbackContractError {
            contract_addr,
            code_hash,
        } => Ok(exec_with_callback_contract_error(contract_addr, code_hash)),
        ExecuteMsg::SetState { key, value } => Ok(set_state(deps, key, value)),
        ExecuteMsg::GetState { key } => Ok(get_state(deps, key)),
        ExecuteMsg::RemoveState { key } => Ok(remove_state(deps, key)),
        ExecuteMsg::TestCanonicalizeAddressErrors {} => test_canonicalize_address_errors(deps),
        ExecuteMsg::Panic {} => panic!("panic in exec"),
        ExecuteMsg::AllocateOnHeap { bytes } => Ok(allocate_on_heap(bytes as usize)),
        ExecuteMsg::PassNullPointerToImportsShouldThrow { pass_type } => {
            Ok(pass_null_pointer_to_imports_should_throw(deps, pass_type))
        }
        ExecuteMsg::SendExternalQuery { to, code_hash } => {
            Ok(Response::new().set_data(vec![send_external_query(deps.as_ref(), to, code_hash)]))
        }
        ExecuteMsg::SendExternalQueryDepthCounter {
            to,
            code_hash,
            depth,
        } => Ok(
            Response::new().set_data(vec![send_external_query_depth_counter(
                deps.as_ref(),
                to,
                depth,
                code_hash,
            )]),
        ),
        ExecuteMsg::SendExternalQueryRecursionLimit {
            to,
            code_hash,
            depth,
        } => Ok(
            Response::new().set_data(to_binary(&send_external_query_recursion_limit(
                deps.as_ref(),
                to,
                depth,
                code_hash,
            )?)?),
        ),
        ExecuteMsg::SendExternalQueryPanic { to, code_hash } => {
            send_external_query_panic(deps, to, code_hash)
        }
        ExecuteMsg::SendExternalQueryError { to, code_hash } => {
            send_external_query_stderror(deps, to, code_hash)
        }
        ExecuteMsg::SendExternalQueryBadAbi { to, code_hash } => {
            send_external_query_bad_abi(deps, to, code_hash)
        }
        ExecuteMsg::SendExternalQueryBadAbiReceiver { to, code_hash } => {
            send_external_query_bad_abi_receiver(deps, to, code_hash)
        }
        ExecuteMsg::LogMsgSender {} => {
            Ok(Response::new().add_attribute("msg.sender", info.sender.as_str()))
        }
        ExecuteMsg::CallbackToLogMsgSender { to, code_hash } => Ok(Response::new()
            .add_message(CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: to.clone(),
                code_hash,
                msg: Binary::from(r#"{"log_msg_sender":{}}"#.as_bytes().to_vec()),
                funds: vec![],
            }))
            .add_attribute("hi", "hey")),
        ExecuteMsg::DepositToContract {} => {
            Ok(Response::new().set_data(to_binary(&info.funds).unwrap()))
        }
        ExecuteMsg::SendFunds {
            amount,
            from: _,
            to,
            denom,
        } => Ok(Response::new().add_message(CosmosMsg::Bank(BankMsg::Send {
            to_address: to,
            amount: coins(amount.into(), denom),
        }))),
        ExecuteMsg::SendFundsToInitCallback {
            amount,
            denom,
            code_id,
            code_hash,
        } => Ok(
            Response::new().add_message(CosmosMsg::Wasm(WasmMsg::Instantiate {
                msg: Binary("{\"nop\":{}}".as_bytes().to_vec()),
                code_id,
                code_hash,
                label: String::from("yo"),
                funds: coins(amount.into(), denom),
            })),
        ),
        ExecuteMsg::SendFundsToExecCallback {
            amount,
            denom,
            to,
            code_hash,
        } => Ok(
            Response::new().add_message(CosmosMsg::Wasm(WasmMsg::Execute {
                msg: Binary("{\"no_data\":{}}".as_bytes().to_vec()),
                contract_addr: to,
                code_hash,
                funds: coins(amount.into(), denom),
            })),
        ),
        ExecuteMsg::Sleep { ms } => {
            thread::sleep(time::Duration::from_millis(ms));

            Ok(Response::new())
        }
        ExecuteMsg::WithFloats { x, y } => Ok(Response::new().set_data(use_floats(x, y))),
        ExecuteMsg::CallToInit {
            code_id,
            code_hash,
            label,
            msg,
        } => Ok(Response::new()
            .add_message(CosmosMsg::Wasm(WasmMsg::Instantiate {
                code_id,
                code_hash,
                msg: Binary(msg.as_bytes().into()),
                funds: vec![],
                label,
            }))
            .add_attribute("a", "a")),
        ExecuteMsg::CallToExec {
            addr,
            code_hash,
            msg,
        } => Ok(Response::new()
            .add_message(CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: addr,
                code_hash,
                msg: Binary(msg.as_bytes().into()),
                funds: vec![],
            }))
            .add_attribute("b", "b")),
        ExecuteMsg::CallToQuery {
            addr,
            code_hash,
            msg,
        } => {
            let answer: u32 = deps
                .querier
                .query(&QueryRequest::Wasm(WasmQuery::Smart {
                    contract_addr: addr,
                    code_hash,
                    msg: Binary::from(msg.as_bytes().to_vec()),
                }))
                .map_err(|err| {
                    StdError::generic_err(format!("Got an error from query: {:?}", err))
                })?;

            Ok(Response::new().add_attribute("c", format!("{}", answer)))
        }
        ExecuteMsg::StoreReallyLongKey {} => {
            let mut store = PrefixedStorage::new(deps.storage, b"my_prefix");
            store.set(REALLY_LONG, b"hello");
            Ok(Response::default())
        }
        ExecuteMsg::StoreReallyShortKey {} => {
            let mut store = PrefixedStorage::new(deps.storage, b"my_prefix");
            store.set(b"a", b"hello");
            Ok(Response::default())
        }
        ExecuteMsg::StoreReallyLongValue {} => {
            let mut store = PrefixedStorage::new(deps.storage, b"my_prefix");
            store.set(b"hello", REALLY_LONG);
            Ok(Response::default())
        }
        ExecuteMsg::Secp256k1Verify {
            pubkey,
            sig,
            msg_hash,
            iterations,
        } => {
            let mut res = Ok(Response::new());

            // loop for benchmarking
            for _ in 0..iterations {
                res = match deps.api.secp256k1_verify(
                    msg_hash.as_slice(),
                    sig.as_slice(),
                    pubkey.as_slice(),
                ) {
                    Ok(result) => {
                        Ok(Response::new().add_attribute("result", format!("{}", result)))
                    }
                    Err(err) => Err(StdError::generic_err(format!("{:?}", err))),
                };
            }

            return res;
        }
        ExecuteMsg::Secp256k1VerifyFromCrate {
            pubkey,
            sig,
            msg_hash,
            iterations,
        } => {
            let mut res = Ok(Response::new());

            // loop for benchmarking
            for _ in 0..iterations {
                let secp256k1_verifier = Secp256k1::verification_only();

                let secp256k1_signature =
                    secp256k1::Signature::from_compact(&sig.0).map_err(|err| {
                        StdError::generic_err(format!("Malformed signature: {:?}", err))
                    })?;
                let secp256k1_pubkey = secp256k1::PublicKey::from_slice(pubkey.0.as_slice())
                    .map_err(|err| StdError::generic_err(format!("Malformed pubkey: {:?}", err)))?;
                let secp256k1_msg =
                    secp256k1::Message::from_slice(&msg_hash.as_slice()).map_err(|err| {
                        StdError::generic_err(format!(
                            "Failed to create a secp256k1 message from signed_bytes: {:?}",
                            err
                        ))
                    })?;

                res = match secp256k1_verifier.verify(
                    &secp256k1_msg,
                    &secp256k1_signature,
                    &secp256k1_pubkey,
                ) {
                    Ok(()) => Ok(Response::new().add_attribute("result", "true")),
                    Err(_err) => Ok(Response::new().add_attribute("result", "false")),
                };
            }

            return res;
        }
        ExecuteMsg::Ed25519Verify {
            pubkey,
            sig,
            msg,
            iterations,
        } => {
            let mut res = Ok(Response::new());

            // loop for benchmarking
            for _ in 0..iterations {
                res =
                    match deps
                        .api
                        .ed25519_verify(msg.as_slice(), sig.as_slice(), pubkey.as_slice())
                    {
                        Ok(result) => {
                            Ok(Response::new().add_attribute("result", format!("{}", result)))
                        }
                        Err(err) => Err(StdError::generic_err(format!("{:?}", err))),
                    };
            }

            return res;
        }
        ExecuteMsg::Ed25519BatchVerify {
            pubkeys,
            sigs,
            msgs,
            iterations,
        } => {
            let mut res = Ok(Response::new());

            // loop for benchmarking
            for _ in 0..iterations {
                res = match deps.api.ed25519_batch_verify(
                    msgs.iter()
                        .map(|m| m.as_slice())
                        .collect::<Vec<&[u8]>>()
                        .as_slice(),
                    sigs.iter()
                        .map(|s| s.as_slice())
                        .collect::<Vec<&[u8]>>()
                        .as_slice(),
                    pubkeys
                        .iter()
                        .map(|p| p.as_slice())
                        .collect::<Vec<&[u8]>>()
                        .as_slice(),
                ) {
                    Ok(result) => {
                        Ok(Response::new().add_attribute("result", format!("{}", result)))
                    }
                    Err(err) => Err(StdError::generic_err(format!("{:?}", err))),
                };
            }

            return res;
        }
        ExecuteMsg::Secp256k1RecoverPubkey {
            msg_hash,
            sig,
            recovery_param,
            iterations,
        } => {
            let mut res = Ok(Response::new());

            // loop for benchmarking
            for _ in 0..iterations {
                res = match deps.api.secp256k1_recover_pubkey(
                    msg_hash.as_slice(),
                    sig.as_slice(),
                    recovery_param,
                ) {
                    Ok(result) => Ok(Response::new()
                        .add_attribute("result", format!("{}", Binary(result).to_base64()))),
                    Err(err) => Err(StdError::generic_err(format!("{:?}", err))),
                };
            }

            return res;
        }
        ExecuteMsg::Secp256k1Sign {
            msg,
            privkey,
            iterations,
        } => {
            let mut res = Ok(Response::new());

            // loop for benchmarking
            for _ in 0..iterations {
                res = match deps.api.secp256k1_sign(msg.as_slice(), privkey.as_slice()) {
                    Ok(result) => Ok(Response::new()
                        .add_attribute("result", format!("{}", Binary(result).to_base64()))),
                    Err(err) => Err(StdError::generic_err(format!("{:?}", err))),
                };
            }

            return res;
        }
        ExecuteMsg::Ed25519Sign {
            msg,
            privkey,
            iterations,
        } => {
            let mut res = Ok(Response::new());

            // loop for benchmarking
            for _ in 0..iterations {
                res = match deps.api.ed25519_sign(msg.as_slice(), privkey.as_slice()) {
                    Ok(result) => Ok(Response::new()
                        .add_attribute("result", format!("{}", Binary(result).to_base64()))),
                    Err(err) => Err(StdError::generic_err(format!("{:?}", err))),
                };
            }

            return res;
        }
        ExecuteMsg::BankMsgSend { to, amount } => {
            Ok(Response::new().add_message(CosmosMsg::Bank(BankMsg::Send {
                to_address: to,
                amount,
            })))
        }
        ExecuteMsg::BankMsgBurn { amount } => {
            Ok(Response::new().add_message(CosmosMsg::Bank(BankMsg::Burn { amount })))
        }
        ExecuteMsg::CosmosMsgCustom {} => {
            Ok(Response::new().add_message(CosmosMsg::Custom(Empty {})))
        }
        ExecuteMsg::SendMultipleFundsToInitCallback {
            coins,
            code_id,
            code_hash,
        } => Ok(
            Response::new().add_message(CosmosMsg::Wasm(WasmMsg::Instantiate {
                code_id,
                code_hash,
                msg: Binary("{\"nop\":{}}".as_bytes().to_vec()),
                funds: coins,
                label: "test".to_string(),
            })),
        ),
        ExecuteMsg::SendMultipleFundsToExecCallback {
            coins,
            to,
            code_hash,
        } => Ok(
            Response::new().add_message(CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: to,
                code_hash,
                msg: Binary("{\"no_data\":{}}".as_bytes().to_vec()),
                funds: coins,
            })),
        ),
        ExecuteMsg::ValidateAddress { addr } => match deps.api.addr_validate(addr.as_str()) {
            Ok(a) => Ok(Response::new().set_data(a.as_bytes())),
            Err(_) => Ok(Response::new().set_data(to_binary("Apple")?)),
        },
        ExecuteMsg::GetEnv {} => Ok(Response::new()
            .add_attribute("env", serde_json_wasm::to_string(&env).unwrap())
            .add_attribute("info", serde_json_wasm::to_string(&info).unwrap())),
        ExecuteMsg::ExecuteMultipleContracts { details } => {
            if details.len() == 0 {
                return Ok(Response::default().set_data(env.contract.address.as_bytes()));
            }

            if details[0].should_error {
                return Err(StdError::generic_err("Error by request"));
            }

            Ok(Response::new()
                .add_submessage(SubMsg {
                    id: details[0].msg_id,
                    msg: CosmosMsg::Wasm(WasmMsg::Execute {
                        code_hash: details[0].contract_hash.clone(),
                        contract_addr: details[0].contract_address.clone(),
                        msg: Binary(
                            format!(
                                r#"{{"execute_multiple_contracts":{{"details":{}}}}}"#,
                                serde_json_wasm::to_string(&details[1..].to_vec()).unwrap(),
                            )
                            .into_bytes(),
                        ),
                        funds: vec![],
                    })
                    .into(),
                    reply_on: match details[0].msg_id {
                        0 => ReplyOn::Never,
                        _ => ReplyOn::Always,
                    },
                    gas_limit: None,
                })
                .set_data(details[0].data.as_bytes()))
        }
    }
}

pub fn increment(env: Env, deps: DepsMut, c: u64) -> StdResult<Response> {
    if c == 0 {
        return Err(StdError::generic_err("got wrong counter on increment"));
    }

    if c == 9875 {
        let new_count = count_read(deps.storage).load()? + 100;
        count(deps.storage).save(&new_count)?;
        return Err(StdError::generic_err("got wrong counter on increment"));
    }

    if c == 9876 {
        let new_count = count_read(deps.storage).load()? + 100;
        count(deps.storage).save(&new_count)?;
        panic!()
    }

    if c == 9911 {
        let mut resp = Response::default();
        resp.messages.push(SubMsg {
            id: 1304,
            msg: CosmosMsg::Wasm(WasmMsg::Execute {
                contract_addr: env.contract.address.into_string(),
                code_hash: env.contract.code_hash,
                msg: Binary::from("{\"increment\":{\"addition\":5}}".as_bytes().to_vec()),
                funds: vec![],
            }),
            gas_limit: Some(10000000_u64),
            reply_on: ReplyOn::Always,
        });

        return Ok(resp);
    }

    let new_count = count_read(deps.storage).load()? + c;
    count(deps.storage).save(&new_count)?;

    let mut resp = Response::default();
    resp.data = Some((new_count as u32).to_be_bytes().into());

    Ok(resp)
}

pub fn transfer_money(_deps: DepsMut, amount: u64) -> StdResult<Response> {
    let mut resp = Response::default();
    resp.messages.push(SubMsg {
        id: 1337,
        msg: CosmosMsg::Bank(BankMsg::Send {
            to_address: "secret105w4vl4gm7q00yg5jngewt5kp7aj0xjk7zrnhw".to_string(),
            amount: coins(amount as u128, "uscrt"),
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    Ok(resp)
}

pub fn recursive_reply(env: Env, _deps: DepsMut) -> StdResult<Response> {
    let mut resp = Response::default();
    resp.messages.push(SubMsg {
        id: 1304,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.into_string(),
            code_hash: env.contract.code_hash,
            msg: Binary::from("{\"increment\":{\"addition\":2}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    Ok(resp)
}

pub fn sub_msg_loop(env: Env, _deps: DepsMut, iter: u64) -> StdResult<Response> {
    if iter == 0 {
        return Err(StdError::generic_err("stopped loop"));
    }

    let mut resp = Response::default();
    let msg = "{\"sub_msg_loop_iner\":{\"iter\":".to_string() + iter.to_string().as_str() + "}}";
    resp.messages.push(SubMsg {
        id: 1500,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.into_string(),
            code_hash: env.contract.code_hash,
            msg: cosmwasm_std::Binary(msg.as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    Ok(resp)
}

pub fn sub_msg_loop_iner(_env: Env, deps: DepsMut, iter: u64) -> StdResult<Response> {
    if iter == 0 {
        return Err(StdError::generic_err("stopped loop"));
    }

    increment(_env, deps, 1)?;

    let mut resp = Response::default();
    resp.data = Some(((iter - 1) as u64).to_string().as_bytes().into());

    Ok(resp)
}

pub fn send_multiple_sub_messages(env: Env, _deps: DepsMut) -> StdResult<Response> {
    let mut resp = Response::default();
    resp.messages.push(SubMsg {
        id: 1600,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":2}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Never,
    });

    resp.messages.push(SubMsg {
        id: 1601,
        msg: CosmosMsg::Wasm(WasmMsg::Instantiate {
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from(
                "{\"counter\":{\"counter\":150, \"expires\":100}}"
                    .as_bytes()
                    .to_vec(),
            ),
            funds: vec![],
            label: "new202213".to_string(),
            code_id: 1,
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    resp.messages.push(SubMsg {
        id: 1602,
        msg: CosmosMsg::Bank(BankMsg::Send {
            to_address: "secret105w4vl4gm7q00yg5jngewt5kp7aj0xjk7zrnhw".to_string(),
            amount: coins(1200 as u128, "uscrt"),
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    resp.messages.push(SubMsg {
        id: 1603,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":2}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Never,
    });

    Ok(resp)
}

pub fn send_multiple_sub_messages_no_reply(env: Env, deps: DepsMut) -> StdResult<Response> {
    let mut resp = Response::default();

    resp.messages.push(SubMsg {
        id: 1610,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":2}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Never,
    });

    resp.messages.push(SubMsg {
        id: 1611,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":2}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Never,
    });

    resp.messages.push(SubMsg {
        id: 1612,
        msg: CosmosMsg::Wasm(WasmMsg::Instantiate {
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from(
                "{\"counter\":{\"counter\":150, \"expires\":100}}"
                    .as_bytes()
                    .to_vec(),
            ),
            funds: vec![],
            label: "new202213".to_string(),
            code_id: 1,
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Never,
    });

    resp.messages.push(SubMsg {
        id: 1613,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":2}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Never,
    });

    resp.data = Some(
        (count_read(deps.storage).load()? as u32)
            .to_be_bytes()
            .into(),
    );
    Ok(resp)
}

pub fn send_multiple_sub_messages_no_reply_with_error(
    env: Env,
    deps: DepsMut,
) -> StdResult<Response> {
    let mut resp = Response::default();

    resp.messages.push(SubMsg {
        id: 1610,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":2}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Error,
    });

    resp.messages.push(SubMsg {
        id: 1611,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":9875}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Success,
    });

    resp.messages.push(SubMsg {
        id: 1613,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":2}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Never,
    });

    let new_count = count_read(deps.storage).load()? + 5;
    count(deps.storage).save(&new_count)?;

    resp.data = Some(
        (count_read(deps.storage).load()? as u32)
            .to_be_bytes()
            .into(),
    );
    Ok(resp)
}

pub fn send_multiple_sub_messages_with_reply_with_error(
    env: Env,
    deps: DepsMut,
) -> StdResult<Response> {
    let mut resp = Response::default();

    resp.messages.push(SubMsg {
        id: 3000,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":2}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    resp.messages.push(SubMsg {
        id: 3001,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":9875}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Error,
    });

    resp.messages.push(SubMsg {
        id: 3003,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":2}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Never,
    });

    resp.messages.push(SubMsg {
        id: 3001,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":9875}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    let new_count = count_read(deps.storage).load()? + 5;
    count(deps.storage).save(&new_count)?;

    resp.data = Some(
        (count_read(deps.storage).load()? as u32)
            .to_be_bytes()
            .into(),
    );
    Ok(resp)
}

pub fn send_multiple_sub_messages_with_reply_with_panic(
    env: Env,
    deps: DepsMut,
) -> StdResult<Response> {
    let mut resp = Response::default();

    resp.messages.push(SubMsg {
        id: 3000,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":2}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    resp.messages.push(SubMsg {
        id: 3001,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":9876}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    resp.messages.push(SubMsg {
        id: 3001,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":9876}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Error,
    });
    let new_count = count_read(deps.storage).load()? + 5;
    count(deps.storage).save(&new_count)?;

    resp.data = Some(
        (count_read(deps.storage).load()? as u32)
            .to_be_bytes()
            .into(),
    );
    Ok(resp)
}

pub fn send_multiple_sub_messages_no_reply_with_panic(
    env: Env,
    deps: DepsMut,
) -> StdResult<Response> {
    let mut resp = Response::default();

    resp.messages.push(SubMsg {
        id: 1610,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":2}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Never,
    });

    resp.messages.push(SubMsg {
        id: 1611,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":9876}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Never,
    });

    resp.messages.push(SubMsg {
        id: 1613,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.clone().into_string(),
            code_hash: env.contract.code_hash.clone(),
            msg: Binary::from("{\"increment\":{\"addition\":2}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Never,
    });

    let new_count = count_read(deps.storage).load()? + 5;
    count(deps.storage).save(&new_count)?;

    resp.data = Some(
        (count_read(deps.storage).load()? as u32)
            .to_be_bytes()
            .into(),
    );
    Ok(resp)
}

pub fn recursive_reply_fail(env: Env, _deps: DepsMut) -> StdResult<Response> {
    let mut resp = Response::default();
    resp.messages.push(SubMsg {
        id: 1305,
        msg: CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: env.contract.address.into_string(),
            code_hash: env.contract.code_hash,
            msg: Binary::from("{\"increment\":{\"addition\":0}}".as_bytes().to_vec()),
            funds: vec![],
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    Ok(resp)
}

pub fn init_new_contract(env: Env, _deps: DepsMut) -> StdResult<Response> {
    let mut resp = Response::default();
    resp.messages.push(SubMsg {
        id: 1404,
        msg: CosmosMsg::Wasm(WasmMsg::Instantiate {
            code_hash: env.contract.code_hash,
            msg: Binary::from(
                "{\"counter\":{\"counter\":150, \"expires\":100}}"
                    .as_bytes()
                    .to_vec(),
            ),
            funds: vec![],
            label: "new202213".to_string(),
            code_id: 1,
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    Ok(resp)
}

pub fn init_new_contract_with_error(env: Env, _deps: DepsMut) -> StdResult<Response> {
    let mut resp = Response::default();
    resp.messages.push(SubMsg {
        id: 1405,
        msg: CosmosMsg::Wasm(WasmMsg::Instantiate {
            code_hash: env.contract.code_hash,
            msg: Binary::from(
                "{\"counter\":{\"counter\":0, \"expires\":100}}"
                    .as_bytes()
                    .to_vec(),
            ),
            funds: vec![],
            label: "new2022133".to_string(),
            code_id: 1,
        }),
        gas_limit: Some(10000000_u64),
        reply_on: ReplyOn::Always,
    });

    Ok(resp)
}

#[entry_point]
pub fn query(deps: Deps, env: Env, msg: QueryMsg) -> StdResult<Binary> {
    match msg {
        QueryMsg::Get {} => to_binary(&get(deps, env)?),

        // These were ported from the v0.10 test-contract:
        QueryMsg::ContractError { error_type } => Err(map_string_to_error(error_type)),
        QueryMsg::Panic {} => panic!("panic in query"),
        QueryMsg::ReceiveExternalQuery { num } | QueryMsg::ReceiveExternalQueryV1 { num } => {
            Ok(Binary(serde_json_wasm::to_vec(&(num + 1)).unwrap()))
        }
        QueryMsg::SendExternalQueryInfiniteLoop { to, code_hash } => {
            send_external_query_infinite_loop(deps, to, code_hash)
        }
        QueryMsg::WriteToStorage {} => write_to_storage_in_query(deps.storage),
        QueryMsg::RemoveFromStorage {} => remove_from_storage_in_query(deps.storage),
        QueryMsg::SendExternalQueryDepthCounter {
            to,
            depth,
            code_hash,
        } => Ok(to_binary(&send_external_query_depth_counter(
            deps, to, depth, code_hash,
        ))
        .unwrap()),
        QueryMsg::SendExternalQueryRecursionLimit {
            to,
            depth,
            code_hash,
        } => to_binary(&send_external_query_recursion_limit(
            deps, to, depth, code_hash,
        )?),
        QueryMsg::CallToQuery {
            addr,
            code_hash,
            msg,
        } => {
            let answer: u32 = deps
                .querier
                .query(&QueryRequest::Wasm(WasmQuery::Smart {
                    contract_addr: addr,
                    code_hash,
                    msg: Binary::from(msg.as_bytes().to_vec()),
                }))
                .map_err(|err| {
                    StdError::generic_err(format!("Got an error from query: {:?}", err))
                })?;
            return Ok(to_binary(&answer)?);
        }
        QueryMsg::GetContractVersion {} => {
            let answer: u8 = 1;
            return Ok(to_binary(&answer)?);
        }
        QueryMsg::GetEnv {} => Ok(Binary::from(
            serde_json_wasm::to_string(&env)
                .unwrap()
                .as_bytes()
                .to_vec(),
        )),
    }
}

#[entry_point]
pub fn reply(deps: DepsMut, env: Env, reply: Reply) -> StdResult<Response> {
    match (reply.id, reply.result) {
        (1337, SubMsgResult::Err(_)) => {
            let mut resp = Response::default();
            resp.data = Some(
                (count_read(deps.storage).load()? as u32)
                    .to_be_bytes()
                    .into(),
            );

            Ok(resp)
        }
        (1337, SubMsgResult::Ok(_)) => Err(StdError::generic_err("got wrong bank answer")),

        (1304, SubMsgResult::Err(e)) => Err(StdError::generic_err(format!(
            "recursive reply failed: {}",
            e
        ))),
        (1304, SubMsgResult::Ok(_)) => {
            let mut resp = Response::default();
            resp.data = Some(
                (count_read(deps.storage).load()? as u32)
                    .to_be_bytes()
                    .into(),
            );

            Ok(resp)
        }
        (1305, SubMsgResult::Ok(_)) => {
            Err(StdError::generic_err(format!("recursive reply failed")))
        }
        (1305, SubMsgResult::Err(_)) => {
            let mut resp = Response::default();
            let new_count = 10;
            count(deps.storage).save(&new_count)?;

            resp.data = Some(
                (count_read(deps.storage).load()? as u32)
                    .to_be_bytes()
                    .into(),
            );

            Ok(resp)
        }

        (1404, SubMsgResult::Err(e)) => Err(StdError::generic_err(format!(
            "recursive init failed: {}",
            e
        ))),
        (1404, SubMsgResult::Ok(s)) => match s.data {
            Some(x) => {
                let response = deps.querier.query(&QueryRequest::Wasm(WasmQuery::Smart {
                    code_hash: env.contract.code_hash,
                    contract_addr: String::from_utf8(x.to_vec())?,
                    msg: to_binary(&QueryMsg::Get {})?,
                }))?;

                match response {
                    QueryRes::Get { count } => {
                        let mut resp = Response::default();
                        resp.data = Some((count as u32).to_be_bytes().into());
                        return Ok(resp);
                    }
                }
            }
            None => Err(StdError::generic_err(format!(
                "Init didn't response with contract address",
            ))),
        },
        (1405, SubMsgResult::Ok(_)) => Err(StdError::generic_err(format!(
            "recursive init with error failed"
        ))),
        (1405, SubMsgResult::Err(_)) => {
            let mut resp = Response::default();
            let new_count = 1337;
            count(deps.storage).save(&new_count)?;

            resp.data = Some(
                (count_read(deps.storage).load()? as u32)
                    .to_be_bytes()
                    .into(),
            );

            Ok(resp)
        }

        (1500, SubMsgResult::Ok(iter)) => match iter.data {
            Some(x) => {
                let it = String::from_utf8(x.to_vec())?;
                let mut resp = Response::default();

                let msg = "{\"sub_msg_loop_iner\":{\"iter\":".to_string() + it.as_str() + "}}";
                resp.messages.push(SubMsg {
                    id: 1500,
                    msg: CosmosMsg::Wasm(WasmMsg::Execute {
                        contract_addr: env.contract.address.into_string(),
                        code_hash: env.contract.code_hash,
                        msg: Binary::from(msg.as_bytes().to_vec()),
                        funds: vec![],
                    }),
                    gas_limit: Some(10000000_u64),
                    reply_on: ReplyOn::Always,
                });

                Ok(resp)
            }
            None => Err(StdError::generic_err(format!(
                "Init didn't response with contract address",
            ))),
        },
        (1500, SubMsgResult::Err(_)) => {
            let mut resp = Response::default();
            resp.data = Some(
                (count_read(deps.storage).load()? as u32)
                    .to_be_bytes()
                    .into(),
            );

            Ok(resp)
        }

        (1601, SubMsgResult::Err(e)) => Err(StdError::generic_err(format!(
            "recursive init failed: {}",
            e
        ))),
        (1601, SubMsgResult::Ok(s)) => match s.data {
            Some(_) => {
                let mut resp = Response::default();
                let new_count = 101;
                count(deps.storage).save(&new_count)?;

                resp.data = Some(
                    (count_read(deps.storage).load()? as u32)
                        .to_be_bytes()
                        .into(),
                );

                Ok(resp)
            }
            None => Err(StdError::generic_err(format!(
                "Init didn't response with contract address",
            ))),
        },
        (1602, SubMsgResult::Err(_)) => {
            let mut resp = Response::default();
            let new_count = 102;
            count(deps.storage).save(&new_count)?;

            resp.data = Some(
                (count_read(deps.storage).load()? as u32)
                    .to_be_bytes()
                    .into(),
            );

            Ok(resp)
        }
        (1602, SubMsgResult::Ok(_)) => Err(StdError::generic_err("got wrong bank answer")),
        (1700, SubMsgResult::Ok(s)) => {
            if s.events.len() == 0 {
                return Err(StdError::generic_err(format!(
                    "Init didn't response with contract address",
                )));
            }

            if s.events[0].attributes.len() == 0 {
                return Err(StdError::generic_err(format!(
                    "Init didn't response with contract address",
                )));
            }

            if s.events[0].attributes[0].key != "contract_address" {
                return Err(StdError::generic_err(format!(
                    "Init didn't response with contract address, key was {:?}",
                    s.events[0].attributes[0].key,
                )));
            }

            let mut resp = Response::default();
            resp.data = Some(s.events[0].attributes[0].value.as_bytes().into());

            Ok(resp)
        }
        (1700, SubMsgResult::Err(_)) => Err(StdError::generic_err("Failed to init v010 contract")),
        (1800, SubMsgResult::Ok(s)) => match s.data {
            Some(x) => {
                let counter = String::from_utf8(x.to_vec())?;
                let mut resp = Response::default();
                resp.data = Some(counter.as_bytes().into());

                Ok(resp)
            }
            None => Err(StdError::generic_err(format!(
                "Init didn't response with contract address",
            ))),
        },
        (1800, SubMsgResult::Err(_)) => {
            Err(StdError::generic_err("Failed to execute v010 contract"))
        }

        (2000, SubMsgResult::Ok(_)) => Err(StdError::generic_err(format!(
            "Init with error didn't response with error",
        ))),
        (2000, SubMsgResult::Err(_)) => Ok(Response::default()),
        (2100, SubMsgResult::Ok(_)) => Err(StdError::generic_err(format!(
            "Execute with error didn't response with error",
        ))),
        (2100, SubMsgResult::Err(_)) => Ok(Response::default()),
        (2200, SubMsgResult::Ok(_)) => Ok(Response::new()
            .add_attribute("attr5", "🤯")
            .add_attribute("attr6", "🦄")),
        (2200, SubMsgResult::Err(_)) => {
            Err(StdError::generic_err(format!("Add attributes failed",)))
        }
        (2300, SubMsgResult::Ok(_)) => Ok(Response::new()
            .add_attribute_plaintext("attr5", "🤯")
            .add_attribute_plaintext("attr6", "🦄")),
        (2300, SubMsgResult::Err(_)) => {
            Err(StdError::generic_err(format!("Add attributes failed",)))
        }
        (2400, SubMsgResult::Ok(_)) => Ok(Response::new()
            .add_event(
                Event::new("cyber5".to_string())
                    .add_attribute("attr1", "😗")
                    .add_attribute("attr2", "😋"),
            )
            .add_event(
                Event::new("cyber6".to_string())
                    .add_attribute("attr3", "😉")
                    .add_attribute("attr4", "😊"),
            )),
        (2400, SubMsgResult::Err(_)) => Err(StdError::generic_err(format!("Add events failed",))),
        (2500, SubMsgResult::Ok(_)) => Ok(Response::new()
            .add_event(
                Event::new("cyber3".to_string())
                    .add_attribute("attr9", "🤯")
                    .add_attribute("attr10", "🤟"),
            )
            .add_attribute("attr11", "😉")
            .add_attribute_plaintext("attr12", "😊")),
        (2500, SubMsgResult::Err(_)) => {
            Err(StdError::generic_err(format!("Add mixed events failed",)))
        }
        (2600, SubMsgResult::Ok(_)) => {
            // busy work
            let mut v = vec![0; 65536];
            let mut x = 0;
            loop {
                x += (x + 1) % 65536;
                v[x] = 65536 - x;
            }
        }
        (2600, SubMsgResult::Err(_)) => {
            Err(StdError::generic_err(format!("Gas submessage failed",)))
        }
        (3000, SubMsgResult::Ok(_)) => {
            let new_count = count_read(deps.storage).load()? + 3;
            count(deps.storage).save(&new_count)?;
            Ok(Response::default())
        }
        (3000, SubMsgResult::Err(_)) => {
            Err(StdError::generic_err(format!("Revert submessage failed",)))
        }
        (3001, SubMsgResult::Ok(_)) => Err(StdError::generic_err(format!(
            "Revert submessage should fail",
        ))),
        (3001, SubMsgResult::Err(_)) => Ok(Response::default()),
        (8000, SubMsgResult::Ok(_)) => Err(StdError::generic_err(format!("Unreachable",))),
        (8000, SubMsgResult::Err(_)) => Err(StdError::generic_err(format!("Unreachable",))),
        (8001, SubMsgResult::Ok(_)) => Ok(Response::default()),
        (8001, SubMsgResult::Err(_)) => {
            Err(StdError::generic_err(format!("Funds message failed",)))
        }
        (9000, SubMsgResult::Ok(o)) => match o.data {
            None => Ok(Response::default().set_data(env.contract.address.as_bytes())),
            Some(d) => {
                let new_data =
                    String::from_utf8_lossy(d.as_slice()) + " -> " + env.contract.address.as_str();
                Ok(Response::default().set_data(new_data.as_bytes()))
            }
        },
        (9000, SubMsgResult::Err(_)) => Ok(Response::default().set_data("err".as_bytes())),
        (8451, SubMsgResult::Ok(_)) => Ok(Response::new()
            .add_attribute_plaintext("attr_reply", "🦄")
            .set_data(to_binary("reply")?)),
        //(9000, SubMsgResult::Err(_)) => Err(StdError::generic_err("err")),
        _ => Err(StdError::generic_err("invalid reply id or result")),
    }
}

fn get(deps: Deps, env: Env) -> StdResult<QueryRes> {
    let count = count_read(deps.storage).load()?;
    let expiration = expiration_read(deps.storage).load()?;

    if env.block.height > expiration {
        return Ok(QueryRes::Get { count: 0 });
    }

    Ok(QueryRes::Get { count })
}

fn map_string_to_error(error_type: String) -> StdError {
    let as_str: &str = &error_type[..];
    match as_str {
        "generic_err" => StdError::generic_err("la la 🤯"),
        "invalid_base64" => StdError::invalid_base64("ra ra 🤯"),
        "invalid_utf8" => StdError::invalid_utf8("ka ka 🤯"),
        "not_found" => StdError::not_found("za za 🤯"),
        "parse_err" => StdError::parse_err("na na 🤯", "pa pa 🤯"),
        "serialize_err" => StdError::serialize_err("ba ba 🤯", "ga ga 🤯"),
        // "unauthorized" => StdError::unauthorized(), // dosn't exist in v1
        // "underflow" => StdError::underflow("minuend 🤯", "subtrahend 🤯"), // dosn't exist in v1
        _ => StdError::generic_err("catch-all 🤯"),
    }
}

fn send_external_query_recursion_limit(
    deps: Deps,
    contract_addr: String,
    depth: u8,
    code_hash: String,
) -> StdResult<String> {
    let result = deps
        .querier
        .query(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr: contract_addr.clone(),
            code_hash: code_hash.clone(),
            msg: Binary(
                format!(
                    r#"{{"send_external_query_recursion_limit":{{"to":"{}","code_hash":"{}","depth":{}}}}}"#,
                    contract_addr.clone().to_string(),
                    code_hash.clone().to_string(),
                    depth + 1
                )
                .into_bytes(),
            ),
        }));

    // 10 is the current recursion limit.
    if depth != 10 {
        result
    } else {
        match result {
            Err(StdError::GenericErr { msg, .. }) if msg.contains("exceeded_recursion_limit") => {
                Ok(String::from("Recursion limit was correctly enforced"))
            }
            _ => Err(StdError::generic_err(
                "Recursion limit was bypassed! this is a bug!",
            )),
        }
    }
}

#[cfg(feature = "with_floats")]
fn use_floats(x: u8, y: u8) -> Binary {
    let res: f64 = (x as f64) / (y as f64);
    to_binary(&format!("{}", res)).unwrap()
}

#[cfg(not(feature = "with_floats"))]
fn use_floats(x: u8, y: u8) -> Binary {
    Binary(vec![x, y])
}

fn send_external_query(deps: Deps, contract_addr: String, code_hash: String) -> u8 {
    let answer: u8 = deps
        .querier
        .query(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr,
            code_hash,
            msg: Binary::from(r#"{"receive_external_query":{"num":2}}"#.as_bytes().to_vec()),
        }))
        .unwrap();
    answer
}

fn send_external_query_depth_counter(
    deps: Deps,
    contract_addr: String,
    depth: u8,
    code_hash: String,
) -> u8 {
    if depth == 0 {
        return 0;
    }

    let answer: u8 = deps
        .querier
        .query(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr: contract_addr.clone(),
            code_hash: code_hash.clone(),
            msg: Binary(
                format!(
                    r#"{{"send_external_query_depth_counter":{{"to":"{}","code_hash":"{}","depth":{}}}}}"#,
                    contract_addr.clone(),
                    code_hash.clone(),
                    depth - 1
                )
                .into(),
            ),
        }))
        .unwrap();

    answer + 1
}

fn send_external_query_panic(
    deps: DepsMut,
    contract_addr: String,
    code_hash: String,
) -> StdResult<Response> {
    let err = deps
        .querier
        .query::<u8>(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr,
            msg: Binary::from(r#"{"panic":{}}"#.as_bytes().to_vec()),
            code_hash,
        }))
        .unwrap_err();

    Err(err)
}

fn send_external_query_stderror(
    deps: DepsMut,
    contract_addr: String,
    code_hash: String,
) -> StdResult<Response> {
    let answer = deps
        .querier
        .query::<Binary>(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr,
            msg: Binary::from(
                r#"{"contract_error":{"error_type":"generic_err"}}"#
                    .as_bytes()
                    .to_vec(),
            ),
            code_hash,
        }));

    match answer {
        Ok(wtf) => Ok(Response::new().set_data(wtf)),
        Err(e) => Err(e),
    }
}

fn send_external_query_bad_abi(
    deps: DepsMut,
    contract_addr: String,
    code_hash: String,
) -> StdResult<Response> {
    let answer = deps
        .querier
        .query::<Binary>(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr,
            code_hash,
            msg: Binary::from(
                r#""contract_error":{"error_type":"generic_err"}}"#.as_bytes().to_vec(),
            ),
        }));

    match answer {
        Ok(wtf) => Ok(Response::new().set_data(wtf)),
        Err(e) => Err(e),
    }
}

fn send_external_query_bad_abi_receiver(
    deps: DepsMut,
    contract_addr: String,
    code_hash: String,
) -> StdResult<Response> {
    let answer = deps
        .querier
        .query::<String>(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr,
            msg: Binary::from(r#"{"receive_external_query":{"num":25}}"#.as_bytes().to_vec()),
            code_hash,
        }));

    match answer {
        Ok(wtf) => Ok(Response::new().add_attribute("wtf", wtf)),
        Err(e) => Err(e),
    }
}

fn exec_callback_bad_params(contract_addr: String, code_hash: String) -> Response {
    Response::new().add_message(CosmosMsg::Wasm(WasmMsg::Execute {
        contract_addr: contract_addr.clone(),
        code_hash,
        msg: Binary::from(r#"{"c":{"x":"banana","y":3}}"#.as_bytes().to_vec()),
        funds: vec![],
    }))
}

pub fn a(
    _deps: DepsMut,
    _env: Env,
    contract_addr: String,
    code_hash: String,
    x: u8,
    y: u8,
) -> Response {
    Response::new()
        .add_message(CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: contract_addr.clone(),
            code_hash: code_hash.clone(),
            msg: Binary::from(
                format!(
            "{{\"b\":{{\"x\":{} ,\"y\": {},\"contract_addr\": \"{}\",\"code_hash\": \"{}\" }}}}",
            x,
            y,
            contract_addr.as_str(),
            &code_hash
        )
                .as_bytes()
                .to_vec(),
            ),
            funds: vec![],
        }))
        .add_attribute("banana", "🍌")
        .set_data(vec![x, y])
}

pub fn b(
    _deps: DepsMut,
    _env: Env,
    contract_addr: String,
    code_hash: String,
    x: u8,
    y: u8,
) -> Response {
    Response::new()
        .add_message(CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: contract_addr.clone(),
            code_hash,
            msg: Binary::from(
                format!("{{\"c\":{{\"x\":{} ,\"y\": {} }}}}", x + 1, y + 1)
                    .as_bytes()
                    .to_vec(),
            ),
            funds: vec![],
        }))
        .add_attribute("kiwi", "🥝")
        .set_data(vec![x + y])
}

pub fn c(_deps: DepsMut, _env: Env, x: u8, y: u8) -> Response {
    Response::new()
        .add_attribute("watermelon", "🍉")
        .set_data(vec![x + y])
}

pub fn empty_log_key_value(_deps: DepsMut, _env: Env) -> Response {
    Response::new().add_attributes(vec![
        attr("my value is empty", ""),
        attr("", "my key is empty"),
    ])
}

pub fn empty_data(_deps: DepsMut, _env: Env) -> Response {
    Response::new().set_data(vec![])
}

pub fn unicode_data(_deps: DepsMut, _env: Env) -> Response {
    Response::new().set_data("🍆🥑🍄".as_bytes().to_vec())
}

pub fn no_data(_deps: DepsMut, _env: Env) -> Response {
    Response::new()
}

pub fn exec_callback_to_init(
    _deps: DepsMut,
    _env: Env,
    code_id: u64,
    code_hash: String,
) -> Response {
    Response::new()
        .add_message(CosmosMsg::Wasm(WasmMsg::Instantiate {
            code_id,
            msg: Binary::from("{\"nop\":{}}".as_bytes().to_vec()),
            code_hash,
            funds: vec![],
            label: String::from("hi"),
        }))
        .add_attribute("instantiating a new contract", "🪂")
}

fn exec_with_callback_contract_error(contract_addr: String, code_hash: String) -> Response {
    Response::new()
        .add_message(CosmosMsg::Wasm(WasmMsg::Execute {
            contract_addr: contract_addr.clone(),
            code_hash,
            msg: Binary::from(
                r#"{"contract_error":{"error_type":"generic_err"}}"#
                    .as_bytes()
                    .to_vec(),
            ),
            funds: vec![],
        }))
        .add_attribute("exec with a callback with contract error", "🤷‍♂️")
}

fn allocate_on_heap(bytes: usize) -> Response {
    let mut values: Vec<u8> = vec![0; bytes];
    values[bytes - 1] = 1;

    Response::new().set_data("😅".as_bytes().to_vec())
}

fn get_state(deps: DepsMut, key: String) -> Response {
    let store = PrefixedStorage::new(deps.storage, b"my_prefix");

    match store.get(key.as_bytes()) {
        Some(value) => Response::new().set_data(value),
        None => Response::default(),
    }
}

fn set_state(deps: DepsMut, key: String, value: String) -> Response {
    let mut store = PrefixedStorage::new(deps.storage, b"my_prefix");
    store.set(key.as_bytes(), value.as_bytes());
    Response::default()
}

fn remove_state(deps: DepsMut, key: String) -> Response {
    let mut store = PrefixedStorage::new(deps.storage, b"my_prefix");
    store.remove(key.as_bytes());
    Response::default()
}

#[allow(invalid_value)]
#[allow(unused_must_use)]
fn pass_null_pointer_to_imports_should_throw(deps: DepsMut, pass_type: String) -> Response {
    let null_ptr_slice: &[u8] = unsafe { MaybeUninit::zeroed().assume_init() };

    match &pass_type[..] {
        "read_db_key" => {
            deps.storage.get(null_ptr_slice);
        }
        "write_db_key" => {
            deps.storage.set(null_ptr_slice, b"write value");
        }
        "write_db_value" => {
            deps.storage.set(b"write key", null_ptr_slice);
        }
        "remove_db_key" => {
            deps.storage.remove(null_ptr_slice);
        }
        "canonicalize_address_input" => {
            deps.api
                .addr_canonicalize(unsafe { MaybeUninit::zeroed().assume_init() });
        }
        "canonicalize_address_output" => { /* TODO */ }
        "humanize_address_input" => {
            deps.api
                .addr_humanize(unsafe { MaybeUninit::zeroed().assume_init() });
        }
        "humanize_address_output" => { /* TODO */ }
        "validate_address_input" => {
            deps.api
                .addr_validate(unsafe { MaybeUninit::zeroed().assume_init() });
        }
        "validate_address_output" => { /* TODO */ }
        _ => {}
    };

    Response::default()
}

fn test_canonicalize_address_errors(deps: DepsMut) -> StdResult<Response> {
    match deps.api.addr_canonicalize("") {
        Err(StdError::GenericErr { msg }) => {
            if !msg.to_lowercase().contains("input is empty") {
                return Err(StdError::generic_err(format!(
                    "empty address should have failed with 'addr_canonicalize errored: Input is empty; got {:?}'",
                msg)));
            }
            // all is good, continue
        }
        other => {
            return Err(StdError::generic_err(
                format!("empty address should have failed with 'addr_canonicalize errored: Input is empty', instead was {:?}", other),
            ))
        }
    }

    match deps.api.addr_canonicalize("   ") {
        Err(StdError::GenericErr { msg }) => {
            if !msg.to_lowercase().contains("invalid length") {
                return Err(StdError::generic_err(format!(
                    "empty trimmed address should have failed with 'addr_canonicalize errored: invalid length; got {:?}'",
                    msg)));
            }
            // all is good, continue
        }
        other => {
            return Err(StdError::generic_err(
                format!("empty trimmed address should have failed with 'addr_canonicalize errored: invalid length', instead was: {:?}", other),
            ))
        }
    }

    match deps.api.addr_canonicalize("cosmos1h99hrcc54ms9lxxxx") {
        Err(StdError::GenericErr { msg })
            if msg == String::from("addr_canonicalize errored: invalid checksum") =>
        {
            // all is good, continue
        }
        other => {
            return Err(StdError::generic_err(
                format!("bad bech32 should have failed with 'addr_canonicalize errored: invalid checksum', instead was {:?}", other),
            ))
        }
    }

    match deps.api.addr_canonicalize("cosmos1h99hrcc54ms9luwpex9kw0rwdt7etvfdyxh6gu") {
        Err(StdError::GenericErr { msg })
            if msg == String::from("addr_canonicalize errored: wrong address prefix: \"cosmos\"") =>
        {
            // all is good, continue
        }
        other => {
            return Err(StdError::generic_err(
                format!("bad prefix should have failed with 'addr_canonicalize errored: wrong address prefix: \"cosmos\"', instead was {:?}", other),
            ))
        }
    }

    Ok(Response::new().set_data("🤟".as_bytes().to_vec()))
}

/////////////////////////////// Query ///////////////////////////////

fn send_external_query_infinite_loop(
    deps: Deps,
    contract_addr: String,
    code_hash: String,
) -> StdResult<Binary> {
    let answer = deps
        .querier
        .query::<Binary>(&QueryRequest::Wasm(WasmQuery::Smart {
            contract_addr: contract_addr.clone(),
            code_hash: code_hash.clone(),
            msg: Binary::from(
                format!(
                    r#"{{"send_external_query_infinite_loop":{{"to":"{}", "code_hash":"{}"}}}}"#,
                    contract_addr.clone().to_string(),
                    &code_hash
                )
                .as_bytes()
                .to_vec(),
            ),
        }));

    match answer {
        Ok(wtf) => Ok(Binary(wtf.into())),
        Err(e) => Err(e),
    }
}

fn write_to_storage_in_query(storage: &dyn Storage) -> StdResult<Binary> {
    #[allow(clippy::cast_ref_to_mut)]
    let storage = unsafe { &mut *(storage as *const _ as *mut dyn Storage) };
    storage.set(b"abcd", b"dcba");

    Ok(Binary(vec![]))
}

fn remove_from_storage_in_query(storage: &dyn Storage) -> StdResult<Binary> {
    #[allow(clippy::cast_ref_to_mut)]
    let storage = unsafe { &mut *(storage as *const _ as *mut dyn Storage) };
    storage.remove(b"abcd");

    Ok(Binary(vec![]))
}

//// consts

const REALLY_LONG: &[u8] = b"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
