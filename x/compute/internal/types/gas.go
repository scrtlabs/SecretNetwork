package types

// GasMultiplier is how many cosmwasm gas points = 1 sdk gas point
// SDK reference costs can be found here: https://github.com/cosmos/cosmos-sdk/blob/02c6c9fafd58da88550ab4d7d494724a477c8a68/store/types/gas.go#L153-L164
// A write at ~3000 gas and ~200us = 10 gas per us (microsecond) cpu/io
// Rough timing have 88k gas at 90us, which is equal to 1k sdk gas... (one read)
//
// Please not that all gas prices returned to the wasmer engine should have this multiplied
const GasMultiplier uint64 = 1000

// MaxGas for a contract is 10 billion wasmer gas (enforced in rust to prevent overflow)
// The limit for v0.9.3 is defined here: https://github.com/CosmWasm/cosmwasm/blob/v0.9.3/packages/vm/src/backends/singlepass.rs#L15-L23
// (this will be increased in future releases)
const MaxGas = 10_000_000_000

// InstanceCost is how much SDK gas we charge each time we load a WASM instance.
// Creating a new instance is costly, and this helps put a recursion limit to contracts calling contracts.
const InstanceCost uint64 = 10_000

// CompileCost is how much SDK gas we charge *per byte* for compiling WASM code.
const CompileCost uint64 = 2
