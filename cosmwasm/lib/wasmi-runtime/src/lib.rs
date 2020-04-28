#![feature(rustc_private)]

mod contract_operations;
mod errors;
pub mod exports;
mod gas;
pub mod imports;
pub mod logger;
mod node_reg;
mod results;
mod runtime;

use ctor::*;
use log::{LevelFilter, SetLoggerError};

use crate::logger::*;

static LOGGER: SimpleLogger = SimpleLogger;

#[ctor]
fn init_logger() {
    log::set_logger(&LOGGER).map(|()| log::set_max_level(LevelFilter::Trace));
}
