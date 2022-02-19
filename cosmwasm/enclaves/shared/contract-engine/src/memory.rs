use parity_wasm::elements::{MemoryType, Module};

use log::*;

use enclave_ffi_types::EnclaveError;

pub fn validate_memory(p_modlue: &mut Module) -> Result<(), EnclaveError> {
    let memory_section = p_modlue
        .memory_section_mut()
        .ok_or(EnclaveError::CannotInitializeWasmMemory).map_err(|err| {
            error!(
                "Error accessing memory section of WASM while trying to validate memory demands: {:?}",
                err
            );
            err
        })?;

    if memory_section.entries().len() != 1 {
        error!(
            "WASM demands too many memory sections. Must be 1, demands {}",
            memory_section.entries().len()
        );
        return Err(EnclaveError::CannotInitializeWasmMemory);
    }

    let memory_entry = memory_section
        .entries()
        .first()
        .ok_or(EnclaveError::CannotInitializeWasmMemory)
        .map_err(|err| {
            error!(
                "Error accessing memory entry of WASM while trying to validate memory demands: {:?}",
                err
            );
            err
        })?;

    let requested_initial_pages: u32 = memory_entry.limits().initial();
    let maximum_allowed_pages: u32 = 192; // 12 MiB

    if requested_initial_pages > maximum_allowed_pages {
        error!(
            "WASM Requested to initialize with {} pages, maximum allowed is {}",
            requested_initial_pages, maximum_allowed_pages
        );
        return Err(EnclaveError::CannotInitializeWasmMemory);
    }

    *memory_section.entries_mut() = vec![MemoryType::new(
        requested_initial_pages,
        Some(maximum_allowed_pages),
    )];

    Ok(())
}
