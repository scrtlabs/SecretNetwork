use serde::{de, ser, Deserialize, Deserializer, Serialize};
use std::convert::{TryFrom, TryInto};
use std::fmt::{self};
use std::iter::Sum;
use std::ops;

use crate::errors::{DivideByZeroError, OverflowError, OverflowOperation, StdError};

/// A thin wrapper around u64 that is using strings for JSON encoding/decoding,
/// such that the full u64 range can be used for clients that convert JSON numbers to floats,
/// like JavaScript and jq.
///
/// # Examples
///
/// Use `from` to create instances of this and `u64` to get the value out:
///
/// ```
/// # use cosmwasm_std::Uint64;
/// let a = Uint64::from(42u64);
/// assert_eq!(a.u64(), 42);
///
/// let b = Uint64::from(70u32);
/// assert_eq!(b.u64(), 70);
/// ```
#[derive(Copy, Clone, Default, Debug, PartialEq, Eq, PartialOrd, Ord)]
pub struct Uint64(u64);

impl Uint64 {
    /// Creates a Uint64(value).
    ///
    /// This method is less flexible than `from` but can be called in a const context.
    pub const fn new(value: u64) -> Self {
        Uint64(value)
    }

    /// Creates a Uint64(0)
    pub const fn zero() -> Self {
        Uint64(0)
    }

    /// Returns a copy of the internal data
    pub const fn u64(&self) -> u64 {
        self.0
    }

    pub fn is_zero(&self) -> bool {
        self.0 == 0
    }

    pub fn checked_add(self, other: Self) -> Result<Self, OverflowError> {
        self.0
            .checked_add(other.0)
            .map(Self)
            .ok_or_else(|| OverflowError::new(OverflowOperation::Add, self, other))
    }

    pub fn checked_sub(self, other: Self) -> Result<Self, OverflowError> {
        self.0
            .checked_sub(other.0)
            .map(Self)
            .ok_or_else(|| OverflowError::new(OverflowOperation::Sub, self, other))
    }

    pub fn checked_mul(self, other: Self) -> Result<Self, OverflowError> {
        self.0
            .checked_mul(other.0)
            .map(Self)
            .ok_or_else(|| OverflowError::new(OverflowOperation::Mul, self, other))
    }

    pub fn checked_div(self, other: Self) -> Result<Self, DivideByZeroError> {
        self.0
            .checked_div(other.0)
            .map(Self)
            .ok_or_else(|| DivideByZeroError::new(self))
    }

    pub fn checked_div_euclid(self, other: Self) -> Result<Self, DivideByZeroError> {
        self.0
            .checked_div_euclid(other.0)
            .map(Self)
            .ok_or_else(|| DivideByZeroError::new(self))
    }

    pub fn checked_rem(self, other: Self) -> Result<Self, DivideByZeroError> {
        self.0
            .checked_rem(other.0)
            .map(Self)
            .ok_or_else(|| DivideByZeroError::new(self))
    }

    pub fn wrapping_add(self, other: Self) -> Self {
        Self(self.0.wrapping_add(other.0))
    }

    pub fn wrapping_sub(self, other: Self) -> Self {
        Self(self.0.wrapping_sub(other.0))
    }

    pub fn wrapping_mul(self, other: Self) -> Self {
        Self(self.0.wrapping_mul(other.0))
    }

    pub fn wrapping_pow(self, other: u32) -> Self {
        Self(self.0.wrapping_pow(other))
    }

    pub fn saturating_add(self, other: Self) -> Self {
        Self(self.0.saturating_add(other.0))
    }

    pub fn saturating_sub(self, other: Self) -> Self {
        Self(self.0.saturating_sub(other.0))
    }

    pub fn saturating_mul(self, other: Self) -> Self {
        Self(self.0.saturating_mul(other.0))
    }

    pub fn saturating_pow(self, other: u32) -> Self {
        Self(self.0.saturating_pow(other))
    }
}

// `From<u{128,64,32,16,8}>` is implemented manually instead of
// using `impl<T: Into<u64>> From<T> for Uint64` because
// of the conflict with `TryFrom<&str>` as described here
// https://stackoverflow.com/questions/63136970/how-do-i-work-around-the-upstream-crates-may-add-a-new-impl-of-trait-error

