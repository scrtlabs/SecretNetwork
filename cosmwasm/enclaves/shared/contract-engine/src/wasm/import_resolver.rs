use wasmi::{
    Error as InterpreterError, FuncInstance, FuncRef, ImportsBuilder, ModuleImportResolver,
    Signature, ValueType,
};

use super::externals::HostFunctions;

pub fn create_builder(resolver: &dyn ModuleImportResolver) -> ImportsBuilder {
    ImportsBuilder::new().with_resolver("env", resolver)
}

/// EnigmaImportResolver maps function name to its function signature and also to function index in Runtime
/// When instansiating a module we give it this resolver
/// When invoking a function inside the module we can give it different runtimes (which we probably won't do)
#[derive(Debug, Clone)]
pub struct WasmiImportResolver {}

/// These functions should be available to invoke from wasm code
/// These should pass the request up to go-cosmwasm:
/// fn read_db(key: *const c_void, value: *mut c_void) -> i32;
/// fn write_db(key: *const c_void, value: *mut c_void);
/// These should be implemented here: + TODO: Check Cosmwasm implementation for these:
/// fn canonicalize_address(human: *const c_void, canonical: *mut c_void) -> i32;
/// fn humanize_address(canonical: *const c_void, human: *mut c_void) -> i32;
impl ModuleImportResolver for WasmiImportResolver {
    fn resolve_func(
        &self,
        func_name: &str,
        _signature: &Signature,
    ) -> Result<FuncRef, InterpreterError> {
        let func_ref = match func_name {
            // fn db_read(key: u32) -> u32;
            // v0.10 + v1
            "db_read" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32][..], Some(ValueType::I32)),
                HostFunctions::DbReadIndex.into(),
            ),
            // fn db_write(key: u32, value: u32);
            // v0.10 + v1
            "db_write" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32, ValueType::I32][..], None),
                HostFunctions::DbWriteIndex.into(),
            ),
            // fn db_remove(key: u32);
            // v0.10 + v1
            "db_remove" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32][..], None),
                HostFunctions::DbRemoveIndex.into(),
            ),
            // fn query_chain(request: u32) -> u32;
            // v0.10 + v1
            "query_chain" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32][..], Some(ValueType::I32)),
                HostFunctions::QueryChainIndex.into(),
            ),
            // fn canonicalize_address(source: u32, destination: u32) -> u32;
            // v0.10
            "canonicalize_address" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32, ValueType::I32][..], Some(ValueType::I32)),
                HostFunctions::CanonicalizeAddressIndex.into(),
            ),
            // fn humanize_address(source: u32, destination: u32) -> u32;
            // v0.10
            "humanize_address" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32, ValueType::I32][..], Some(ValueType::I32)),
                HostFunctions::HumanizeAddressIndex.into(),
            ),
            // fn addr_canonicalize(source: u32, destination: u32) -> u32;
            // v1
            "addr_canonicalize" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32, ValueType::I32][..], Some(ValueType::I32)),
                HostFunctions::AddrCanonicalizeIndex.into(),
            ),
            // fn addr_humanize(source: u32, destination: u32) -> u32;
            // v1
            "addr_humanize" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32, ValueType::I32][..], Some(ValueType::I32)),
                HostFunctions::AddrHumanizeIndex.into(),
            ),
            // fn addr_validate(source_ptr: u32) -> u32;
            // v1
            "addr_validate" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32][..], Some(ValueType::I32)),
                HostFunctions::AddrValidateIndex.into(),
            ),
            // fn debug(source_ptr: u32);
            // v1
            "debug" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32][..], None),
                HostFunctions::DebugPrintIndex.into(),
            ),
            "debug_print" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32][..], None),
                HostFunctions::DebugPrintIndex.into(),
            ),
            // fn gas(amount: i32);
            // internal
            "gas" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32][..], None),
                HostFunctions::GasIndex.into(),
            ),
            // fn secp256k1_verify(message_hash_ptr: u32, signature_ptr: u32, public_key_ptr: u32) -> u32;
            "secp256k1_verify" => FuncInstance::alloc_host(
                Signature::new(
                    &[ValueType::I32, ValueType::I32, ValueType::I32][..],
                    Some(ValueType::I32),
                ),
                HostFunctions::Secp256k1VerifyIndex.into(),
            ),
            // fn secp256k1_recover_pubkey(message_hash_ptr: u32, signature_ptr: u32, recovery_param: u32) -> u64;
            "secp256k1_recover_pubkey" => FuncInstance::alloc_host(
                Signature::new(
                    &[ValueType::I32, ValueType::I32, ValueType::I32][..],
                    Some(ValueType::I64),
                ),
                HostFunctions::Secp256k1RecoverPubkeyIndex.into(),
            ),
            // fn ed25519_verify(message_ptr: u32, signature_ptr: u32, public_key_ptr: u32) -> u32;
            "ed25519_verify" => FuncInstance::alloc_host(
                Signature::new(
                    &[ValueType::I32, ValueType::I32, ValueType::I32][..],
                    Some(ValueType::I32),
                ),
                HostFunctions::Ed25519VerifyIndex.into(),
            ),
            // fn ed25519_batch_verify(messages_ptr: u32, signatures_ptr: u32, public_keys_ptr: u32) -> u32;
            "ed25519_batch_verify" => FuncInstance::alloc_host(
                Signature::new(
                    &[ValueType::I32, ValueType::I32, ValueType::I32][..],
                    Some(ValueType::I32),
                ),
                HostFunctions::Ed25519BatchVerifyIndex.into(),
            ),
            // fn secp256k1_sign(message_ptr: u32, private_key_ptr: u32) -> u32;
            "secp256k1_sign" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32, ValueType::I32][..], Some(ValueType::I64)),
                HostFunctions::Secp256k1SignIndex.into(),
            ),
            // fn ed25519_sign(message_ptr: u32, private_key_ptr: u32) -> u32;
            "ed25519_sign" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32, ValueType::I32][..], Some(ValueType::I64)),
                HostFunctions::Ed25519SignIndex.into(),
            ),
            _ => {
                return Err(InterpreterError::Function(format!(
                    "WASM VM doesn't export function with name {}",
                    func_name
                )));
            }
        };
        Ok(func_ref)
    }
}
