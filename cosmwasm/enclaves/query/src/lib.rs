// Trick to get the IDE to use sgx_tstd even when it doesn't know we're targeting SGX
#[cfg(not(target_env = "sgx"))]
extern crate sgx_tstd as std;

extern crate sgx_types;

use std::env;
use std::sync::SgxMutex;
use std::untrusted::fs::File;

#[allow(unused_imports)]
use ctor::*;
use lazy_static::lazy_static;
use log::LevelFilter;

// Force linking to all the ecalls/ocalls in this package
pub use enclave_contract_engine;

use enclave_utils::logger::{FileLogger, LOG_LEVEL_ENV_VAR};

lazy_static! {
    static ref LOG_FILE: SgxMutex<File> = SgxMutex::new(
        File::with_options()
            .create(true)
            .truncate(true)
            .open("/opt/secret/query-enclave.log")
            .expect("failed to open log file")
    );
    static ref LOGGER: FileLogger = FileLogger::new(&LOG_FILE);
}

#[cfg(all(feature = "production", feature = "SGX_MODE_HW"))]
#[ctor]
fn init_logger() {
    log::set_logger(&*LOGGER).unwrap(); // It's ok to panic at this stage. This shouldn't happen though
    set_log_level_or_default(LevelFilter::Error, LevelFilter::Warn);
}

#[cfg(all(not(feature = "production"), not(feature = "test")))]
#[ctor]
fn init_logger() {
    log::set_logger(&*LOGGER).unwrap(); // It's ok to panic at this stage. This shouldn't happen though
    set_log_level_or_default(LevelFilter::Trace, LevelFilter::Trace);
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
    if default > max_level {
        panic!(
            "Logging configuration is broken, stopping to prevent secret leaking. default: {:?}, max level: {:?}",
            default, max_level
        );
    }

    let mut log_level = default;

    if let Some(env_log_level) =
        log_level_from_str(&env::var(LOG_LEVEL_ENV_VAR).unwrap_or_default())
    {
        // We want to make sure log level is not higher than WARN in production to prevent accidental secret leakage
        if env_log_level <= max_level {
            log_level = env_log_level;
        }
    }

    log::set_max_level(log_level);
}
