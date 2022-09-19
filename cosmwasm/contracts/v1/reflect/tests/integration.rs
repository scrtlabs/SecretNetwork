//! This integration test tries to run and call the generated wasm.
//! It depends on a Wasm build being available, which you can create with `cargo wasm`.
//! Then running `cargo integration-test` will validate we can properly call into that generated Wasm.
//!
//! You can easily convert unit tests to integration tests as follows:
//! 1. Copy them over verbatim
//! 2. Then change
//!      let mut deps = mock_dependencies(20, &[]);
//!    to
//!      let mut deps = mock_instance(WASM, &[]);
//! 3. If you access raw storage, where ever you see something like:
//!      deps.storage.get(CONFIG_KEY).expect("no data stored");
//!    replace it with:
//!      deps.with_storage(|store| {
//!          let data = store.get(CONFIG_KEY).expect("no data stored");
//!          //...
//!      });
//! 4. Anywhere you see query(&deps, ...) you must replace it with query(&mut deps, ...)

use cosmwasm_std::{
    coin, coins, from_binary, BankMsg, Binary, Coin, ContractResult, Event, Reply, Response,
    StakingMsg, SubMsg, SubMsgExecutionResponse, SystemResult,
};
use cosmwasm_vm::{
    testing::{
        execute, instantiate, mock_env, mock_info, mock_instance, mock_instance_options, query,
        reply, MockApi, MockQuerier, MockStorage, MOCK_CONTRACT_ADDR,
    },
    Backend, Instance,
};

use reflect::msg::{
    CapitalizedResponse, CustomMsg, ExecuteMsg, InstantiateMsg, OwnerResponse, QueryMsg,
    SpecialQuery,
};
use reflect::testing::custom_query_execute;

// This line will test the output of cargo wasm
static WASM: &[u8] = include_bytes!("../target/wasm32-unknown-unknown/release/reflect.wasm");
// You can uncomment this line instead to test productionified build from cosmwasm-opt
// static WASM: &[u8] = include_bytes!("../contract.wasm");

/// A drop-in replacement for cosmwasm_vm::testing::mock_dependencies
/// that supports SpecialQuery.
pub fn mock_dependencies_with_custom_querier(
    contract_balance: &[Coin],
) -> Backend<MockApi, MockStorage, MockQuerier<SpecialQuery>> {
    let custom_querier: MockQuerier<SpecialQuery> =
        MockQuerier::new(&[(MOCK_CONTRACT_ADDR, contract_balance)])
            .with_custom_handler(|query| SystemResult::Ok(custom_query_execute(query)));

    Backend {
        api: MockApi::default(),
        storage: MockStorage::default(),
        querier: custom_querier,
    }
}

#[test]
fn proper_initialization() {
    let mut deps = mock_instance(WASM, &[]);

    let msg = InstantiateMsg {};
    let info = mock_info("creator", &coins(1000, "earth"));

    // we can just call .unwrap() to assert this was a success
    let res: Response<CustomMsg> = instantiate(&mut deps, mock_env(), info, msg).unwrap();
    assert_eq!(0, res.messages.len());

    // it worked, let's query the state
    let res = query(&mut deps, mock_env(), QueryMsg::Owner {}).unwrap();
    let value: OwnerResponse = from_binary(&res).unwrap();
    assert_eq!("creator", value.owner.as_str());
}

#[test]
fn reflect() {
    let mut deps = mock_instance(WASM, &[]);

    let msg = InstantiateMsg {};
    let info = mock_info("creator", &coins(2, "token"));
    let _res: Response<CustomMsg> = instantiate(&mut deps, mock_env(), info, msg).unwrap();

    let payload = vec![
        BankMsg::Send {
            to_address: String::from("friend"),
            amount: coins(1, "token"),
        }
        .into(),
        // make sure we can pass through custom native messages
        CustomMsg::Raw(Binary(b"{\"foo\":123}".to_vec())).into(),
        CustomMsg::Debug("Hi, Dad!".to_string()).into(),
        StakingMsg::Delegate {
            validator: String::from("validator"),
            amount: coin(100, "ustake"),
        }
        .into(),
    ];
    let msg = ExecuteMsg::ReflectMsg {
        msgs: payload.clone(),
    };
    let info = mock_info("creator", &[]);
    let res: Response<CustomMsg> = execute(&mut deps, mock_env(), info, msg).unwrap();

    // should return payload
    let payload: Vec<_> = payload.into_iter().map(SubMsg::new).collect();
    assert_eq!(payload, res.messages);
}

