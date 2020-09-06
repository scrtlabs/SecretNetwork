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
mod recursion_depth;
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

#[cfg(all(feature = "production", feature = "SGX_MODE_HW"))]
#[ctor]
fn init_logger() {
    log::set_logger(&LOGGER).unwrap(); // It's ok to panic at this stage. This shouldn't happen though
    set_log_level_or_default(LevelFilter::Error, LevelFilter::Warn);
}

#[cfg(all(not(feature = "production"), not(feature = "test")))]
#[ctor]
fn init_logger() {
    log::set_logger(&LOGGER).unwrap(); // It's ok to panic at this stage. This shouldn't happen though
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
        log_level_from_str(&env::var(consts::LOG_LEVEL_ENV_VAR).unwrap_or_default())
    {
        // We want to make sure log level is not higher than WARN in production to prevent accidental secret leakage
        if env_log_level <= max_level {
            log_level = env_log_level;
        }
    }

    log::set_max_level(log_level);
}

#[cfg(feature = "test")]
pub mod logging_tests {
    use crate::{count_failures, set_log_level_or_default};
    use ctor::*;
    use lazy_static::lazy_static;
    use log::*;
    use log::{Metadata, Record};
    use std::sync::SgxMutex;
    use std::{env, panic};

    lazy_static! {
        static ref LOG_BUF: SgxMutex<Vec<String>> = SgxMutex::new(Vec::new());
    }
    pub struct TestLogger;
    impl log::Log for TestLogger {
        fn enabled(&self, _metadata: &Metadata) -> bool {
            true
        }
        fn log(&self, record: &Record) {
            LOG_BUF.lock().unwrap().push(format!(
                "{}  [{}] {}",
                record.level(),
                record.target(),
                record.args()
            ));
        }
        fn flush(&self) {}
    }

    #[ctor]
    fn init_logger_test() {
        log::set_logger(&TestLogger).unwrap();
    }

    pub fn run_tests() {
        println!();
        let mut failures = 0;

        count_failures!(failures, {
            test_log_level();
            test_log_default_greater_than_max();
        });

        if failures != 0 {
            panic!("{}: {} tests failed", file!(), failures);
        }
    }

    fn test_log_level() {
        env::set_var("LOG_LEVEL", "WARN");
        set_log_level_or_default(LevelFilter::Error, LevelFilter::Info);
        assert_eq!(log::max_level(), LevelFilter::Warn);
        info!("Should not process");
        assert!(LOG_BUF.lock().unwrap().is_empty());

        env::set_var("LOG_LEVEL", "TRACE");
        set_log_level_or_default(LevelFilter::Error, LevelFilter::Info);
        assert_eq!(log::max_level(), LevelFilter::Error);
        debug!("Should not process");
        assert!(LOG_BUF.lock().unwrap().is_empty());

        env::set_var("LOG_LEVEL", "WARN");
        set_log_level_or_default(LevelFilter::Warn, LevelFilter::Warn);
        assert_eq!(log::max_level(), LevelFilter::Warn);
        trace!("Should not process");
        assert!(LOG_BUF.lock().unwrap().is_empty());

        warn!("This should process");
        assert_eq!(LOG_BUF.lock().unwrap().len(), 1);
    }

    fn test_log_default_greater_than_max() {
        let result = panic::catch_unwind(|| {
            set_log_level_or_default(LevelFilter::Trace, LevelFilter::Error);
        });
        assert!(result.is_err());
    }
}
