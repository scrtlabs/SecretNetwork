use log::*;

use wasmi::{Externals, RuntimeArgs, RuntimeValue, Trap};

use crate::errors::WasmEngineError;

use super::contract::ContractInstance;
use super::traits::WasmiApi;

#[derive(PartialEq, Eq)]
pub enum HostFunctions {
    DbReadIndex = 0,
    DbWriteIndex = 1,
    DbRemoveIndex = 2,
    CanonicalizeAddressIndex = 3,
    HumanizeAddressIndex = 4,
    GasIndex = 5,
    QueryChainIndex = 6,
    AddrValidateIndex = 7,
    AddrCanonicalizeIndex = 8,
    AddrHumanizeIndex = 9,
    Secp256k1VerifyIndex = 10,
    Secp256k1RecoverPubkeyIndex = 11,
    Ed25519VerifyIndex = 12,
    Ed25519BatchVerifyIndex = 13,
    Secp256k1SignIndex = 14,
    Ed25519SignIndex = 15,
    DebugIndex = 16,
    DebugPrintIndex = 254,
    Unknown,
}

impl From<usize> for HostFunctions {
    fn from(v: usize) -> Self {
        match v {
            x if x == HostFunctions::DbReadIndex as usize => HostFunctions::DbReadIndex,
            x if x == HostFunctions::DbWriteIndex as usize => HostFunctions::DbWriteIndex,
            x if x == HostFunctions::DbRemoveIndex as usize => HostFunctions::DbRemoveIndex,
            x if x == HostFunctions::CanonicalizeAddressIndex as usize => {
                HostFunctions::CanonicalizeAddressIndex
            }
            x if x == HostFunctions::HumanizeAddressIndex as usize => {
                HostFunctions::HumanizeAddressIndex
            }
            x if x == HostFunctions::GasIndex as usize => HostFunctions::GasIndex,
            x if x == HostFunctions::QueryChainIndex as usize => HostFunctions::QueryChainIndex,
            x if x == HostFunctions::AddrValidateIndex as usize => HostFunctions::AddrValidateIndex,
            x if x == HostFunctions::AddrCanonicalizeIndex as usize => {
                HostFunctions::AddrCanonicalizeIndex
            }
            x if x == HostFunctions::AddrHumanizeIndex as usize => HostFunctions::AddrHumanizeIndex,
            x if x == HostFunctions::DebugIndex as usize => HostFunctions::DebugIndex,
            x if x == HostFunctions::Secp256k1VerifyIndex as usize => {
                HostFunctions::Secp256k1VerifyIndex
            }
            x if x == HostFunctions::Secp256k1RecoverPubkeyIndex as usize => {
                HostFunctions::Secp256k1RecoverPubkeyIndex
            }
            x if x == HostFunctions::Ed25519VerifyIndex as usize => {
                HostFunctions::Ed25519VerifyIndex
            }
            x if x == HostFunctions::Ed25519BatchVerifyIndex as usize => {
                HostFunctions::Ed25519BatchVerifyIndex
            }
            x if x == HostFunctions::Secp256k1SignIndex as usize => {
                HostFunctions::Secp256k1SignIndex
            }
            x if x == HostFunctions::Ed25519SignIndex as usize => HostFunctions::Ed25519SignIndex,
            x if x == HostFunctions::DebugPrintIndex as usize => HostFunctions::DebugPrintIndex,
            _ => HostFunctions::Unknown,
        }
    }
}

