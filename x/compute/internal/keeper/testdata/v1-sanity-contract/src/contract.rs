use cosmwasm_std::{
    attr, coins, entry_point, to_binary, BankMsg, Binary, CosmosMsg, Deps, DepsMut, Env,
    MessageInfo, QueryRequest, Reply, ReplyOn, Response, StdError, StdResult, SubMsg, SubMsgResult,
    WasmMsg, WasmQuery,
};

use crate::msg::{ExecuteMsg, InstantiateMsg, QueryMsg, QueryRes};
use crate::state::{count, count_read, expiration, expiration_read};

#[entry_point]
pub fn instantiate(
    deps: DepsMut,
    env: Env,
    _info: MessageInfo,
    msg: InstantiateMsg,
) -> StdResult<Response> {
    match msg {
        InstantiateMsg::Counter { counter, expires } => {
            count(deps.storage).save(&counter)?;
            let expires = env.block.height + expires;
            expiration(deps.storage).save(&expires)?;
            let mut resp = Response::default();
            resp.data = Some(env.contract.address.as_bytes().into());
            Ok(resp)
        }

        // These were ported from the v0.10 test-contract:
        InstantiateMsg::Nop {} => Ok(Response::new().add_attribute("init", "🌈")),
        InstantiateMsg::Callback {
            contract_addr,
            code_hash,
        } => todo!(),
        InstantiateMsg::CallbackContractError {
            contract_addr,
            code_hash,
        } => todo!(),
        InstantiateMsg::ContractError { error_type } => todo!(),
        InstantiateMsg::NoLogs {} => todo!(),
        InstantiateMsg::CallbackToInit { code_id, code_hash } => todo!(),
        InstantiateMsg::CallbackBadParams {
            contract_addr,
            code_hash,
        } => todo!(),
        InstantiateMsg::Panic {} => todo!(),
        InstantiateMsg::SendExternalQueryDepthCounter {
            to,
            depth,
            code_hash,
        } => todo!(),
        InstantiateMsg::SendExternalQueryRecursionLimit {
            to,
            depth,
            code_hash,
        } => todo!(),
        InstantiateMsg::CallToInit {
            code_id,
            code_hash,
            label,
            msg,
        } => todo!(),
        InstantiateMsg::CallToExec {
            addr,
            code_hash,
            msg,
        } => todo!(),
        InstantiateMsg::CallToQuery {
            addr,
            code_hash,
            msg,
        } => todo!(),
    }
}

