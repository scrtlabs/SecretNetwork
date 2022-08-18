//! Gas metering instrumentation.

use walrus::{ir::*, FunctionBuilder, GlobalId, LocalFunction, Module};

use enclave_ffi_types::EnclaveError;

/// Name of the exported global that holds the gas limit.
pub const EXPORT_GAS_LIMIT: &str = "gas_limit";
/// Name of the exported global that holds the gas limit exhausted flag.
pub const EXPORT_GAS_LIMIT_EXHAUSTED: &str = "gas_limit_exhausted";

/// Configures the gas limit on the given instance.
pub fn set_gas_limit<C>(
    instance: &wasm3::Instance<'_, '_, C>,
    gas_limit: u64,
) -> Result<(), EnclaveError> {
    instance
        .set_global(EXPORT_GAS_LIMIT, gas_limit)
        .map_err(|_err| EnclaveError::FailedGasMeteringInjection)
}

/// Returns the remaining gas.
pub fn get_remaining_gas<C>(instance: &wasm3::Instance<'_, '_, C>) -> u64 {
    instance.get_global(EXPORT_GAS_LIMIT).unwrap_or_default()
}

/// Returns the amount of gas requested that was over the limit.
pub fn get_exhausted_amount<C>(instance: &wasm3::Instance<'_, '_, C>) -> u64 {
    instance
        .get_global(EXPORT_GAS_LIMIT_EXHAUSTED)
        .unwrap_or_default()
}

/// Attempts to use the given amount of gas.
pub fn use_gas<C>(instance: &wasm3::Instance<'_, '_, C>, amount: u64) -> Result<(), wasm3::Trap> {
    let gas_limit: u64 = instance
        .get_global(EXPORT_GAS_LIMIT)
        .map_err(|_| wasm3::Trap::Abort)?;
    if gas_limit < amount {
        let _ = instance.set_global(EXPORT_GAS_LIMIT_EXHAUSTED, amount);
        return Err(wasm3::Trap::Abort);
    }
    instance
        .set_global(EXPORT_GAS_LIMIT, gas_limit - amount)
        .map_err(|_| wasm3::Trap::Abort)?;
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

// todo remove
/// A block of instructions which is metered.
#[derive(Debug)]
struct MeteredBlock {
    /// Instruction sequence where metering code should be injected.
    seq_id: InstrSeqId,
    /// Start index of instruction within the instruction sequence before which the metering code
    /// should be injected.
    start_index: usize,
    /// Instruction cost.
    cost: u64,
}

impl MeteredBlock {
    fn new(seq_id: InstrSeqId, start_index: usize) -> Self {
        Self {
            seq_id,
            start_index,
            cost: 0,
        }
    }
}

fn transform_function(
    func: &mut LocalFunction,
    gas_limit_global: GlobalId,
    gas_limit_exhausted_global: GlobalId,
) {
    let block_ids: Vec<_> = func.blocks().map(|(block_id, _block)| block_id).collect();
    for block_id in block_ids {
        let block = func.block(block_id);
        let new_block_len = block.len() + METERING_INSTRUCTION_COUNT;
        let mut new_block = Vec::with_capacity(new_block_len);

        let mut metered_block = MeteredBlock::new(block_id, 0);
        metered_block.cost = block
            .instrs
            .iter()
            .map(|(inst, _instr_loc)| instruction_cost(inst))
            .sum();

        let builder = func.builder_mut();
        inject_metering(
            builder,
            &mut new_block,
            metered_block,
            gas_limit_global,
            gas_limit_exhausted_global,
        );

        let block = func.block_mut(block_id);
        new_block.extend_from_slice(&block);
        block.instrs = new_block;
    }
}

/// Number of injected metering instructions (needed to calculate final instruction size).
const METERING_INSTRUCTION_COUNT: usize = 8;

fn inject_metering(
    builder: &mut FunctionBuilder,
    instrs: &mut Vec<(Instr, InstrLocId)>,
    block: MeteredBlock,
    gas_limit_global: GlobalId,
    gas_limit_exhausted_global: GlobalId,
) {
    let mut builder = builder.dangling_instr_seq(None);
    let seq = builder
        // if unsigned(globals[gas_limit]) < unsigned(block.cost) { throw(); }
        .global_get(gas_limit_global)
        .i64_const(block.cost as i64)
        .binop(BinaryOp::I64LtU)
        .if_else(
            None,
            |then| {
                then.i64_const(block.cost as i64)
                    .global_set(gas_limit_exhausted_global)
                    .unreachable();
            },
            |_else| {},
        )
        // globals[gas_limit] -= block.cost;
        .global_get(gas_limit_global)
        .i64_const(block.cost as i64)
        .binop(BinaryOp::I64Sub)
        .global_set(gas_limit_global);

    instrs.append(seq.instrs_mut());
}
