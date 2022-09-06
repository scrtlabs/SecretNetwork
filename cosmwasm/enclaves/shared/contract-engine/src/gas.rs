pub use pwasm_utils::{inject_gas_counter, rules};

pub const OCALL_BASE_GAS: u64 = 200_000;

/// Wasm cost table
pub struct WasmCosts {
    /// Default opcode cost
    pub regular: u32,
    /// Div operations multiplier.
    pub div: u32,
    /// Div operations multiplier.
    pub mul: u32,
    /// Memory (load/store) operations multiplier.
    pub mem: u32,
    /// General static query of U256 value from env-info
    pub static_u256: u32,
    /// General static query of Address value from env-info
    pub static_address: u32,
    /// Memory stipend. Amount of free memory (in 64kb pages) each contract can use for stack.
    pub initial_mem: u32,
    /// Grow memory cost, per page (64kb)
    pub grow_mem: u32,
    /// Memory copy cost, per byte
    pub memcpy: u32,
    /// Max stack height (native WebAssembly stack limiter)
    pub max_stack_height: u32,
    /// Cost of wasm opcode is calculated as TABLE_ENTRY_COST * `opcodes_mul` / `opcodes_div`
    pub opcodes_mul: u32,
    /// Cost of wasm opcode is calculated as TABLE_ENTRY_COST * `opcodes_mul` / `opcodes_div`
    pub opcodes_div: u32,
    /// Cost invoking humanize_address from WASM
    pub external_humanize_address: u32,
    /// Cost invoking canonicalize_address from WASM
    pub external_canonicalize_address: u32,
    /// Cost invoking addr_validate from WASM
    pub external_addr_validate: u32,
    /// Cost invoking secp256k1_verify from WASM
    pub external_secp256k1_verify: u32,
    /// Cost invoking secp256k1_recover_pubkey from WASM
    pub external_secp256k1_recover_pubkey: u32,
    /// Cost invoking ed25519_verify from WASM
    pub external_ed25519_verify: u32,
    /// Cost invoking ed25519_batch_verify from WASM
    pub external_ed25519_batch_verify_base: u32,
    /// Cost invoking ed25519_batch_verify from WASM
    pub external_ed25519_batch_verify_each: u32,
    /// Cost invoking secp256k1_sign from WASM
    pub external_secp256k1_sign: u32,
    /// Cost invoking ed25519_sign from WASM
    pub external_ed25519_sign: u32,
}

impl Default for WasmCosts {
    fn default() -> Self {
        WasmCosts {
            regular: 1,
            div: 16,
            mul: 4,
            mem: 2,
            static_u256: 64,
            static_address: 40,
            initial_mem: 8192,
            grow_mem: 8192,
            memcpy: 1,
            max_stack_height: 64 * 1024, // Assaf: I don't think that this goes anywhere
            opcodes_mul: 3,
            opcodes_div: 8,
            external_humanize_address: 8192,
            external_canonicalize_address: 8192,
            external_addr_validate: 8192,
            external_secp256k1_verify: 98304,
            external_secp256k1_recover_pubkey: 98304,
            external_ed25519_verify: 73728,
            external_ed25519_batch_verify_base: 5000,
            external_ed25519_batch_verify_each: 70000,
            external_secp256k1_sign: 100000,
            external_ed25519_sign: 75000,
        }
    }
}

pub fn gas_rules(wasm_costs: &WasmCosts) -> rules::Set {
    rules::Set::new(wasm_costs.regular, {
        let mut vals = ::std::collections::BTreeMap::new();
        vals.insert(
            rules::InstructionType::Load,
            rules::Metering::Fixed(wasm_costs.mem as u32),
        );
        vals.insert(
            rules::InstructionType::Store,
            rules::Metering::Fixed(wasm_costs.mem as u32),
        );
        vals.insert(
            rules::InstructionType::Div,
            rules::Metering::Fixed(wasm_costs.div as u32),
        );
        vals.insert(
            rules::InstructionType::Mul,
            rules::Metering::Fixed(wasm_costs.mul as u32),
        );
        vals.insert(
            rules::InstructionType::CurrentMemory,
            rules::Metering::Fixed(wasm_costs.initial_mem as u32),
        );
        vals
    })
    .with_grow_cost(wasm_costs.grow_mem)
}

#[derive(Debug, Clone)]
pub struct RuntimeWasmCosts {
    pub write_value: u64,
    pub write_additional_byte: u64,
    pub deploy_byte: u64,
    pub execution: u64,
}

impl Default for RuntimeWasmCosts {
    fn default() -> Self {
        RuntimeWasmCosts {
            write_value: 10,
            write_additional_byte: 1,
            deploy_byte: 1,
            execution: 10_000,
        }
    }
}

#[derive(Debug, Clone)]
pub struct RuntimeGas {
    pub counter: u64,
    pub limit: u64,
    pub refund: u64,
    pub costs: RuntimeWasmCosts,
}
