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
            // fn read_db(key: *const c_void, value: *mut c_void) -> i32;
            "db_read" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32][..], Some(ValueType::I32)),
                HostFunctions::ReadDbIndex.into(),
            ),
            // fn write_db(key: *const c_void, value: *mut c_void);
            "db_write" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32, ValueType::I32][..], None),
                HostFunctions::WriteDbIndex.into(),
            ),
            // fn db_remove(key: *const c_void, value: *mut c_void) -> i32;
            "db_remove" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32][..], None),
                HostFunctions::RemoveDbIndex.into(),
            ),
            // fn canonicalize_address(human: *const c_void, canonical: *mut c_void) -> i32;
            "canonicalize_address" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32, ValueType::I32][..], Some(ValueType::I32)),
                HostFunctions::CanonicalizeAddressIndex.into(),
            ),
            // fn humanize_address(canonical: *const c_void, human: *mut c_void) -> i32;
            "humanize_address" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32, ValueType::I32][..], Some(ValueType::I32)),
                HostFunctions::HumanizeAddressIndex.into(),
            ),
            "query_chain" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32][..], Some(ValueType::I32)),
                HostFunctions::QueryChainIndex.into(),
            ),
            #[cfg(feature = "debug-print")]
            "debug_print" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32][..], None),
                HostFunctions::DebugPrintIndex.into(),
            ),
            // fn gas(amount: i32);
            "gas" => FuncInstance::alloc_host(
                Signature::new(&[ValueType::I32][..], None),
                HostFunctions::GasIndex.into(),
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
