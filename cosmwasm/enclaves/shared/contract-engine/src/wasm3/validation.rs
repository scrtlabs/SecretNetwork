use log::*;

use walrus::Module;

use enclave_ffi_types::EnclaveError;

pub fn validate_memory(module: &mut Module) -> Result<(), EnclaveError> {
    // Verify that there is no start function defined.
    if module.start.is_some() {
        return Err(EnclaveError::WasmModuleWithStart);
    }

    // Verify that there is at most one memory defined.
    if module.memories.iter().count() > 1 {
        return Err(EnclaveError::CannotInitializeWasmMemory);
    }

    for memory in module.memories.iter_mut() {
        let requested_initial_pages: u32 = memory.initial;
        let maximum_allowed_pages: u32 = 192; // 12 MiB

        if requested_initial_pages > maximum_allowed_pages {
            error!(
                "WASM Requested to initialize with {} pages, maximum allowed is {}",
                requested_initial_pages, maximum_allowed_pages
            );
            return Err(EnclaveError::CannotInitializeWasmMemory);
        }

        memory.maximum = Some(maximum_allowed_pages);
    }

    Ok(())
}

pub fn deny_floating_point(module: &Module) -> Result<(), EnclaveError> {
    for func in module.funcs.iter() {
        use walrus::FunctionKind::*;
        match &func.kind {
            Local(func) => {
                for (_block_id, block) in func.blocks() {
                    deny_fp_block(module, block)?
                }
            }
            Import(walrus::ImportedFunction { ty, .. }) | Uninitialized(ty) => {
                let ty = module.types.get(*ty);
                for val_type in ty.params().iter().chain(ty.results().iter()) {
                    deny_fp_valtype(*val_type)?;
                }
            }
        }
    }

    Ok(())
}

fn deny_fp_block(module: &Module, block: &walrus::ir::InstrSeq) -> Result<(), EnclaveError> {
    use walrus::ir::*;
    for (instr, _instr_loc_id) in &block.instrs {
        match instr {
            Instr::LocalGet(LocalGet { local }) => deny_fp_local(module, *local),
            Instr::LocalSet(LocalSet { local }) => deny_fp_local(module, *local),
            Instr::LocalTee(LocalTee { local }) => deny_fp_local(module, *local),
            Instr::GlobalGet(GlobalGet { global }) => deny_fp_global(module, *global),
            Instr::GlobalSet(GlobalSet { global }) => deny_fp_global(module, *global),
            Instr::Const(Const { value }) => deny_fp_const(*value),
            Instr::Binop(Binop { op }) => deny_fp_binop(*op),
            Instr::Unop(Unop { op }) => deny_fp_unop(*op),
            Instr::Select(Select { ty: Some(ty) }) => deny_fp_valtype(*ty),
            Instr::Load(Load { kind, .. }) => deny_fp_load_kind(*kind),
            Instr::Store(Store { kind, .. }) => deny_fp_store_kind(*kind),
            Instr::RefNull(RefNull { ty }) => deny_fp_valtype(*ty),

            _ => Ok(()),
        }?;
    }

    Ok(())
}

#[inline(always)]
fn deny_fp_valtype(ty: walrus::ValType) -> Result<(), EnclaveError> {
    use walrus::ValType::{F32, F64};

    match ty {
        F32 | F64 => Err(EnclaveError::WasmModuleWithFP),
        _ => Ok(()),
    }
}

fn deny_fp_local(module: &Module, local: walrus::LocalId) -> Result<(), EnclaveError> {
    deny_fp_valtype(module.locals.get(local).ty())
}

fn deny_fp_global(module: &Module, global: walrus::GlobalId) -> Result<(), EnclaveError> {
    deny_fp_valtype(module.globals.get(global).ty)
}

fn deny_fp_const(value: walrus::ir::Value) -> Result<(), EnclaveError> {
    use walrus::ir::Value::{F32, F64};

    match value {
        F32(_) | F64(_) => Err(EnclaveError::WasmModuleWithFP),
        _ => Ok(()),
    }
}

