use serde::{Deserialize, Serialize};
use std::fmt;

use cw_types_v010::coins::Coin as V010Coin;

use super::math::Uint128;

#[derive(Serialize, Deserialize, Clone, Default, Debug, PartialEq)]
pub struct Coin {
    pub denom: String,
    pub amount: Uint128,
}

impl Coin {
    pub fn new(amount: u128, denom: impl Into<String>) -> Self {
        Coin {
            amount: Uint128::new(amount),
            denom: denom.into(),
        }
    }
}

impl fmt::Display for Coin {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        // We use the formatting without a space between amount and denom,
        // which is common in the Cosmos SDK ecosystem:
        // https://github.com/cosmos/cosmos-sdk/blob/v0.42.4/types/coin.go#L643-L645
        // For communication to end users, Coin needs to transformed anways (e.g. convert integer uatom to decimal ATOM).
        write!(f, "{}{}", self.amount, self.denom)
    }
}

impl From<V010Coin> for Coin {
    fn from(other: V010Coin) -> Self {
        Coin {
            amount: other.amount.into(),
            denom: other.denom,
        }
    }
}
