use snafu::ResultExt;

use cosmwasm::serde::{from_slice, to_vec};
use cosmwasm::traits::Api;
use cosmwasm::types::{ContractResult, Env, QueryResult};

use crate::errors::{Error, ParseErr, /*RuntimeErr,*/ SerializeErr};
use crate::instance::Instance;
use crate::Storage;

pub fn call_init<S: Storage + 'static, A: Api + 'static>(
    instance: &mut Instance<S, A>,
    env: &Env,
    msg: &[u8],
) -> Result<ContractResult, Error> {
    let env = to_vec(env).context(SerializeErr {})?;
    let data = call_init_raw(instance, &env, msg)?;
    let res: ContractResult = from_slice(&data).context(ParseErr {})?;
    Ok(res)
}

pub fn call_handle<S: Storage + 'static, A: Api + 'static>(
    instance: &mut Instance<S, A>,
    env: &Env,
    msg: &[u8],
) -> Result<ContractResult, Error> {
    let env = to_vec(env).context(SerializeErr {})?;
    let data = call_handle_raw(instance, &env, msg)?;
    let res: ContractResult = from_slice(&data).context(ParseErr {})?;
    Ok(res)
}

pub fn call_query<S: Storage + 'static, A: Api + 'static>(
    instance: &mut Instance<S, A>,
    msg: &[u8],
) -> Result<QueryResult, Error> {
    let data = call_query_raw(instance, msg)?;
    let res: QueryResult = from_slice(&data).context(ParseErr {})?;
    Ok(res)
}

pub fn call_query_raw<S: Storage + 'static, A: Api + 'static>(
    instance: &mut Instance<S, A>,
    msg: &[u8],
) -> Result<Vec<u8>, Error> {
    instance.call_query(msg).into()
}

pub fn call_init_raw<S: Storage + 'static, A: Api + 'static>(
    instance: &mut Instance<S, A>,
    env: &[u8],
    msg: &[u8],
) -> Result<Vec<u8>, Error> {
    instance.call_init(env, msg).into()
}

pub fn call_handle_raw<S: Storage + 'static, A: Api + 'static>(
    instance: &mut Instance<S, A>,
    env: &[u8],
    msg: &[u8],
) -> Result<Vec<u8>, Error> {
    instance.call_handle(env, msg).into()
}


/*
fn call_raw<S: Storage + 'static, A: Api + 'static>(
    instance: &mut Instance<S, A>,
    name: &str,
    env: &[u8],
    msg: &[u8],
) -> Result<Vec<u8>, Error> {
    let param_offset = instance.allocate(env)?;
    let msg_offset = instance.allocate(msg)?;

    let func: Func<(u32, u32), u32> = instance.func(name)?;
    let res_offset = func.call(param_offset, msg_offset).context(RuntimeErr {})?;

    let data = instance.memory(res_offset);
    // free return value in wasm (arguments were freed in wasm code)
    instance.deallocate(res_offset)?;
    Ok(data)
}
*/