#[test]
fn reflect_requires_owner() {
    let mut deps = mock_instance(WASM, &[]);

    let msg = InstantiateMsg {};
    let info = mock_info("creator", &coins(2, "token"));
    let _res: Response<CustomMsg> = instantiate(&mut deps, mock_env(), info, msg).unwrap();

    // signer is not owner
    let payload = vec![BankMsg::Send {
        to_address: String::from("friend"),
        amount: coins(1, "token"),
    }
    .into()];
    let msg = ExecuteMsg::ReflectMsg { msgs: payload };

    let info = mock_info("someone", &[]);
    let res: ContractResult<Response<CustomMsg>> = execute(&mut deps, mock_env(), info, msg);
    let msg = res.unwrap_err();
    assert!(msg.contains("Permission denied: the sender is not the current owner"));
}

#[test]
fn transfer() {
    let mut deps = mock_instance(WASM, &[]);

    let msg = InstantiateMsg {};
    let info = mock_info("creator", &coins(2, "token"));
    let _res: Response<CustomMsg> = instantiate(&mut deps, mock_env(), info, msg).unwrap();

    let info = mock_info("creator", &[]);
    let new_owner = String::from("friend");
    let msg = ExecuteMsg::ChangeOwner { owner: new_owner };
    let res: Response<CustomMsg> = execute(&mut deps, mock_env(), info, msg).unwrap();

    // should change state
    assert_eq!(0, res.messages.len());
    let res = query(&mut deps, mock_env(), QueryMsg::Owner {}).unwrap();
    let value: OwnerResponse = from_binary(&res).unwrap();
    assert_eq!("friend", value.owner.as_str());
}

#[test]
fn transfer_requires_owner() {
    let mut deps = mock_instance(WASM, &[]);

    let msg = InstantiateMsg {};
    let info = mock_info("creator", &coins(2, "token"));
    let _res: Response<CustomMsg> = instantiate(&mut deps, mock_env(), info, msg).unwrap();

    let info = mock_info("random", &[]);
    let new_owner = String::from("friend");
    let msg = ExecuteMsg::ChangeOwner { owner: new_owner };

    let res: ContractResult<Response> = execute(&mut deps, mock_env(), info, msg);
    let msg = res.unwrap_err();
    assert!(msg.contains("Permission denied: the sender is not the current owner"));
}

#[test]
fn dispatch_custom_query() {
    // stub gives us defaults. Consume it and override...
    let custom = mock_dependencies_with_custom_querier(&[]);
    // we cannot use mock_instance, so we just copy and modify code from cosmwasm_vm::testing
    let (instance_options, memory_limit) = mock_instance_options();
    let mut deps = Instance::from_code(WASM, custom, instance_options, memory_limit).unwrap();

    // we don't even initialize, just trigger a query
    let res = query(
        &mut deps,
        mock_env(),
        QueryMsg::Capitalized {
            text: "demo one".to_string(),
        },
    )
    .unwrap();
    let value: CapitalizedResponse = from_binary(&res).unwrap();
    assert_eq!(value.text, "DEMO ONE");
}

#[test]
fn reflect_subcall() {
    let mut deps = mock_instance(WASM, &[]);

    let msg = InstantiateMsg {};
    let info = mock_info("creator", &coins(2, "token"));
    let _res: Response = instantiate(&mut deps, mock_env(), info, msg).unwrap();

    let id = 123u64;
    let payload = SubMsg::reply_always(
        BankMsg::Send {
            to_address: String::from("friend"),
            amount: coins(1, "token"),
        },
        id,
    );

    let msg = ExecuteMsg::ReflectSubMsg {
        msgs: vec![payload.clone()],
    };
    let info = mock_info("creator", &[]);
    let mut res: Response<CustomMsg> = execute(&mut deps, mock_env(), info, msg).unwrap();
    assert_eq!(1, res.messages.len());
    let msg = res.messages.pop().expect("must have a message");
    assert_eq!(payload, msg);
}

// this mocks out what happens after reflect_subcall
#[test]
fn reply_and_query() {
    let mut deps = mock_instance(WASM, &[]);

    let msg = InstantiateMsg {};
    let info = mock_info("creator", &coins(2, "token"));
    let _res: Response = instantiate(&mut deps, mock_env(), info, msg).unwrap();

    let id = 123u64;
    let data = Binary::from(b"foobar");
    let events = vec![Event::new("message").add_attribute("signer", "caller-addr")];
    let result = ContractResult::Ok(SubMsgExecutionResponse {
        events: events.clone(),
        data: Some(data.clone()),
    });
    let subcall = Reply { id, result };
    let res: Response = reply(&mut deps, mock_env(), subcall).unwrap();
    assert_eq!(0, res.messages.len());

    // query for a non-existant id
    let qres = query(&mut deps, mock_env(), QueryMsg::SubMsgResult { id: 65432 });
    assert!(qres.is_err());

    // query for the real id
    let raw = query(&mut deps, mock_env(), QueryMsg::SubMsgResult { id }).unwrap();
    let qres: Reply = from_binary(&raw).unwrap();
    assert_eq!(qres.id, id);
    let result = qres.result.unwrap();
    assert_eq!(result.data, Some(data));
    assert_eq!(result.events, events);
}