impl From<u64> for Uint64 {
    fn from(val: u64) -> Self {
        Uint64(val)
    }
}

impl From<u32> for Uint64 {
    fn from(val: u32) -> Self {
        Uint64(val.into())
    }
}

impl From<u16> for Uint64 {
    fn from(val: u16) -> Self {
        Uint64(val.into())
    }
}

impl From<u8> for Uint64 {
    fn from(val: u8) -> Self {
        Uint64(val.into())
    }
}

impl TryFrom<&str> for Uint64 {
    type Error = StdError;

    fn try_from(val: &str) -> Result<Self, Self::Error> {
        match val.parse::<u64>() {
            Ok(u) => Ok(Uint64(u)),
            Err(e) => Err(StdError::generic_err(format!("Parsing u64: {}", e))),
        }
    }
}

impl From<Uint64> for String {
    fn from(original: Uint64) -> Self {
        original.to_string()
    }
}

impl From<Uint64> for u64 {
    fn from(original: Uint64) -> Self {
        original.0
    }
}

impl fmt::Display for Uint64 {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl ops::Add<Uint64> for Uint64 {
    type Output = Self;

    fn add(self, rhs: Self) -> Self {
        Uint64(self.u64().checked_add(rhs.u64()).unwrap())
    }
}

impl<'a> ops::Add<&'a Uint64> for Uint64 {
    type Output = Self;

    fn add(self, rhs: &'a Uint64) -> Self {
        Uint64(self.u64().checked_add(rhs.u64()).unwrap())
    }
}

impl ops::AddAssign<Uint64> for Uint64 {
    fn add_assign(&mut self, rhs: Uint64) {
        self.0 = self.0.checked_add(rhs.u64()).unwrap();
    }
}

impl<'a> ops::AddAssign<&'a Uint64> for Uint64 {
    fn add_assign(&mut self, rhs: &'a Uint64) {
        self.0 = self.0.checked_add(rhs.u64()).unwrap();
    }
}

impl Uint64 {
    /// Returns `self * numerator / denominator`
    pub fn multiply_ratio<A: Into<u64>, B: Into<u64>>(
        &self,
        numerator: A,
        denominator: B,
    ) -> Uint64 {
        let numerator = numerator.into();
        let denominator = denominator.into();
        if denominator == 0 {
            panic!("Denominator must not be zero");
        }

        let val: u64 = (self.full_mul(numerator) / denominator as u128)
            .try_into()
            .expect("multiplication overflow");
        Uint64::from(val)
    }

    /// Multiplies two u64 values without overflow.
    fn full_mul(self, rhs: impl Into<u64>) -> u128 {
        self.u64() as u128 * rhs.into() as u128
    }
}

impl Serialize for Uint64 {
    /// Serializes as an integer string using base 10
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: ser::Serializer,
    {
        serializer.serialize_str(&self.to_string())
    }
}

impl<'de> Deserialize<'de> for Uint64 {
    /// Deserialized from an integer string using base 10
    fn deserialize<D>(deserializer: D) -> Result<Uint64, D::Error>
    where
        D: Deserializer<'de>,
    {
        deserializer.deserialize_str(Uint64Visitor)
    }
}

struct Uint64Visitor;

impl<'de> de::Visitor<'de> for Uint64Visitor {
    type Value = Uint64;

    fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
        formatter.write_str("string-encoded integer")
    }

    fn visit_str<E>(self, v: &str) -> Result<Self::Value, E>
    where
        E: de::Error,
    {
        match v.parse::<u64>() {
            Ok(u) => Ok(Uint64(u)),
            Err(e) => Err(E::custom(format!("invalid Uint64 '{}' - {}", v, e))),
        }
    }
}

impl Sum<Uint64> for Uint64 {
    fn sum<I: Iterator<Item = Uint64>>(iter: I) -> Self {
        iter.fold(Uint64::zero(), ops::Add::add)
    }
}

impl<'a> Sum<&'a Uint64> for Uint64 {
    fn sum<I: Iterator<Item = &'a Uint64>>(iter: I) -> Self {
        iter.fold(Uint64::zero(), ops::Add::add)
    }
}
