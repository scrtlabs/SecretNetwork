//! must keep this file in sync with cosmwasm/packages/std/src/math.rs

use derive_more::Display;
use serde::{de, ser, Deserialize, Deserializer, Serialize};
use std::convert::{TryFrom, TryInto};
use std::fmt::{self, Write};
use std::ops;
use std::str::FromStr;

/// A fixed-point decimal value with 18 fractional digits, i.e. Decimal(1_000_000_000_000_000_000) == 1.0
///
/// The greatest possible value that can be represented is 340282366920938463463.374607431768211455 (which is (2^128 - 1) / 10^18)
#[derive(Copy, Clone, Default, Debug, PartialEq, Eq, PartialOrd, Ord)]
pub struct Decimal(u128);

const DECIMAL_FRACTIONAL: u128 = 1_000_000_000_000_000_000;

impl Decimal {
    pub fn is_zero(&self) -> bool {
        self.0 == 0
    }
}

#[derive(Display)]
pub struct DecimalParseErr(&'static str);

impl FromStr for Decimal {
    type Err = DecimalParseErr;

    /// Converts the decimal string to a Decimal
    /// Possible inputs: "1.23", "1", "000012", "1.123000000"
    /// Disallowed: "", ".23"
    ///
    /// This never performs any kind of rounding.
    /// More than 18 fractional digits, even zeros, result in an error.
    fn from_str(input: &str) -> Result<Self, Self::Err> {
        let parts: Vec<&str> = input.split('.').collect();
        match parts.len() {
            1 => {
                let whole = parts[0]
                    .parse::<u128>()
                    .map_err(|_| DecimalParseErr("Error parsing whole"))?;

                let whole_as_atomics = whole
                    .checked_mul(DECIMAL_FRACTIONAL)
                    .ok_or(DecimalParseErr("Value too big"))?;
                Ok(Decimal(whole_as_atomics))
            }
            2 => {
                let whole = parts[0]
                    .parse::<u128>()
                    .map_err(|_| DecimalParseErr("Error parsing whole"))?;
                let fractional = parts[1]
                    .parse::<u128>()
                    .map_err(|_| DecimalParseErr("Error parsing fractional"))?;
                let exp = (18usize.checked_sub(parts[1].len())).ok_or(DecimalParseErr(
                    "Cannot parse more than 18 fractional digits",
                ))?;
                let fractional_factor = 10u128
                    .checked_pow(exp.try_into().unwrap())
                    .ok_or(DecimalParseErr("Cannot compute fractional factor"))?;

                let whole_as_atomics = whole
                    .checked_mul(DECIMAL_FRACTIONAL)
                    .ok_or(DecimalParseErr("Value too big"))?;
                let atomics = whole_as_atomics
                    .checked_add(fractional * fractional_factor)
                    .ok_or(DecimalParseErr("Value too big"))?;
                Ok(Decimal(atomics))
            }
            _ => Err(DecimalParseErr("Unexpected number of dots")),
        }
    }
}

impl fmt::Display for Decimal {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        let whole = (self.0) / DECIMAL_FRACTIONAL;
        let fractional = (self.0) % DECIMAL_FRACTIONAL;

        if fractional == 0 {
            write!(f, "{}", whole)
        } else {
            let fractional_string = format!("{:018}", fractional);
            f.write_str(&whole.to_string())?;
            f.write_char('.')?;
            f.write_str(fractional_string.trim_end_matches('0'))?;
            Ok(())
        }
    }
}

impl ops::Add for Decimal {
    type Output = Self;

    fn add(self, other: Self) -> Self {
        Decimal(self.0 + other.0)
    }
}

/// Serializes as a decimal string
impl Serialize for Decimal {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: ser::Serializer,
    {
        serializer.serialize_str(&self.to_string())
    }
}

/// Deserializes as a base64 string
impl<'de> Deserialize<'de> for Decimal {
    fn deserialize<D>(deserializer: D) -> Result<Decimal, D::Error>
    where
        D: Deserializer<'de>,
    {
        deserializer.deserialize_str(DecimalVisitor)
    }
}

struct DecimalVisitor;

impl<'de> de::Visitor<'de> for DecimalVisitor {
    type Value = Decimal;

    fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
        formatter.write_str("string-encoded decimal")
    }

    fn visit_str<E>(self, v: &str) -> Result<Self::Value, E>
    where
        E: de::Error,
    {
        match Decimal::from_str(v) {
            Ok(d) => Ok(d),
            Err(e) => Err(E::custom(format!("Error parsing decimal '{}': {}", v, e))),
        }
    }
}

//*** Uint128 ***/
#[derive(Copy, Clone, Default, Debug, PartialEq, Eq, PartialOrd, Ord)]
pub struct Uint128(pub u128);

impl Uint128 {
    /// Creates a Uint128(0)
    pub const fn zero() -> Self {
        Uint128(0)
    }

    /// Returns a copy of the internal data
    pub fn u128(&self) -> u128 {
        self.0
    }

    pub fn is_zero(&self) -> bool {
        self.0 == 0
    }
}

impl From<u128> for Uint128 {
    fn from(val: u128) -> Self {
        Uint128(val)
    }
}

impl From<u64> for Uint128 {
    fn from(val: u64) -> Self {
        Uint128(val.into())
    }
}

#[derive(Display)]
pub struct Uint128ParseErr(String);

impl TryFrom<&str> for Uint128 {
    type Error = Uint128ParseErr;

    fn try_from(val: &str) -> Result<Self, Self::Error> {
        match val.parse::<u128>() {
            Ok(u) => Ok(Uint128(u)),
            Err(e) => Err(Uint128ParseErr(format!("Parsing coin: {}", e))),
        }
    }
}

impl Into<String> for Uint128 {
    fn into(self) -> String {
        self.0.to_string()
    }
}

impl Into<u128> for Uint128 {
    fn into(self) -> u128 {
        self.0
    }
}

impl fmt::Display for Uint128 {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

/// Both d*u and u*d with d: Decimal and u: Uint128 returns an Uint128. There is no
/// specific reason for this decision other than the initial use cases we have. If you
/// need a Decimal result for the same calculation, use Decimal(d*u) or Decimal(u*d).
impl ops::Mul<Decimal> for Uint128 {
    type Output = Self;

    #[allow(clippy::suspicious_arithmetic_impl)]
    fn mul(self, rhs: Decimal) -> Self::Output {
        // 0*a and b*0 is always 0
        if self.is_zero() || rhs.is_zero() {
            return Uint128::zero();
        }
        self.multiply_ratio(rhs.0, DECIMAL_FRACTIONAL)
    }
}

impl ops::Mul<Uint128> for Decimal {
    type Output = Uint128;

    fn mul(self, rhs: Uint128) -> Self::Output {
        rhs * self
    }
}

impl Uint128 {
    /// returns self * nom / denom
    pub fn multiply_ratio<A: Into<u128>, B: Into<u128>>(&self, nom: A, denom: B) -> Uint128 {
        let nominator: u128 = nom.into();
        let denominator: u128 = denom.into();
        if denominator == 0 {
            panic!("Denominator must not be zero");
        }
        // TODO: minimize rounding that takes place (using gcd algorithm)
        let val = self.u128() * nominator / denominator;
        Uint128::from(val)
    }
}

/// Serializes as a base64 string
impl Serialize for Uint128 {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: ser::Serializer,
    {
        serializer.serialize_str(&self.to_string())
    }
}

/// Deserializes as a base64 string
impl<'de> Deserialize<'de> for Uint128 {
    fn deserialize<D>(deserializer: D) -> Result<Uint128, D::Error>
    where
        D: Deserializer<'de>,
    {
        deserializer.deserialize_str(Uint128Visitor)
    }
}

struct Uint128Visitor;

impl<'de> de::Visitor<'de> for Uint128Visitor {
    type Value = Uint128;

    fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
        formatter.write_str("string-encoded integer")
    }

    fn visit_str<E>(self, v: &str) -> Result<Self::Value, E>
    where
        E: de::Error,
    {
        match v.parse::<u128>() {
            Ok(u) => Ok(Uint128(u)),
            Err(e) => Err(E::custom(format!("invalid Uint128 '{}' - {}", v, e))),
        }
    }
}
