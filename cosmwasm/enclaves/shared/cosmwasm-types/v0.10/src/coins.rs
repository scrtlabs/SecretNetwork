//! must keep this file in sync with cosmwasm/packages/std/src/coins.rs

use serde::{Deserialize, Serialize};

use super::math::Uint128;

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct Coin {
    pub denom: String,
    pub amount: Uint128,
}
