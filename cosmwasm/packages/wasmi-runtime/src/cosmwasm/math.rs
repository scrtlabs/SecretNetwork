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
    pub const MAX: Decimal = Decimal(std::u128::MAX);

    /// Create a 1.0 Decimal
    pub const fn one() -> Decimal {
        Decimal(DECIMAL_FRACTIONAL)
    }

    /// Create a 0.0 Decimal
    pub const fn zero() -> Decimal {
        Decimal(0)
    }

    /// Convert x% into Decimal
    pub fn percent(x: u64) -> Decimal {
        Decimal((x as u128) * 10_000_000_000_000_000)
    }

    /// Convert permille (x/1000) into Decimal
    pub fn permille(x: u64) -> Decimal {
        Decimal((x as u128) * 1_000_000_000_000_000)
    }

    /// Returns the ratio (nominator / denominator) as a Decimal
    pub fn from_ratio<A: Into<u128>, B: Into<u128>>(nominator: A, denominator: B) -> Decimal {
        let nominator: u128 = nominator.into();
        let denominator: u128 = denominator.into();
        if denominator == 0 {
            panic!("Denominator must not be zero");
        }
        // TODO: better algorithm with less rounding potential?
        Decimal(nominator * DECIMAL_FRACTIONAL / denominator)
    }

    pub fn is_zero(&self) -> bool {
        self.0 == 0
    }
}

#[derive(Display)]
struct DecimalParseErr(&'static str);

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
                    .ok_or_else(|| DecimalParseErr("Value too big"))?;
                Ok(Decimal(whole_as_atomics))
            }
            2 => {
                let whole = parts[0]
                    .parse::<u128>()
                    .map_err(|_| DecimalParseErr("Error parsing whole"))?;
                let fractional = parts[1]
                    .parse::<u128>()
                    .map_err(|_| DecimalParseErr("Error parsing fractional"))?;
                let exp = (18usize.checked_sub(parts[1].len())).ok_or_else(|| {
                    DecimalParseErr("Cannot parse more than 18 fractional digits")
                })?;
                let fractional_factor = 10u128
                    .checked_pow(exp.try_into().unwrap())
                    .ok_or_else(|| DecimalParseErr("Cannot compute fractional factor"))?;

                let whole_as_atomics = whole
                    .checked_mul(DECIMAL_FRACTIONAL)
                    .ok_or_else(|| DecimalParseErr("Value too big"))?;
                let atomics = whole_as_atomics
                    .checked_add(fractional * fractional_factor)
                    .ok_or_else(|| DecimalParseErr("Value too big"))?;
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
