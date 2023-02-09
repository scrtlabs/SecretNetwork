pub struct SignedBlock {
    height: u64,
    tx_hash: [u8; 32],
    txs: u32
}

pub struct SignedBlockStorage {
    last_block: SignedBlock
}
