#![feature(try_reserve)]
// similar trick to get the IDE to use sgx_tstd even when it doesn't know we're targeting SGX
#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;
// This annotation is here to trick the IDE into ignoring the extern crate, and instead pull in sgx_types from our
// Cargo.toml. By importing sgx_types using `extern crate` but without letting it resolve in Cargo.toml when compiling
// to SGX, we make the compiler pull it in from the target root, which contains the sgx_types listed in Xargo.toml.
// This in turn silences errors about using the same types from two versions of the same crate.
// (go ahead, try to remove this block and change the Cargo.toml import to a normal one)
#[cfg(target_env = "sgx")]
extern crate sgx_types;

use ctor::*;
use log::LevelFilter;

use crate::logger::*;

mod macros;

pub mod exports;
pub mod imports;
pub mod logger;
mod oom_handler;
pub mod registration;
use std::env;

mod consts;
mod cosmwasm;
mod crypto;
mod results;
mod storage;
mod utils;
mod wasm;

mod tests;

static LOGGER: SimpleLogger = SimpleLogger;

#[cfg(all(not(feature = "production"), feature = "SGX_MODE_HW"))]
#[ctor]
fn init_logger() {
    set_log_level_or_default(LevelFilter::Info, LevelFilter::Info);
}

#[cfg(all(feature = "production", feature = "SGX_MODE_HW"))]
#[ctor]
fn init_logger() {
    set_log_level_or_default(LevelFilter::Error, LevelFilter::Warn)
}

#[cfg(not(feature = "SGX_MODE_HW"))]
#[ctor]
fn init_logger() {
    set_log_level_or_default(LevelFilter::Trace, LevelFilter::Trace)
}

fn log_level_from_str(env_log_level: &str) -> Option<LevelFilter> {
    match env_log_level {
        "OFF" => Some(LevelFilter::Off),
        "ERROR" => Some(LevelFilter::Error),
        "WARN" => Some(LevelFilter::Warn),
        "INFO" => Some(LevelFilter::Info),
        "DEBUG" => Some(LevelFilter::Debug),
        "TRACE" => Some(LevelFilter::Trace),
        _ => None,
    }
}

fn set_log_level_or_default(default: LevelFilter, max_level: LevelFilter) {
    let mut log_level = default;

    if let Some(env_log_level) =
        log_level_from_str(&env::var(consts::LOG_LEVEL_ENV_VAR).unwrap_or_default())
    {
        // We want to make sure log level is not higher than WARN in production to prevent accidental secret leakage
        if env_log_level <= max_level {
            log_level = env_log_level;
        }
    }

    log::set_logger(&LOGGER)
        .map(|()| log::set_max_level(log_level))
        .unwrap();
}
