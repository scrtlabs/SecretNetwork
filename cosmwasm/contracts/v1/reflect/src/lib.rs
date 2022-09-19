pub mod contract;
mod errors;
pub mod msg;
pub mod state;

#[cfg(not(target_arch = "wasm32"))]
pub mod testing;

#[cfg(target_arch = "wasm32")]
cosmwasm_std::create_entry_points!(contract);
