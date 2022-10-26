use benches::cpu::do_cpu_loop;

use crate::benches;
use crate::benches::allocate::do_allocate_large_memory;
use crate::benches::read_storage::bench_read_storage_same_key;
use crate::benches::write_storage::bench_write_storage_different_key;
use cosmwasm_std::{
    entry_point, Binary, Deps, DepsMut, Env, MessageInfo, Reply, Response, StdResult,
};

use crate::msg::{ExecuteMsg, InstantiateMsg, QueryMsg};

#[entry_point]
pub fn instantiate(
    _deps: DepsMut,
    _env: Env,
    _info: MessageInfo,
    _msg: InstantiateMsg,
) -> StdResult<Response> {
    Ok(Response::default())
}

#[entry_point]
pub fn execute(
    deps: DepsMut,
    _env: Env,
    _info: MessageInfo,
    msg: ExecuteMsg,
) -> StdResult<Response> {
    let _ = match msg {
        ExecuteMsg::Noop {} => Ok(()),
        ExecuteMsg::BenchCPU {} => do_cpu_loop(5000),
        ExecuteMsg::BenchReadStorage {} => bench_read_storage_same_key(deps, 100),
        ExecuteMsg::BenchWriteStorage {} => bench_write_storage_different_key(deps, 100),
        ExecuteMsg::BenchAllocate {} => do_allocate_large_memory(),
        ExecuteMsg::BenchReadLargeItemFromStorage { .. } => Ok(()),
        ExecuteMsg::BenchWriteLargeItemToStorage { .. } => Ok(()),
    };

    Ok(Response::default())
}

#[entry_point]
pub fn query(_deps: Deps, _env: Env, _msg: QueryMsg) -> StdResult<Binary> {
    Ok(Binary::default())
}

#[entry_point]
pub fn reply(_deps: DepsMut, _env: Env, _reply: Reply) -> StdResult<Response> {
    Ok(Response::default())
}
