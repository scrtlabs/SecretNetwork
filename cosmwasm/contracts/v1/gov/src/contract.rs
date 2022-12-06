use cosmwasm_std::{
    log, to_binary, Api, Binary, CosmosMsg, Env, Extern, GovMsg, GovQuery, HandleResponse,
    HumanAddr, InitResponse, InitResult, ProposalsResponse, Querier, StdResult, Storage,
    VoteOption,
};

use crate::msg::{HandleMsg, InitMsg};

pub fn init<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    _msg: InitMsg,
) -> InitResult {
    Ok(InitResponse::default())
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
    msg: HandleMsg,
) -> StdResult<HandleResponse> {
    match msg {
        HandleMsg::Proposals {} => try_query_proposal(deps, env),
        HandleMsg::Vote {} => try_vote(deps, env),
    }
}

pub fn try_vote<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
) -> StdResult<HandleResponse> {
    Ok(HandleResponse {
        messages: vec![CosmosMsg::Gov(GovMsg::Vote {
            proposal: 1,
            vote_option: VoteOption::Yes,
        })],
        log: vec![],
        data: None,
    })
}

pub fn try_query_proposal<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
) -> StdResult<HandleResponse> {
    let query = GovQuery::Proposals {};

    let query_rewards: ProposalsResponse = deps
        .querier
        .query(&query.into())
        .unwrap_or_else(|_| ProposalsResponse { proposals: vec![] });

    let active_proposal = query_rewards.proposals.len() as u64;

    Ok(HandleResponse {
        messages: vec![],
        log: vec![],
        data: Some(Binary::from(active_proposal.to_be_bytes().to_vec())),
    })
}
