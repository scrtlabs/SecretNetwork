use log::*;
use wasmi::{Externals, RuntimeArgs, RuntimeValue, Trap};

use crate::wasm::errors::WasmEngineError;

use super::contract::ContractInstance;
use super::traits::WasmiApi;

#[derive(PartialEq, Eq)]
pub enum HostFunctions {
    ReadDbIndex = 0,
    WriteDbIndex = 1,
    CanonicalizeAddressIndex = 2,
    HumanizeAddressIndex = 3,
    GasIndex = 4,
    Unknown,
}

impl From<usize> for HostFunctions {
    fn from(v: usize) -> Self {
        match v {
            x if x == HostFunctions::ReadDbIndex as usize => HostFunctions::ReadDbIndex,
            x if x == HostFunctions::WriteDbIndex as usize => HostFunctions::WriteDbIndex,
            x if x == HostFunctions::CanonicalizeAddressIndex as usize => {
                HostFunctions::CanonicalizeAddressIndex
            }
            x if x == HostFunctions::HumanizeAddressIndex as usize => {
                HostFunctions::HumanizeAddressIndex
            }
            x if x == HostFunctions::GasIndex as usize => HostFunctions::GasIndex,
            _ => HostFunctions::Unknown,
        }
    }
}

impl Into<usize> for HostFunctions {
    fn into(self) -> usize {
        self as usize
    }
}

/// Wasmi Trait implementation
impl Externals for ContractInstance {
    fn invoke_index(
        &mut self,
        index: usize,
        args: RuntimeArgs,
    ) -> Result<Option<RuntimeValue>, Trap> {
        match HostFunctions::from(index) {
            HostFunctions::ReadDbIndex => {
                let key: i32 = args.nth_checked(0).map_err(|err| {
                    error!(
                        "read_db() error reading arguments, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                // Get pointer to the region of the value buffer
                let value: i32 = args.nth_checked(1)?;

                self.read_db_index(key, value)
            }
            HostFunctions::WriteDbIndex => {
                let key: i32 = args.nth_checked(0).map_err(|err| {
                    error!(
                        "write_db() error reading arguments, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                // Get pointer to the region of the value
                let value: i32 = args.nth_checked(1)?;

                self.write_db_index(key, value)
            }

            HostFunctions::CanonicalizeAddressIndex => {
                let human: i32 = args.nth_checked(0).map_err(|err| {
                    error!(
                        "canonicalize_address() error reading arguments, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                let canonical: i32 = args.nth_checked(1)?;

                self.canonicalize_address_index(canonical, human)
            }
            // fn humanize_address(canonical: *const c_void, human: *mut c_void) -> i32;
            HostFunctions::HumanizeAddressIndex => {
                let canonical: i32 = args.nth_checked(0).map_err(|err| {
                    error!(
                        "humanize_address() error reading first argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                let human: i32 = args.nth_checked(1).map_err(|err| {
                    error!(
                        "humanize_address() error reading second argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.humanize_address_index(canonical, human)
            }
            HostFunctions::GasIndex => {
                let gas_amount: i32 = args.nth_checked(0).map_err(|err| {
                    error!("gas() error reading arguments, stopping wasm: {:?}", err);
                    err
                })?;
                self.gas_index(gas_amount)
            }
            HostFunctions::Unknown => {
                error!("unknown function index");
                Err(WasmEngineError::DbError.into())
            }
        }
    }
}
