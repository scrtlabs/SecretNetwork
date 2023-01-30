use crate::contract_validation::ContractKey;
use cw_types_v010::encoding::Binary;
use lazy_static::lazy_static;
use log::trace;

use std::sync::SgxMutex;

#[derive(Default, Clone, Copy, Debug)]
struct MsgCounter {
    height: u64,
    counter: u64,
}

lazy_static! {
    static ref MSG_COUNTER: SgxMutex<MsgCounter> = SgxMutex::new(MsgCounter::default());
}

pub fn derive_random(seed: &Binary, contract_key: &ContractKey, height: u64) -> Binary {
    let mut counter = MSG_COUNTER.lock().unwrap();

    if counter.height != height {
        counter.height = height;
        counter.counter = 0;
    }

    trace!("counter used to derive random for height {}: {:?}", height, counter);

    let height_bytes = height.to_be_bytes();
    let counter_bytes = counter.counter.to_be_bytes();
    let data = vec![
        height_bytes.as_slice(),
        &contract_key.as_slice(),
        counter_bytes.as_slice()
    ];

    Binary(
        enclave_crypto::hkdf_sha_256(&seed.0.as_slice(), data.as_slice())
            .get()
            .to_vec(),
    )
}

pub fn update_msg_counter(height: u64) {
    let mut counter = MSG_COUNTER.lock().unwrap();

    counter.height = height;
    counter.counter += 1;

    trace!("counter incremented to: {:?}", counter);
}
