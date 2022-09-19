use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

use cosmwasm_std::{
    entry_point, from_slice, to_binary, to_vec, Binary, Deps, DepsMut, Env, MessageInfo, Order,
    QueryResponse, Response, StdResult, Storage,
};

use crate::msg::{InstantiateMsg, MigrateMsg};

// we store one entry for each item in the queue
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct Item {
    pub value: i32,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum ExecuteMsg {
    // Enqueue will add some value to the end of list
    Enqueue { value: i32 },
    // Dequeue will remove value from start of the list
    Dequeue {},
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum QueryMsg {
    // how many items are in the queue
    Count {},
    // total of all values in the queue
    Sum {},
    // Reducer holds open two iterators at once
    Reducer {},
    List {},
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct CountResponse {
    pub count: u32,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct SumResponse {
    pub sum: i32,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
// the Vec contains pairs for every element in the queue
// (value of item i, sum of all elements where value > value[i])
pub struct ReducerResponse {
    pub counters: Vec<(i32, i32)>,
}

#[derive(Serialize, Deserialize, Clone, PartialEq, JsonSchema, Debug)]
pub struct ListResponse {
    /// List an empty range, both bounded
    pub empty: Vec<u32>,
    /// List all IDs lower than 0x20
    pub early: Vec<u32>,
    /// List all IDs starting from 0x20
    pub late: Vec<u32>,
}

// A no-op, just empty data
pub fn instantiate(
    _deps: DepsMut,
    _env: Env,
    _info: MessageInfo,
    _msg: InstantiateMsg,
) -> StdResult<Response> {
    Ok(Response::default())
}

pub fn execute(
    deps: DepsMut,
    _env: Env,
    _info: MessageInfo,
    msg: ExecuteMsg,
) -> StdResult<Response> {
    match msg {
        ExecuteMsg::Enqueue { value } => handle_enqueue(deps, value),
        ExecuteMsg::Dequeue {} => handle_dequeue(deps),
    }
}

const FIRST_KEY: u8 = 0;

fn handle_enqueue(deps: DepsMut, value: i32) -> StdResult<Response> {
    enqueue(deps.storage, value)?;
    Ok(Response::default())
}

fn enqueue(storage: &mut dyn Storage, value: i32) -> StdResult<()> {
    // find the last element in the queue and extract key
    let last_item = storage.range(None, None, Order::Descending).next();

    let new_key = match last_item {
        None => FIRST_KEY,
        Some((key, _)) => {
            key[0] + 1 // all keys are one byte
        }
    };
    let new_value = to_vec(&Item { value })?;

    storage.set(&[new_key], &new_value);
    Ok(())
}

#[allow(clippy::unnecessary_wraps)]
fn handle_dequeue(deps: DepsMut) -> StdResult<Response> {
    // find the first element in the queue and extract value
    let first = deps.storage.range(None, None, Order::Ascending).next();

    let mut res = Response::default();
    if let Some((key, value)) = first {
        // remove from storage and return old value
        deps.storage.remove(&key);
        res.data = Some(Binary(value));
        Ok(res)
    } else {
        Ok(res)
    }
}

#[cfg_attr(not(feature = "library"), entry_point)]
pub fn migrate(deps: DepsMut, _env: Env, _msg: MigrateMsg) -> StdResult<Response> {
    // clear all
    let keys: Vec<_> = deps
        .storage
        .range(None, None, Order::Ascending)
        .map(|(key, _)| key)
        .collect();
    for key in keys {
        deps.storage.remove(&key);
    }

    // Write new values
    enqueue(deps.storage, 100)?;
    enqueue(deps.storage, 101)?;
    enqueue(deps.storage, 102)?;
    Ok(Response::default())
}

pub fn query(deps: Deps, _env: Env, msg: QueryMsg) -> StdResult<QueryResponse> {
    match msg {
        QueryMsg::Count {} => to_binary(&query_count(deps)),
        QueryMsg::Sum {} => to_binary(&query_sum(deps)?),
        QueryMsg::Reducer {} => to_binary(&query_reducer(deps)?),
        QueryMsg::List {} => to_binary(&query_list(deps)),
    }
}

fn query_count(deps: Deps) -> CountResponse {
    let count = deps.storage.range(None, None, Order::Ascending).count() as u32;
    CountResponse { count }
}

fn query_sum(deps: Deps) -> StdResult<SumResponse> {
    let values: StdResult<Vec<Item>> = deps
        .storage
        .range(None, None, Order::Ascending)
        .map(|(_, v)| from_slice(&v))
        .collect();
    let sum = values?.iter().fold(0, |s, v| s + v.value);
    Ok(SumResponse { sum })
}

fn query_reducer(deps: Deps) -> StdResult<ReducerResponse> {
    let mut out: Vec<(i32, i32)> = vec![];
    // val: StdResult<Item>
    for val in deps
        .storage
        .range(None, None, Order::Ascending)
        .map(|(_, v)| from_slice::<Item>(&v))
    {
        // this returns error on parse error
        let my_val = val?.value;
        // now, let's do second iterator
        let sum: i32 = deps
            .storage
            .range(None, None, Order::Ascending)
            // get value. ignore parse errors, just count as 0
            .map(|(_, v)| {
                from_slice::<Item>(&v)
                    .map(|v| v.value)
                    .expect("error in item")
            })
            .filter(|v| *v > my_val)
            .sum();
        out.push((my_val, sum))
    }
    Ok(ReducerResponse { counters: out })
}

/// Does a range query with both bounds set. Not really useful but to debug an issue
/// between VM and Wasm: https://github.com/CosmWasm/cosmwasm/issues/508
fn query_list(deps: Deps) -> ListResponse {
    let empty: Vec<u32> = deps
        .storage
        .range(Some(b"large"), Some(b"larger"), Order::Ascending)
        .map(|(k, _)| k[0] as u32)
        .collect();
    let early: Vec<u32> = deps
        .storage
        .range(None, Some(b"\x20"), Order::Ascending)
        .map(|(k, _)| k[0] as u32)
        .collect();
    let late: Vec<u32> = deps
        .storage
        .range(Some(b"\x20"), None, Order::Ascending)
        .map(|(k, _)| k[0] as u32)
        .collect();
    ListResponse { empty, early, late }
}

#[cfg(test)]
mod tests {
    use super::*;
    use cosmwasm_std::testing::{
        mock_dependencies, mock_env, mock_info, MockApi, MockQuerier, MockStorage,
    };
    use cosmwasm_std::{coins, from_binary, OwnedDeps};

    fn create_contract() -> (OwnedDeps<MockStorage, MockApi, MockQuerier>, MessageInfo) {
        let mut deps = mock_dependencies(&coins(1000, "earth"));
        let info = mock_info("creator", &coins(1000, "earth"));
        let res = instantiate(deps.as_mut(), mock_env(), info.clone(), InstantiateMsg {}).unwrap();
        assert_eq!(0, res.messages.len());
        (deps, info)
    }

    fn get_count(deps: Deps) -> u32 {
        query_count(deps).count
    }

    fn get_sum(deps: Deps) -> i32 {
        query_sum(deps).unwrap().sum
    }

    #[test]
    fn instantiate_and_query() {
        let (deps, _) = create_contract();
        assert_eq!(get_count(deps.as_ref()), 0);
        assert_eq!(get_sum(deps.as_ref()), 0);
    }

    #[test]
    fn push_and_query() {
        let (mut deps, info) = create_contract();
        execute(
            deps.as_mut(),
            mock_env(),
            info,
            ExecuteMsg::Enqueue { value: 25 },
        )
        .unwrap();
        assert_eq!(get_count(deps.as_ref()), 1);
        assert_eq!(get_sum(deps.as_ref()), 25);
    }

    #[test]
    fn multiple_push() {
        let (mut deps, info) = create_contract();
        execute(
            deps.as_mut(),
            mock_env(),
            info.clone(),
            ExecuteMsg::Enqueue { value: 25 },
        )
        .unwrap();
        execute(
            deps.as_mut(),
            mock_env(),
            info.clone(),
            ExecuteMsg::Enqueue { value: 35 },
        )
        .unwrap();
        execute(
            deps.as_mut(),
            mock_env(),
            info,
            ExecuteMsg::Enqueue { value: 45 },
        )
        .unwrap();
        assert_eq!(get_count(deps.as_ref()), 3);
        assert_eq!(get_sum(deps.as_ref()), 105);
    }

    #[test]
    fn push_and_pop() {
        let (mut deps, info) = create_contract();
        execute(
            deps.as_mut(),
            mock_env(),
            info.clone(),
            ExecuteMsg::Enqueue { value: 25 },
        )
        .unwrap();
        execute(
            deps.as_mut(),
            mock_env(),
            info.clone(),
            ExecuteMsg::Enqueue { value: 17 },
        )
        .unwrap();
        let res = execute(deps.as_mut(), mock_env(), info, ExecuteMsg::Dequeue {}).unwrap();
        // ensure we popped properly
        assert!(res.data.is_some());
        let data = res.data.unwrap();
        let state: Item = from_slice(data.as_slice()).unwrap();
        assert_eq!(state.value, 25);

        assert_eq!(get_count(deps.as_ref()), 1);
        assert_eq!(get_sum(deps.as_ref()), 17);
    }

    #[test]
    fn push_and_reduce() {
        let (mut deps, info) = create_contract();
        execute(
            deps.as_mut(),
            mock_env(),
            info.clone(),
            ExecuteMsg::Enqueue { value: 40 },
        )
        .unwrap();
        execute(
            deps.as_mut(),
            mock_env(),
            info.clone(),
            ExecuteMsg::Enqueue { value: 15 },
        )
        .unwrap();
        execute(
            deps.as_mut(),
            mock_env(),
            info.clone(),
            ExecuteMsg::Enqueue { value: 85 },
        )
        .unwrap();
        execute(
            deps.as_mut(),
            mock_env(),
            info,
            ExecuteMsg::Enqueue { value: -10 },
        )
        .unwrap();
        assert_eq!(get_count(deps.as_ref()), 4);
        assert_eq!(get_sum(deps.as_ref()), 130);
        let counters = query_reducer(deps.as_ref()).unwrap().counters;
        assert_eq!(counters, vec![(40, 85), (15, 125), (85, 0), (-10, 140)]);
    }

    #[test]
    fn query_list() {
        let (mut deps, info) = create_contract();
        for _ in 0..0x25 {
            execute(
                deps.as_mut(),
                mock_env(),
                info.clone(),
                ExecuteMsg::Enqueue { value: 40 },
            )
            .unwrap();
        }
        for _ in 0..0x19 {
            execute(
                deps.as_mut(),
                mock_env(),
                info.clone(),
                ExecuteMsg::Dequeue {},
            )
            .unwrap();
        }
        // we add 0x25 items and then remove the first 0x19, leaving [0x19, 0x1a, 0x1b, ..., 0x24]
        // since we count up to 0x20 in early, we get early and late both with data

        let query_msg = QueryMsg::List {};
        let ids: ListResponse =
            from_binary(&query(deps.as_ref(), mock_env(), query_msg).unwrap()).unwrap();
        assert_eq!(ids.empty, Vec::<u32>::new());
        assert_eq!(ids.early, vec![0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f]);
        assert_eq!(ids.late, vec![0x20, 0x21, 0x22, 0x23, 0x24]);
    }
}
