use serde::{Deserialize, Serialize};

use crate::cosmwasm::math::Uint128;

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct Coin {
    pub denom: String,
    pub amount: Uint128,
}
