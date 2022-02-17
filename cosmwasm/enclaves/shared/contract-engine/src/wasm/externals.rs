use log::*;

use wasmi::{Externals, RuntimeArgs, RuntimeValue, Trap};

use crate::errors::WasmEngineError;

use super::contract::ContractInstance;
use super::traits::WasmiApi;

#[derive(PartialEq, Eq)]
pub enum HostFunctions {
    ReadDbIndex = 0,
    WriteDbIndex = 1,
    RemoveDbIndex = 2,
    CanonicalizeAddressIndex = 3,
    HumanizeAddressIndex = 4,
    GasIndex = 5,
    QueryChainIndex = 6,
    #[cfg(feature = "debug-print")]
    DebugPrintIndex = 254,
    Unknown,
}

impl From<usize> for HostFunctions {
    fn from(v: usize) -> Self {
        match v {
            x if x == HostFunctions::ReadDbIndex as usize => HostFunctions::ReadDbIndex,
            x if x == HostFunctions::WriteDbIndex as usize => HostFunctions::WriteDbIndex,
            x if x == HostFunctions::RemoveDbIndex as usize => HostFunctions::RemoveDbIndex,
            x if x == HostFunctions::CanonicalizeAddressIndex as usize => {
                HostFunctions::CanonicalizeAddressIndex
            }
            x if x == HostFunctions::HumanizeAddressIndex as usize => {
                HostFunctions::HumanizeAddressIndex
            }
            x if x == HostFunctions::GasIndex as usize => HostFunctions::GasIndex,
            x if x == HostFunctions::QueryChainIndex as usize => HostFunctions::QueryChainIndex,
            #[cfg(feature = "debug-print")]
            x if x == HostFunctions::DebugPrintIndex as usize => HostFunctions::DebugPrintIndex,
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
                    warn!(
                        "read_db() error reading arguments, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                self.read_db_index(key)
            }
            HostFunctions::RemoveDbIndex => {
                let key: i32 = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "remove_db() error reading arguments, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                self.remove_db_index(key)
            }
            HostFunctions::WriteDbIndex => {
                let key: i32 = args.nth_checked(0).map_err(|err| {
                    warn!(
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
                    warn!(
                        "canonicalize_address() error reading arguments, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                let canonical: i32 = args.nth_checked(1)?;

                self.canonicalize_address_index(human, canonical)
            }
            // fn humanize_address(canonical: *const c_void, human: *mut c_void) -> i32;
            HostFunctions::HumanizeAddressIndex => {
                let canonical: i32 = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "humanize_address() error reading first argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                let human: i32 = args.nth_checked(1).map_err(|err| {
                    warn!(
                        "humanize_address() error reading second argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.humanize_address_index(canonical, human)
            }
            HostFunctions::QueryChainIndex => {
                let query: i32 = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "query_chain() error reading argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.query_chain_index(query)
            }
            HostFunctions::GasIndex => {
                let gas_amount: i32 = args.nth_checked(0).map_err(|err| {
                    warn!("gas() error reading arguments, stopping wasm: {:?}", err);
                    err
                })?;
                self.gas_index(gas_amount)
            }
            #[cfg(feature = "debug-print")]
            HostFunctions::DebugPrintIndex => {
                let message: i32 = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "debug_print() error reading argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.debug_print_index(message)
            }
            HostFunctions::Unknown => {
                warn!("unknown function index");
                Err(WasmEngineError::NonExistentImportFunction.into())
            }
        }
    }
}
