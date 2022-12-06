//! Gas metering instrumentation.

use log::*;

use walrus::{
    ir::*, FunctionBuilder, FunctionId, GlobalId, InitExpr, LocalFunction, Module, ValType,
};

use crate::errors::{WasmEngineError, WasmEngineResult};
use crate::gas::WasmCosts;
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
pub fn add_metering(module: &mut Module, gas_costs: &WasmCosts) {
    let gas_limit_global =
        module
            .globals
            .add_local(ValType::I64, true, InitExpr::Value(Value::I64(0)));
    let gas_limit_exhausted_global =
        module
            .globals
            .add_local(ValType::I64, true, InitExpr::Value(Value::I64(0)));
    module.exports.add(EXPORT_GAS_LIMIT, gas_limit_global);
    module
        .exports
        .add(EXPORT_GAS_LIMIT_EXHAUSTED, gas_limit_exhausted_global);

    let memory_grow_meter = create_memory_grow_meter(
        module,
        gas_costs,
        gas_limit_global,
        gas_limit_exhausted_global,
    );

    for (_, func) in module.funcs.iter_local_mut() {
        transform_function(
            func,
            gas_costs,
            gas_limit_global,
            gas_limit_exhausted_global,
            memory_grow_meter,
        );
    }
}

// todo copy from pwasm_utils
/// Instruction cost function.
fn instruction_cost(_instr: &Instr, _gas_costs: &WasmCosts) -> u64 {
    // Currently default to 1 for all instructions.
    2
}

fn transform_function(
    func: &mut LocalFunction,
    gas_costs: &WasmCosts,
    gas_limit_global: GlobalId,
    gas_limit_exhausted_global: GlobalId,
    memory_grow_meter: FunctionId,
) {
    // get the list of "original" blocks before we start adding more.
    let block_ids: Vec<_> = func.blocks().map(|(block_id, _block)| block_id).collect();
    // for each block, prepend it with metering instructions
    for block_id in block_ids {
        inject_metering(
            func,
            block_id,
            gas_costs,
            gas_limit_global,
            gas_limit_exhausted_global,
            memory_grow_meter,
        );
    }
}

/// Number of injected metering instructions (needed to calculate final instruction size).
const METERING_INSTRUCTION_COUNT: usize = 8;

fn inject_metering(
    func: &mut LocalFunction,
    block_id: InstrSeqId,
    gas_costs: &WasmCosts,
    gas_limit_global: GlobalId,
    gas_limit_exhausted_global: GlobalId,
    memory_grow_meter: FunctionId,
) {
    let block = func.block_mut(block_id);
    let block_instrs = &mut block.instrs;
    let block_len = block_instrs.len();
    let block_cost: u64 = block_instrs
        .iter()
        .map(|(inst, _instr_loc)| instruction_cost(inst, gas_costs))
        .sum();
    let block_cost = block_cost as i64;

    // find all location in the block that use Instr::MemoryGrow
    let mut grow_locations = vec![];
    for (loc, (instr, _)) in block_instrs.iter().enumerate() {
        if let Instr::MemoryGrow { .. } = instr {
            grow_locations.push(loc);
        }
    }

    // Prepend instances of Instr::MemoryGrow with a call to the memory grow meter.
    // This is done in reverse because the indices are locations in the
    // underlying instruction array. Doing this in order would invalidate the
    // indices.
    for loc in grow_locations.into_iter().rev() {
        let call_grow_meter = Instr::from(Call {
            func: memory_grow_meter,
        });
        // using Default is fine - it's the same as what `InstrSeqBuilder::instr_at` does.
        block_instrs.insert(loc, (call_grow_meter, Default::default()));
    }

    let builder = func.builder_mut();
    let mut builder = builder.dangling_instr_seq(None);
    let seq = builder
        // if unsigned(globals[gas_limit]) < unsigned(block_cost) { throw(); }
        .global_get(gas_limit_global)
        .i64_const(block_cost)
        .binop(BinaryOp::I64LtU)
        .if_else(
            None,
            |then| {
                then.i64_const(block_cost)
                    .global_set(gas_limit_exhausted_global)
                    .unreachable();
            },
            |_else| {},
        )
        // globals[gas_limit] -= block_cost;
        .global_get(gas_limit_global)
        .i64_const(block_cost)
        .binop(BinaryOp::I64Sub)
        .global_set(gas_limit_global);

    let mut new_instrs = Vec::with_capacity(block_len + METERING_INSTRUCTION_COUNT);
    new_instrs.append(seq.instrs_mut());

    let block = func.block_mut(block_id);
    new_instrs.extend_from_slice(block);
    block.instrs = new_instrs;
}

fn create_memory_grow_meter(
    module: &mut Module,
    gas_costs: &WasmCosts,
    gas_limit_global: GlobalId,
    gas_limit_exhausted_global: GlobalId,
) -> FunctionId {
    // function input
    let num_pages = module.locals.add(ValType::I32);
    // cache cost of memory grow
    let grow_cost = module.locals.add(ValType::I64);

    let mut func = FunctionBuilder::new(&mut module.types, &[ValType::I32], &[ValType::I32]);

    func.func_body()
        // multiply the number of pages by the grow cost
        .local_get(num_pages)
        // num_pages as i64
        .unop(UnaryOp::I64ExtendSI32)
        .i64_const(gas_costs.grow_mem as i64)
        .binop(BinaryOp::I64Mul)
        // save the cost
        .local_set(grow_cost)
        // from here it's very similar to the code in `fn inject_metering()`.
        // if unsigned(globals[gas_limit]) < unsigned(grow_cost) { throw(); }
        .global_get(gas_limit_global)
        .local_get(grow_cost)
        .binop(BinaryOp::I64LtU)
        .if_else(
            None,
            |then| {
                then.local_get(grow_cost)
                    .global_set(gas_limit_exhausted_global)
                    .unreachable();
            },
            |_else| {},
        )
        // globals[gas_limit] -= grow_cost;
        .global_get(gas_limit_global)
        .local_get(grow_cost)
        .binop(BinaryOp::I64Sub)
        .global_set(gas_limit_global)
        // return the original number of pages for the MemoryGrow instruction
        // right after this function call.
        .local_get(num_pages);

    // register the function
    func.finish(vec![num_pages], &mut module.funcs)
}
