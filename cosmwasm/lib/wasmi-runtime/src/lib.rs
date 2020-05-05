#![feature(rustc_private)]
//
// #[macro_use]
// extern crate serde_json;

mod contract_operations;
mod errors;
mod consts;
pub mod exports;
mod gas;
pub mod imports;
pub mod logger;
mod node_reg;
mod results;
mod runtime;
mod document_storage_t;
use ctor::*;
use log::{LevelFilter, SetLoggerError};

use crate::logger::*;

static LOGGER: SimpleLogger = SimpleLogger;

#[ctor]
fn init_logger() {
    log::set_logger(&LOGGER).map(|()| log::set_max_level(LevelFilter::Trace));
}
