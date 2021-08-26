use wasmi::{RuntimeValue, Trap};

/// These functions are imported to WASM code
pub trait WasmiApi {
    fn db_read(&mut self, state_key_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap>;

    fn db_remove(&mut self, state_key_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap>;

    fn db_write(
        &mut self,
        state_key_ptr_ptr: i32,
        value_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap>;

    fn canonicalize_address(
        &mut self,
        canonical_ptr_ptr: i32,
        human_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap>;

    fn humanize_address(
        &mut self,
        canonical_ptr_ptr: i32,
        human_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap>;

    fn query_chain(&mut self, query_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap>;

    fn addr_validate(&mut self, source_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap>;

    fn secp256k1_verify(
        &mut self,
        message_hash_ptr_ptr: i32,
        signature_ptr_ptr: i32,
        public_key_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap>;

    fn secp256k1_recover_pubkey(
        &mut self,
        message_hash_ptr_ptr: i32,
        signature_ptr_ptr: i32,
        recovery_param_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap>;

    fn ed25519_verify(
        &mut self,
        message_ptr_ptr: i32,
        signature_ptr_ptr: i32,
        public_key_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap>;

    fn ed25519_batch_verify(
        &mut self,
        messages_ptr_ptr: i32,
        signatures_ptr_ptr: i32,
        public_keys_ptr_ptr: i32,
    ) -> Result<Option<RuntimeValue>, Trap>;

    fn debug(&mut self, message_ptr_ptr: i32) -> Result<Option<RuntimeValue>, Trap>;

    fn gas(&mut self, gas_amount: i32) -> Result<Option<RuntimeValue>, Trap>;
}
