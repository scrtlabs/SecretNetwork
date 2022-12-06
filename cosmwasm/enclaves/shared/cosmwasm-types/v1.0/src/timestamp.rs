use serde::{Deserialize, Serialize};
use std::fmt;

use super::math::Uint64;

/// A point in time in nanosecond precision.
///
/// This type can represent times from 1970-01-01T00:00:00Z to 2554-07-21T23:34:33Z.
///
/// ## Examples
///
/// ```
/// # use cosmwasm_std::Timestamp;
/// let ts = Timestamp::from_nanos(1_000_000_202);
/// assert_eq!(ts.nanos(), 1_000_000_202);
/// assert_eq!(ts.seconds(), 1);
/// assert_eq!(ts.subsec_nanos(), 202);
///
/// let ts = ts.plus_seconds(2);
/// assert_eq!(ts.nanos(), 3_000_000_202);
/// assert_eq!(ts.seconds(), 3);
/// assert_eq!(ts.subsec_nanos(), 202);
/// ```
#[derive(Serialize, Deserialize, Copy, Clone, Default, Debug, PartialEq, Eq, PartialOrd, Ord)]
pub struct Timestamp(Uint64);

impl Timestamp {
    /// Creates a timestamp from nanoseconds since epoch
    pub const fn from_nanos(nanos_since_epoch: u64) -> Self {
        Timestamp(Uint64::new(nanos_since_epoch))
    }

    /// Returns seconds since epoch (truncate nanoseconds)
    #[inline]
    pub fn seconds(&self) -> u64 {
        self.0.u64() / 1_000_000_000
    }

    /// Returns nanoseconds since the last whole second (the remainder truncated
    /// by `seconds()`)
    #[inline]
    pub fn subsec_nanos(&self) -> u64 {
        self.0.u64() % 1_000_000_000
    }
}

impl fmt::Display for Timestamp {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        let whole = self.seconds();
        let fractional = self.subsec_nanos();
        write!(f, "{}.{:09}", whole, fractional)
    }
}
