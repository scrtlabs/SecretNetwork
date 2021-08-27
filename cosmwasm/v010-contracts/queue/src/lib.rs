pub mod contract;

#[cfg(target_arch = "wasm32")]
cosmwasm_v010_std::create_entry_points!(contract);