#[entry_point]
pub fn execute(
    deps: DepsMut,
    env: Env,
    _info: MessageInfo,
    msg: ExecuteMsg,
) -> StdResult<Response> {
    match msg {
        ExecuteMsg::Increment { addition } => increment(deps, addition),
        ExecuteMsg::TransferMoney { amount } => transfer_money(deps, amount),
        ExecuteMsg::RecursiveReply {} => recursive_reply(env, deps),
        ExecuteMsg::RecursiveReplyFail {} => recursive_reply_fail(env, deps),
        ExecuteMsg::InitNewContract {} => init_new_contract(env, deps),

        // These were ported from the v0.10 test-contract:
        ExecuteMsg::A {
            contract_addr,
            code_hash,
            x,
            y,
        } => todo!(),
        ExecuteMsg::B {
            contract_addr,
            code_hash,
            x,
            y,
        } => todo!(),
        ExecuteMsg::C { x, y } => todo!(),
        ExecuteMsg::UnicodeData {} => todo!(),
        ExecuteMsg::EmptyLogKeyValue {} => todo!(),
        ExecuteMsg::EmptyData {} => todo!(),
        ExecuteMsg::NoData {} => todo!(),
        ExecuteMsg::ContractError { error_type } => todo!(),
        ExecuteMsg::NoLogs {} => todo!(),
        ExecuteMsg::CallbackToInit { code_id, code_hash } => todo!(),
        ExecuteMsg::CallbackContractError {
            contract_addr,
            code_hash,
        } => todo!(),
        ExecuteMsg::CallbackBadParams {
            contract_addr,
            code_hash,
        } => todo!(),
        ExecuteMsg::SetState { key, value } => todo!(),
        ExecuteMsg::GetState { key } => todo!(),
        ExecuteMsg::RemoveState { key } => todo!(),
        ExecuteMsg::TestCanonicalizeAddressErrors {} => todo!(),
        ExecuteMsg::Panic {} => todo!(),
        ExecuteMsg::AllocateOnHeap { bytes } => todo!(),
        ExecuteMsg::PassNullPointerToImportsShouldThrow { pass_type } => todo!(),
        ExecuteMsg::SendExternalQuery { to, code_hash } => todo!(),
        ExecuteMsg::SendExternalQueryPanic { to, code_hash } => todo!(),
        ExecuteMsg::SendExternalQueryError { to, code_hash } => todo!(),
        ExecuteMsg::SendExternalQueryBadAbi { to, code_hash } => todo!(),
        ExecuteMsg::SendExternalQueryBadAbiReceiver { to, code_hash } => todo!(),
        ExecuteMsg::LogMsgSender {} => todo!(),
        ExecuteMsg::CallbackToLogMsgSender { to, code_hash } => todo!(),
        ExecuteMsg::DepositToContract {} => todo!(),
        ExecuteMsg::SendFunds {
            amount,
            denom,
            to,
            from,
        } => todo!(),
        ExecuteMsg::SendFundsToInitCallback {
            amount,
            denom,
            code_id,
            code_hash,
        } => todo!(),
        ExecuteMsg::SendFundsToExecCallback {
            amount,
            denom,
            to,
            code_hash,
        } => todo!(),
        ExecuteMsg::Sleep { ms } => todo!(),
        ExecuteMsg::SendExternalQueryDepthCounter {
            to,
            code_hash,
            depth,
        } => todo!(),
        ExecuteMsg::SendExternalQueryRecursionLimit {
            to,
            code_hash,
            depth,
        } => todo!(),
        ExecuteMsg::WithFloats { x, y } => todo!(),
        ExecuteMsg::CallToInit {
            code_id,
            code_hash,
            label,
            msg,
        } => todo!(),
        ExecuteMsg::CallToExec {
            addr,
            code_hash,
            msg,
        } => todo!(),
        ExecuteMsg::CallToQuery {
            addr,
            code_hash,
            msg,
        } => todo!(),
        ExecuteMsg::StoreReallyLongKey {} => todo!(),
        ExecuteMsg::StoreReallyShortKey {} => todo!(),
        ExecuteMsg::StoreReallyLongValue {} => todo!(),
        ExecuteMsg::Secp256k1Verify {
            pubkey,
            sig,
            msg_hash,
            iterations,
        } => todo!(),
        ExecuteMsg::Secp256k1VerifyFromCrate {
            pubkey,
            sig,
            msg_hash,
            iterations,
        } => todo!(),
        ExecuteMsg::Ed25519Verify {
            pubkey,
            sig,
            msg,
            iterations,
        } => todo!(),
        ExecuteMsg::Ed25519BatchVerify {
            pubkeys,
            sigs,
            msgs,
            iterations,
        } => todo!(),
        ExecuteMsg::Secp256k1RecoverPubkey {
            msg_hash,
            sig,
            recovery_param,
            iterations,
        } => todo!(),
        ExecuteMsg::Secp256k1Sign {
            msg,
            privkey,
            iterations,
        } => todo!(),
        ExecuteMsg::Ed25519Sign {
            msg,
            privkey,
            iterations,
        } => todo!(),
    }
}

pub fn increment(deps: DepsMut, c: u64) -> StdResult<Response> {
    if c == 0 {
        return Err(StdError::generic_err("got wrong counter"));
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
            admin: None,
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

#[entry_point]
pub fn query(deps: Deps, env: Env, msg: QueryMsg) -> StdResult<Binary> {
    match msg {
        QueryMsg::Get {} => to_binary(&get(deps, env)?),
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
                    contract_addr: x.to_string(),
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
