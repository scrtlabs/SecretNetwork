use benches::cpu::do_cpu_loop;

use crate::benches;
use crate::benches::allocate::do_allocate_large_memory;

use crate::benches::read_storage::{
    bench_read_large_key_from_storage, bench_read_storage_different_key,
    bench_read_storage_same_key,
};
use crate::benches::write_storage::{
    bench_write_large_storage_key, bench_write_storage_different_key,
};

use cosmwasm_std::{
    entry_point, to_binary, Binary, Deps, DepsMut, Env, MessageInfo, Reply, Response, StdError,
    StdResult, Uint128,
};
use secret_toolkit::permit::{validate, Permit, TokenPermissions};

use secret_toolkit::crypto::sha_256;
use secret_toolkit::viewing_key::{ViewingKey, ViewingKeyStore};

use crate::msg::{ExecuteMsg, InstantiateMsg, QueryAnswer, QueryMsg, QueryWithPermit};
use crate::state::ContractAddressStore;

pub const BALANCE_QUERY_RESULT: u32 = 42;
pub const PREFIX_REVOKED_PERMITS: &str = "revoked_permits";

#[entry_point]
pub fn instantiate(
    deps: DepsMut,
    env: Env,
    _info: MessageInfo,
    _msg: InstantiateMsg,
) -> StdResult<Response> {
    // Keep the contract address for permit validation
    ContractAddressStore::save(deps.storage, env.contract.address)?;

    // Create a seed for viewking-key CreateViewingKey functionality
    let prng_seed_hashed = sha_256("some_prng_seed".as_bytes());
    ViewingKey::set_seed(deps.storage, &prng_seed_hashed);

    Ok(Response::default())
}

#[entry_point]
pub fn execute(deps: DepsMut, env: Env, info: MessageInfo, msg: ExecuteMsg) -> StdResult<Response> {
    let _ = match msg {
        ExecuteMsg::Noop {} => Ok(()),
        ExecuteMsg::BenchCPU {} => do_cpu_loop(5000),
        ExecuteMsg::BenchReadStorage {} => bench_read_storage_same_key(deps, 100),
        ExecuteMsg::BenchWriteStorage {} => bench_write_storage_different_key(deps, 100),
        ExecuteMsg::BenchReadStorageMultipleKeys {} => bench_read_storage_different_key(deps, 100),
        ExecuteMsg::BenchAllocate {} => do_allocate_large_memory(),
        // start with running large item bench once, otherwise cache will skew performance numbers
        ExecuteMsg::BenchReadLargeItemFromStorage { .. } => {
            bench_read_large_key_from_storage(deps, 2)
        }
        ExecuteMsg::BenchWriteLargeItemToStorage { .. } => bench_write_large_storage_key(deps, 2),
        ExecuteMsg::BenchCreateViewingKey {} => {
            create_key(deps, env, info);
            Ok(())
        }
        ExecuteMsg::BenchSetViewingKey { key, .. } => {
            set_key(deps, info, key);
            Ok(())
        }
    };

    Ok(Response::default())
}

#[entry_point]
pub fn query(deps: Deps, _env: Env, msg: QueryMsg) -> StdResult<Binary> {
    match msg {
        QueryMsg::NoopQuery {} => Ok(Binary::default()),
        QueryMsg::BenchGetBalanceWithPermit { permit, query } => {
            query_with_permit(deps, permit, query)
        }
        _ => query_with_view_key(deps, msg),
    }
}

#[entry_point]
pub fn reply(_deps: DepsMut, _env: Env, _reply: Reply) -> StdResult<Response> {
    Ok(Response::default())
}

pub fn create_key(deps: DepsMut, env: Env, info: MessageInfo) {
    ViewingKey::create(
        deps.storage,
        &info,
        &env,
        info.sender.as_str(),
        "some_entropy".as_bytes(),
    );
}

pub fn set_key(deps: DepsMut, info: MessageInfo, key: String) {
    ViewingKey::set(deps.storage, info.sender.as_str(), key.as_str());
}

fn query_with_permit(
    deps: Deps,
    permit: Permit,
    query: QueryWithPermit,
) -> Result<Binary, StdError> {
    // Validate permit content
    let token_address = ContractAddressStore::load(deps.storage)?;

    validate(
        deps,
        PREFIX_REVOKED_PERMITS,
        &permit,
        token_address.into_string(),
        None,
    )?;

    // Permit validated! We can now execute the query.
    match query {
        QueryWithPermit::Balance {} => {
            if !permit.check_permission(&TokenPermissions::Balance) {
                return Err(StdError::generic_err(format!(
                    "No permission to query balance, got permissions {:?}",
                    permit.params.permissions
                )));
            }

            to_binary(&QueryAnswer::Balance {
                amount: Uint128::from(BALANCE_QUERY_RESULT),
            })
        }
    }
}

pub fn query_with_view_key(deps: Deps, msg: QueryMsg) -> StdResult<Binary> {
    let (addresses, key) = msg.get_validation_params();

    for address in addresses {
        let result = ViewingKey::check(deps.storage, address.as_str(), key.as_str());
        if result.is_ok() {
            return match msg {
                QueryMsg::BenchGetBalanceWithViewingKey { .. } => {
                    to_binary(&QueryAnswer::Balance {
                        amount: Uint128::from(BALANCE_QUERY_RESULT),
                    })
                }
                _ => panic!("This query type does not require authentication"),
            };
        }
    }

    to_binary(&QueryAnswer::ViewingKeyError {
        msg: "Wrong viewing key for this address or viewing key not set".to_string(),
    })
}
