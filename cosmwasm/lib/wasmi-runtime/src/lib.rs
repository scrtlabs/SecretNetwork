#![feature(rustc_private)]
//
// #[macro_use]
// extern crate serde_json;

mod consts;
mod contract_operations;
mod errors;
pub mod exports;
mod gas;
pub mod imports;
pub mod logger;
mod node_reg;
mod results;
mod runtime;
mod storage;

use ctor::*;
use log::LevelFilter;

use crate::logger::*;

static LOGGER: SimpleLogger = SimpleLogger;

#[ctor]
fn init_logger() {
    log::set_logger(&LOGGER).map(|()| log::set_max_level(LevelFilter::Trace));
}
