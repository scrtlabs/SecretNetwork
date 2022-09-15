//use log::*;

use wasmi::ModuleRef;
//use wasmi::RuntimeValue;

//use enclave_ffi_types::EnclaveError;

//use cw_types_generic::CosmWasmApiVersion;

use super::contract::ContractInstance;
// use crate::errors::{wasmi_error_to_enclave_error, WasmEngineError};
// use enclave_cosmos_types::types::HandleType;

#[allow(dead_code)]
pub struct Engine {
    pub contract_instance: ContractInstance,
    pub module: ModuleRef,
}

// impl Engine {
//     pub fn new(contract_instance: ContractInstance, module: ModuleRef) -> Self {
//         Self {
//             contract_instance,
//             module,
//         }
//     }
//
//     pub fn gas_used(&self) -> u64 {
//         self.contract_instance.gas_used
//     }
//
//     pub fn write_to_memory(&mut self, buffer: &[u8]) -> Result<u32, WasmEngineError> {
//         self.contract_instance.write_to_memory(buffer)
//     }
//
//     pub fn extract_vector(&self, vec_ptr_ptr: u32) -> Result<Vec<u8>, WasmEngineError> {
//         self.contract_instance.extract_vector(vec_ptr_ptr)
//     }
//
//     pub fn init(
//         &mut self,
//         env_ptr: u32,
//         msg_info_ptr: u32,
//         msg_ptr: u32,
//     ) -> Result<u32, EnclaveError> {
//         info!("Invoking init() in wasm");
//
//         let (func_name, args) = match self.contract_instance.cosmwasm_api_version {
//             CosmWasmApiVersion::V010 => (
//                 "init",
//                 vec![
//                     RuntimeValue::I32(env_ptr as i32),
//                     RuntimeValue::I32(msg_ptr as i32),
//                 ],
//             ),
//             CosmWasmApiVersion::V1 => (
//                 "instantiate",
//                 vec![
//                     RuntimeValue::I32(env_ptr as i32),
//                     RuntimeValue::I32(msg_info_ptr as i32),
//                     RuntimeValue::I32(msg_ptr as i32),
//                 ],
//             ),
//             _ => return Err(EnclaveError::InvalidWasm),
//         };
//
//         match self
//             .module
//             .invoke_export(func_name, &args, &mut self.contract_instance)
//             .map_err(wasmi_error_to_enclave_error)?
//         {
//             Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
//             other => {
//                 warn!("init method returned value which wasn't u32: {:?}", other);
//                 Err(EnclaveError::FailedFunctionCall)
//             }
//         }
//
//         // Itzik: leaving this here as an example in case we will want to do something like this in the future
//
//         // if result.is_ok() {
//         //     write_encrypted_key(
//         //         b"key",
//         //         contract_key,
//         //         &self.contract_instance.context,
//         //         &self.contract_instance.contract_key,
//         //     )
//         //     .map_err(|_| {
//         //         error!("Failed to write contract key to database");
//         //         EnclaveError::InternalError
//         //     })?;
//         // }
//
//         //result
//     }
//
//     pub fn handle(
//         &mut self,
//         env_ptr: u32,
//         msg_info_ptr: u32,
//         msg_ptr: u32,
//         handle_type: HandleType,
//     ) -> Result<u32, EnclaveError> {
//         // Itzik: leaving this here as an example in case we will want to do something like this in the future
//
//         // let stored_address = read_encrypted_key(
//         //     b"key",
//         //     &self.contract_instance.context,
//         //     &self.contract_instance.contract_key,
//         // )
//         // .map_err(|_| {
//         //     error!("WTF wrong contract key are you crazy???");
//         //     EnclaveError::InternalError
//         // })?;
//         //
//         // match stored_address.0 {
//         //     Some(addr) => {
//         //         if addr != contract_key.to_vec() {
//         //             error!("WTF wrong contract key are you crazy???");
//         //             return Err(EnclaveError::FailedUnseal);
//         //         }
//         //         Ok(())
//         //     }
//         //     None => {
//         //         error!("WTF no contract address found you must be trippin' dawg");
//         //         Err(EnclaveError::InternalError)
//         //     }
//         // }?;
//
//         let (func_name, args) = match self.contract_instance.cosmwasm_api_version {
//             CosmWasmApiVersion::V010 => (
//                 "handle",
//                 vec![
//                     RuntimeValue::I32(env_ptr as i32),
//                     RuntimeValue::I32(msg_ptr as i32),
//                 ],
//             ),
//             CosmWasmApiVersion::V1 => match handle_type {
//                 HandleType::HANDLE_TYPE_EXECUTE => (
//                     "execute",
//                     vec![
//                         RuntimeValue::I32(env_ptr as i32),
//                         RuntimeValue::I32(msg_info_ptr as i32),
//                         RuntimeValue::I32(msg_ptr as i32),
//                     ],
//                 ),
//                 HandleType::HANDLE_TYPE_REPLY => (
//                     "reply",
//                     vec![
//                         RuntimeValue::I32(env_ptr as i32),
//                         RuntimeValue::I32(msg_ptr as i32),
//                     ],
//                 ),
//                 HandleType::HANDLE_TYPE_IBC_CHANNEL_OPEN => (
//                     "ibc_channel_open",
//                     vec![
//                         RuntimeValue::I32(env_ptr as i32),
//                         RuntimeValue::I32(msg_ptr as i32),
//                     ],
//                 ),
//                 HandleType::HANDLE_TYPE_IBC_CHANNEL_CONNECT => (
//                     "ibc_channel_connect",
//                     vec![
//                         RuntimeValue::I32(env_ptr as i32),
//                         RuntimeValue::I32(msg_ptr as i32),
//                     ],
//                 ),
//                 HandleType::HANDLE_TYPE_IBC_CHANNEL_CLOSE => (
//                     "ibc_channel_close",
//                     vec![
//                         RuntimeValue::I32(env_ptr as i32),
//                         RuntimeValue::I32(msg_ptr as i32),
//                     ],
//                 ),
//                 HandleType::HANDLE_TYPE_IBC_PACKET_RECEIVE => (
//                     "ibc_packet_receive",
//                     vec![
//                         RuntimeValue::I32(env_ptr as i32),
//                         RuntimeValue::I32(msg_ptr as i32),
//                     ],
//                 ),
//                 HandleType::HANDLE_TYPE_IBC_PACKET_ACK => (
//                     "ibc_packet_ack",
//                     vec![
//                         RuntimeValue::I32(env_ptr as i32),
//                         RuntimeValue::I32(msg_ptr as i32),
//                     ],
//                 ),
//                 HandleType::HANDLE_TYPE_IBC_PACKET_TIMEOUT => (
//                     "ibc_packet_timeout",
//                     vec![
//                         RuntimeValue::I32(env_ptr as i32),
//                         RuntimeValue::I32(msg_ptr as i32),
//                     ],
//                 ),
//             },
//             _ => return Err(EnclaveError::InvalidWasm),
//         };
//
//         info!("Invoking {}() in wasm", func_name);
//
//         match self
//             .module
//             .invoke_export(func_name, &args, &mut self.contract_instance)
//             .map_err(wasmi_error_to_enclave_error)?
//         {
//             Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
//             other => {
//                 warn!(
//                     "{} method returned value which wasn't u32: {:?}",
//                     func_name, other
//                 );
//                 Err(EnclaveError::FailedFunctionCall)
//             }
//         }
//     }
//
//     pub fn query(&mut self, env_ptr: u32, msg_ptr: u32) -> Result<u32, EnclaveError> {
//         info!("Invoking query() in wasm");
//
//         let args = match self.contract_instance.cosmwasm_api_version {
//             CosmWasmApiVersion::V010 => {
//                 vec![
//                     RuntimeValue::I32(msg_ptr as i32),
//                     /* no env in v0.10 */
//                 ]
//             }
//             CosmWasmApiVersion::V1 => vec![
//                 RuntimeValue::I32(env_ptr as i32),
//                 RuntimeValue::I32(msg_ptr as i32),
//             ],
//             _ => return Err(EnclaveError::InvalidWasm),
//         };
//
//         match self
//             .module
//             .invoke_export("query", &args, &mut self.contract_instance)
//             .map_err(wasmi_error_to_enclave_error)?
//         {
//             Some(RuntimeValue::I32(offset)) => Ok(offset as u32),
//             other => {
//                 warn!("query method returned value which wasn't u32: {:?}", other);
//                 Err(EnclaveError::FailedFunctionCall)
//             }
//         }
//     }
// }