#[allow(clippy::from_over_into)]
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
            HostFunctions::DbReadIndex => {
                let key: i32 = args.nth_checked(0).map_err(|err| {
                    warn!("read_db() error reading argument, stopping wasm: {:?}", err);
                    err
                })?;
                self.read_db(key)
            }
            HostFunctions::DbRemoveIndex => {
                let key: i32 = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "remove_db() error reading argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                self.remove_db(key)
            }
            HostFunctions::DbWriteIndex => {
                let key: i32 = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "write_db() error reading 1st arguments, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let value: i32 = args.nth_checked(1).map_err(|err| {
                    warn!(
                        "write_db() error reading 2nd arguments, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.write_db(key, value)
            }
            HostFunctions::CanonicalizeAddressIndex => {
                let human: i32 = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "canonicalize_address() error reading 1st argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let canonical: i32 = args.nth_checked(1).map_err(|err| {
                    warn!(
                        "canonicalize_address() error reading 2nd argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.canonicalize_address(human, canonical)
            }
            // fn humanize_address(canonical: *const c_void, human: *mut c_void) -> i32;
            HostFunctions::HumanizeAddressIndex => {
                let canonical: i32 = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "humanize_address() error reading 1st argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let human: i32 = args.nth_checked(1).map_err(|err| {
                    warn!(
                        "humanize_address() error reading 2nd argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.humanize_address(canonical, human)
            }
            HostFunctions::QueryChainIndex => {
                let query: i32 = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "query_chain() error reading argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.query_chain(query)
            }
            HostFunctions::GasIndex => {
                let gas_amount: i32 = args.nth_checked(0).map_err(|err| {
                    warn!("gas() error reading argument, stopping wasm: {:?}", err);
                    err
                })?;

                self.gas(gas_amount)
            }
            HostFunctions::AddrValidateIndex => {
                let human: i32 = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "addr_validate() error reading 1st argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.addr_validate(human)
            }
            HostFunctions::AddrCanonicalizeIndex => {
                let human: i32 = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "addr_canonicalize() error reading 1st argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let canonical: i32 = args.nth_checked(1).map_err(|err| {
                    warn!(
                        "addr_canonicalize() error reading 2nd argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.addr_canonicalize(human, canonical)
            }
            HostFunctions::AddrHumanizeIndex => {
                let canonical: i32 = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "addr_humanize() error reading 1st argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let human: i32 = args.nth_checked(1).map_err(|err| {
                    warn!(
                        "addr_humanize() error reading 2nd argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.addr_humanize(canonical, human)
            }
            HostFunctions::Secp256k1VerifyIndex => {
                let message_hash = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "secp256k1_verify() error reading 1st argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let signature = args.nth_checked(1).map_err(|err| {
                    warn!(
                        "secp256k1_verify() error reading 2nd argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let public_key = args.nth_checked(2).map_err(|err| {
                    warn!(
                        "secp256k1_verify() error reading 3rd argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.secp256k1_verify(message_hash, signature, public_key)
            }
            HostFunctions::Secp256k1RecoverPubkeyIndex => {
                let message_hash = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "secp256k1_recover_pubkey() error reading 1st argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let signature =  args.nth_checked(1).map_err(|err| {
                    warn!(
                        "secp256k1_recover_pubkey() error reading 2nd argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let recovery_param = args.nth_checked(2).map_err(|err| {
                    warn!(
                        "secp256k1_recover_pubkey() error reading 3rd argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.secp256k1_recover_pubkey(message_hash, signature, recovery_param)
            }
            HostFunctions::Ed25519VerifyIndex => {
                let message = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "ed25519_verify() error reading 1st argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let signature = args.nth_checked(1).map_err(|err| {
                    warn!(
                        "ed25519_verify() error reading 2nd argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let public_key = args.nth_checked(2).map_err(|err| {
                    warn!(
                        "ed25519_verify() error reading 3rd argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.ed25519_verify(message, signature, public_key)
            }
            HostFunctions::Ed25519BatchVerifyIndex => {
                let messages = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "ed25519_verify() error reading 1st argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let signatures = args.nth_checked(1).map_err(|err| {
                    warn!(
                        "ed25519_verify() error reading 2nd argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let public_keys = args.nth_checked(2).map_err(|err| {
                    warn!(
                        "ed25519_verify() error reading 3rd argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.ed25519_batch_verify(messages, signatures, public_keys)
            }
            HostFunctions::DebugIndex => {
                #[cfg(feature = "debug-print")]
                {
                    // this comment is here to let rustfmt know that it's an idiot
                    let message: i32 = args.nth_checked(0).map_err(|err| {
                        warn!("debug() error reading argument, stopping wasm: {:?}", err);
                        err
                    })?;

                    self.debug_print_index(message)
                }
                #[cfg(not(feature = "debug-print"))]
                Ok(None)
            }
            HostFunctions::DebugPrintIndex => {
                #[cfg(feature = "debug-print")]
                {
                    // this comment is here to let rustfmt know that it's an idiot
                    let message: i32 = args.nth_checked(0).map_err(|err| {
                        warn!(
                            "debug_print() error reading argument, stopping wasm: {:?}",
                            err
                        );
                        err
                    })?;

                    self.debug_print_index(message)
                }
                #[cfg(not(feature = "debug-print"))]
                Ok(None)
            }
            HostFunctions::Unknown => {
                warn!("unknown function index");
                Err(WasmEngineError::NonExistentImportFunction.into())
            }
            HostFunctions::Secp256k1SignIndex => {
                let message = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "secp256k1_sign() error reading 1st argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let private_key = args.nth_checked(1).map_err(|err| {
                    warn!(
                        "secp256k1_sign() error reading 2nd argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.secp256k1_sign(message, private_key)
            }
            HostFunctions::Ed25519SignIndex => {
                let message = args.nth_checked(0).map_err(|err| {
                    warn!(
                        "ed25519_sign() error reading 1st argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;
                let private_key = args.nth_checked(1).map_err(|err| {
                    warn!(
                        "ed25519_sign() error reading 2nd argument, stopping wasm: {:?}",
                        err
                    );
                    err
                })?;

                self.ed25519_sign(message, private_key)
            }
        }
    }
}
