use cosmwasm_std::{Addr, StdError, StdResult, Storage};
use secret_toolkit::storage::Item;

pub const KEY_CONTRACT_ADDRESS: &[u8] = b"contract_address";

pub static CONTRACT_ADDRESS: Item<Addr> = Item::new(KEY_CONTRACT_ADDRESS);
pub struct ContractAddressStore {}
impl ContractAddressStore {
    pub fn load(store: &dyn Storage) -> StdResult<Addr> {
        CONTRACT_ADDRESS
            .load(store)
            .map_err(|_err| StdError::generic_err("error loading contract address"))
    }

    pub fn save(store: &mut dyn Storage, contract_address: Addr) -> StdResult<()> {
        CONTRACT_ADDRESS.save(store, &contract_address)
    }
}
