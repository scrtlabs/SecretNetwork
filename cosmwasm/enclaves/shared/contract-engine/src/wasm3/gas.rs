//! Gas metering instrumentation.

use log::*;

use walrus::{ir::*, GlobalId, LocalFunction, Module};

use crate::errors::{WasmEngineError, WasmEngineResult};
use enclave_ffi_types::EnclaveError;

/// Name of the exported global that holds the gas limit.
pub const EXPORT_GAS_LIMIT: &str = "gas_limit";
/// Name of the exported global that holds the gas limit exhausted flag.
pub const EXPORT_GAS_LIMIT_EXHAUSTED: &str = "gas_limit_exhausted";

/// Configures the gas limit on the given instance.
pub fn set_gas_limit<C>(instance: &wasm3::Instance<C>, gas_limit: u64) -> Result<(), EnclaveError> {
    instance
        .set_global(EXPORT_GAS_LIMIT, gas_limit)
        .map_err(|_err| EnclaveError::FailedGasMeteringInjection)
}

/// Returns the remaining gas.
pub fn get_remaining_gas<C>(instance: &wasm3::Instance<C>) -> u64 {
    instance.get_global(EXPORT_GAS_LIMIT).unwrap_or_default()
}

/// Returns the amount of gas requested that was over the limit.
pub fn get_exhausted_amount<C>(instance: &wasm3::Instance<C>) -> u64 {
    instance
        .get_global(EXPORT_GAS_LIMIT_EXHAUSTED)
        .unwrap_or_default()
}

/// Attempts to use the given amount of gas.
pub fn use_gas<C>(instance: &wasm3::Instance<C>, amount: u64) -> WasmEngineResult<()> {
    debug!("external service used gas: {}", amount);
    let gas_limit: u64 = instance
        .get_global(EXPORT_GAS_LIMIT)
        .map_err(|_| WasmEngineError::OutOfGas)?;
    if gas_limit < amount {
        let _ = instance.set_global(EXPORT_GAS_LIMIT_EXHAUSTED, amount);
        return Err(WasmEngineError::OutOfGas);
    }
    instance
        .set_global(EXPORT_GAS_LIMIT, gas_limit - amount)
        .map_err(|_| WasmEngineError::OutOfGas)?;
    Ok(())
}

/// Inject gas metering instrumentation into the module.
pub fn add_metering(module: &mut Module) {
    let gas_limit_global = module.globals.add_local(
        walrus::ValType::I64,
        true,
        walrus::InitExpr::Value(Value::I64(0)),
    );
    let gas_limit_exhausted_global = module.globals.add_local(
        walrus::ValType::I64,
        true,
        walrus::InitExpr::Value(Value::I64(0)),
    );
    module.exports.add(EXPORT_GAS_LIMIT, gas_limit_global);
    module
        .exports
        .add(EXPORT_GAS_LIMIT_EXHAUSTED, gas_limit_exhausted_global);

    for (_, func) in module.funcs.iter_local_mut() {
        transform_function(func, gas_limit_global, gas_limit_exhausted_global);
    }
}

// todo copy from pwasm_utils
/// Instruction cost function.
fn instruction_cost(_instr: &Instr) -> u64 {
    // Currently default to 1 for all instructions.
    1
}

fn transform_function(
    func: &mut LocalFunction,
    gas_limit_global: GlobalId,
    gas_limit_exhausted_global: GlobalId,
) {
    let block_ids: Vec<_> = func.blocks().map(|(block_id, _block)| block_id).collect();
    for block_id in block_ids {
        inject_metering(func, block_id, gas_limit_global, gas_limit_exhausted_global);
    }
}

/// Number of injected metering instructions (needed to calculate final instruction size).
const METERING_INSTRUCTION_COUNT: usize = 8;

fn inject_metering(
    func: &mut LocalFunction,
    block_id: InstrSeqId,
    gas_limit_global: GlobalId,
    gas_limit_exhausted_global: GlobalId,
) {
    let block = func.block(block_id);
    let block_instrs = &block.instrs;
    let block_len = block_instrs.len();
    let block_cost: u64 = block_instrs
        .iter()
        .map(|(inst, _instr_loc)| instruction_cost(inst))
        .sum();

    let builder = func.builder_mut();

    let mut builder = builder.dangling_instr_seq(None);
    let seq = builder
        // if unsigned(globals[gas_limit]) < unsigned(block_cost) { throw(); }
        .global_get(gas_limit_global)
        .i64_const(block_cost as i64)
        .binop(BinaryOp::I64LtU)
        .if_else(
            None,
            |then| {
                then.i64_const(block_cost as i64)
                    .global_set(gas_limit_exhausted_global)
                    .unreachable();
            },
            |_else| {},
        )
        // globals[gas_limit] -= block_cost;
        .global_get(gas_limit_global)
        .i64_const(block_cost as i64)
        .binop(BinaryOp::I64Sub)
        .global_set(gas_limit_global);

    let mut new_instrs = Vec::with_capacity(block_len + METERING_INSTRUCTION_COUNT);
    new_instrs.append(seq.instrs_mut());

    let block = func.block_mut(block_id);
    new_instrs.extend_from_slice(&block);
    block.instrs = new_instrs;
}