fn deny_fp_binop(op: walrus::ir::BinaryOp) -> Result<(), EnclaveError> {
    use walrus::ir::BinaryOp::*;

    match op {
        F32Eq
        | F32Ne
        | F32Lt
        | F32Gt
        | F32Le
        | F32Ge
        | F64Eq
        | F64Ne
        | F64Lt
        | F64Gt
        | F64Le
        | F64Ge
        | F32Add
        | F32Sub
        | F32Mul
        | F32Div
        | F32Min
        | F32Max
        | F32Copysign
        | F64Add
        | F64Sub
        | F64Mul
        | F64Div
        | F64Min
        | F64Max
        | F64Copysign
        | F32x4ReplaceLane { .. }
        | F64x2ReplaceLane { .. }
        | F32x4Eq
        | F32x4Ne
        | F32x4Lt
        | F32x4Gt
        | F32x4Le
        | F32x4Ge
        | F64x2Eq
        | F64x2Ne
        | F64x2Lt
        | F64x2Gt
        | F64x2Le
        | F64x2Ge
        | F32x4Add
        | F32x4Sub
        | F32x4Mul
        | F32x4Div
        | F32x4Min
        | F32x4Max
        | F32x4PMin
        | F32x4PMax
        | F64x2Add
        | F64x2Sub
        | F64x2Mul
        | F64x2Div
        | F64x2Min
        | F64x2Max
        | F64x2PMin
        | F64x2PMax => Err(EnclaveError::WasmModuleWithFP),
        _ => Ok(()),
    }
}

fn deny_fp_unop(op: walrus::ir::UnaryOp) -> Result<(), EnclaveError> {
    use walrus::ir::UnaryOp::*;

    match op {
        F32Abs
        | F32Neg
        | F32Ceil
        | F32Floor
        | F32Trunc
        | F32Nearest
        | F32Sqrt
        | F64Abs
        | F64Neg
        | F64Ceil
        | F64Floor
        | F64Trunc
        | F64Nearest
        | F64Sqrt
        | I32TruncSF32
        | I32TruncUF32
        | I32TruncSF64
        | I32TruncUF64
        | I64TruncSF32
        | I64TruncUF32
        | I64TruncSF64
        | I64TruncUF64
        | F32ConvertSI32
        | F32ConvertUI32
        | F32ConvertSI64
        | F32ConvertUI64
        | F32DemoteF64
        | F64ConvertSI32
        | F64ConvertUI32
        | F64ConvertSI64
        | F64ConvertUI64
        | F64PromoteF32
        | I32ReinterpretF32
        | I64ReinterpretF64
        | F32ReinterpretI32
        | F64ReinterpretI64
        | F32x4Splat
        | F32x4ExtractLane { .. }
        | F64x2Splat
        | F64x2ExtractLane { .. }
        | F32x4Abs
        | F32x4Neg
        | F32x4Sqrt
        | F32x4Ceil
        | F32x4Floor
        | F32x4Trunc
        | F32x4Nearest
        | F64x2Abs
        | F64x2Neg
        | F64x2Sqrt
        | F64x2Ceil
        | F64x2Floor
        | F64x2Trunc
        | F64x2Nearest
        | I32x4TruncSatF64x2SZero
        | I32x4TruncSatF64x2UZero
        | F64x2ConvertLowI32x4S
        | F64x2ConvertLowI32x4U
        | F32x4DemoteF64x2Zero
        | F64x2PromoteLowF32x4
        | I32x4TruncSatF32x4S
        | I32x4TruncSatF32x4U
        | F32x4ConvertI32x4S
        | F32x4ConvertI32x4U
        | I32TruncSSatF32
        | I32TruncUSatF32
        | I32TruncSSatF64
        | I32TruncUSatF64
        | I64TruncSSatF32
        | I64TruncUSatF32
        | I64TruncSSatF64
        | I64TruncUSatF64 => Err(EnclaveError::WasmModuleWithFP),
        _ => Ok(()),
    }
}

fn deny_fp_load_kind(kind: walrus::ir::LoadKind) -> Result<(), EnclaveError> {
    use walrus::ir::LoadKind::{F32, F64};

    match kind {
        F32 | F64 => Err(EnclaveError::WasmModuleWithFP),
        _ => Ok(()),
    }
}

fn deny_fp_store_kind(kind: walrus::ir::StoreKind) -> Result<(), EnclaveError> {
    use walrus::ir::StoreKind::{F32, F64};

    match kind {
        F32 | F64 => Err(EnclaveError::WasmModuleWithFP),
        _ => Ok(()),
    }
}
