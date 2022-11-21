use benches::cpu::do_cpu_loop;

use crate::benches;
use crate::benches::allocate::do_allocate_large_memory;
use crate::benches::read_storage::{
    bench_read_large_key_from_storage, bench_read_storage_different_key,
    bench_read_storage_same_key, setup_read_large_from_storage,
};
use crate::benches::write_storage::{
    bench_write_large_storage_key, bench_write_storage_different_key,
};
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
        ExecuteMsg::BenchReadStorageMultipleKeys {} => bench_read_storage_different_key(deps, 100),
        ExecuteMsg::BenchAllocate {} => do_allocate_large_memory(),
        // start with running large item bench once, otherwise cache will skew performance numbers
        ExecuteMsg::BenchWriteLargeItemToStorage { .. } => bench_write_large_storage_key(deps, 1),
        ExecuteMsg::BenchReadLargeItemFromStorage { .. } => {
            bench_read_large_key_from_storage(deps, 1)
        }
        ExecuteMsg::SetupReadLargeItem { .. } => setup_read_large_from_storage(deps),
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
