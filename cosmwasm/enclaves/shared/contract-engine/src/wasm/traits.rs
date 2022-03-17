use wasmi::{RuntimeValue, Trap};

/// These functions are imported to WASM code
pub trait WasmiApi {
    fn read_db_index(&mut self, state_key_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap>;

    fn remove_db_index(&mut self, state_key_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap>;

    fn write_db_index(
        &mut self,
        state_key_ptr_ptr: i32,
        value_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap>;

    fn canonicalize_address_index(
        &mut self,
        canonical_ptr_ptr: i32,
        human_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap>;

    fn humanize_address_index(
        &mut self,
        canonical_ptr_ptr: i32,
        human_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap>;

    fn query_chain_index(&mut self, query_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap>;

    fn gas_index(&mut self, gas_amount: i32) -> Result<Option<RuntimeValue>, Trap>;

    #[cfg(feature = "debug-print")]
    fn debug_print_index(&self, message: i32) -> Result<Option<RuntimeValue>, Trap>;
}
