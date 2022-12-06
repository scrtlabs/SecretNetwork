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
