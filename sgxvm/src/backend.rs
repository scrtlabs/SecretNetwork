use ethereum::Log;
use evm::backend::{Apply, Backend as EvmBackend, Basic};
use primitive_types::{H160, H256, U256};
use std::vec::Vec;

use crate::{
    coder,
    error::Error,
    protobuf_generated::ffi,
    querier,
    storage::FFIStorage,
    types::{ExtendedBackend, Storage, Vicinity},
};

/// Contains context of the transaction such as gas price, block hash, block timestamp, etc.
pub struct TxContext {
    pub chain_id: U256,
    pub gas_price: U256,
    pub block_number: U256,
    pub timestamp: U256,
    pub block_gas_limit: U256,
    pub block_base_fee_per_gas: U256,
    pub block_coinbase: H160,
}

impl From<ffi::TransactionContext> for TxContext {
    fn from(context: ffi::TransactionContext) -> Self {
        Self {
            chain_id: U256::from(context.chain_id),
            gas_price: U256::from_big_endian(&context.gas_price),
            block_number: U256::from(context.block_number),
            timestamp: U256::from(context.timestamp),
            block_gas_limit: U256::from(context.block_gas_limit),
            block_base_fee_per_gas: U256::from_big_endian(&context.block_base_fee_per_gas),
            block_coinbase: H160::from_slice(&context.block_coinbase),
        }
    }
}

pub struct FFIBackend<'state> {
    // We keep GoQuerier to make it accessible for `OCALL` handlers
    pub querier: *mut querier::GoQuerier,
    // Contains gas price and original sender
    pub vicinity: Vicinity,
    // Accounts state
    pub state: &'state mut FFIStorage,
    // Emitted events
    pub logs: Vec<Log>,
    // Transaction context
    pub tx_context: TxContext,
}

impl<'state> ExtendedBackend for FFIBackend<'state> {
    fn get_logs(&self) -> Vec<Log> {
        self.logs.clone()
    }

    fn apply<A, I, L>(&mut self, values: A, logs: L, _delete_empty: bool) -> Result<(), Error>
    where
        A: IntoIterator<Item = Apply<I>>,
        I: IntoIterator<Item = (H256, H256)>,
        L: IntoIterator<Item = Log>,
    {
        let mut total_supply_add = U256::zero();
        let mut total_supply_sub = U256::zero();

        for apply in values {
            match apply {
                Apply::Modify {
                    address,
                    basic,
                    code,
                    storage,
                    ..
                } => {
                    // Update account balance and nonce
                    let previous_account_data = self.state.get_account(&address);

                    if basic.balance > previous_account_data.balance {
                        total_supply_add = total_supply_add
                            .checked_add(basic.balance - previous_account_data.balance)
                            .unwrap();
                    } else {
                        total_supply_sub = total_supply_sub
                            .checked_add(previous_account_data.balance - basic.balance)
                            .unwrap();
                    }
                    self.state.insert_account(address, basic)?;

                    // Handle contract updates
                    if let Some(code) = code {
                        self.state.insert_account_code(address, code)?;
                    }

                    // Handle storage updates
                    for (index, value) in storage {
                        if value == H256::default() {
                            self.state.remove_storage_cell(&address, &index)?;
                        } else {
                            self.state.insert_storage_cell(address, index, value)?;
                        }
                    }
                }
                // Used by `SELFDESTRUCT` opcode
                Apply::Delete { address } => {
                    self.state.remove(&address)?;
                }
            }
        }

        // Used to avoid corrupting state via invariant violation
        assert_eq!(
            total_supply_add, total_supply_sub,
            "evm execution would lead to invariant violation ({} != {})",
            total_supply_add, total_supply_sub
        );

        for log in logs {
            self.logs.push(log);
        }

        Ok(())
    }
}

impl<'state> EvmBackend for FFIBackend<'state> {
    fn gas_price(&self) -> U256 {
        self.tx_context.gas_price
    }

    fn origin(&self) -> H160 {
        self.vicinity.origin
    }

    fn block_hash(&self, number: U256) -> H256 {
        let encoded_request = coder::encode_query_block_hash(number);
        match querier::make_request(self.querier, encoded_request) {
            Some(result) => {
                // Decode protobuf
                let decoded_result = match protobuf::parse_from_bytes::<ffi::QueryBlockHashResponse>(
                    result.as_slice(),
                ) {
                    Ok(res) => res,
                    Err(err) => {
                        println!("Cannot decode protobuf response: {:?}", err);
                        return H256::default();
                    }
                };
                H256::from_slice(decoded_result.hash.as_slice())
            }
            None => {
                println!("Get block hash failed. Empty response");
                H256::default()
            }
        }
    }

    fn block_number(&self) -> U256 {
        self.tx_context.block_number
    }

    fn block_coinbase(&self) -> H160 {
        self.tx_context.block_coinbase
    }

    fn block_timestamp(&self) -> U256 {
        self.tx_context.timestamp
    }

    fn block_difficulty(&self) -> U256 {
        U256::zero() // Only applicable for PoW
    }

    fn block_randomness(&self) -> Option<H256> {
        None
    }

    fn block_gas_limit(&self) -> U256 {
        self.tx_context.block_gas_limit
    }

    fn block_base_fee_per_gas(&self) -> U256 {
        self.tx_context.block_base_fee_per_gas
    }

    fn chain_id(&self) -> U256 {
        self.tx_context.chain_id
    }

    fn exists(&self, address: H160) -> bool {
        self.state.contains_key(&address)
    }

    fn basic(&self, address: H160) -> Basic {
        if address == self.vicinity.origin {
            let mut account_data = self.state.get_account(&address);
            let updated_nonce = account_data
                .nonce
                .checked_sub(U256::from(1u8))
                .unwrap_or(U256::zero());
            if updated_nonce > self.vicinity.nonce {
                account_data.nonce = updated_nonce;
            } else {
                account_data.nonce = self.vicinity.nonce;
            }

            account_data
        } else {
            self.state.get_account(&address)
        }
    }

    fn code(&self, address: H160) -> Vec<u8> {
        self.state.get_account_code(&address).unwrap_or_default()
    }

    fn storage(&self, address: H160, index: H256) -> H256 {
        self.state
            .get_account_storage_cell(&address, &index)
            .unwrap_or_default()
    }

    fn original_storage(&self, _address: H160, _index: H256) -> Option<H256> {
        None
    }
}

impl<'state> FFIBackend<'state> {
    pub fn new(
        querier: *mut querier::GoQuerier,
        storage: &'state mut FFIStorage,
        vicinity: Vicinity,
        tx_context: TxContext,
    ) -> Self {
        Self {
            querier,
            vicinity,
            state: storage,
            logs: vec![],
            tx_context,
        }
    }
}
