# CosmWasm

[![CircleCI](https://circleci.com/gh/CosmWasm/cosmwasm/tree/main.svg?style=shield)](https://circleci.com/gh/CosmWasm/cosmwasm/tree/main)

**WebAssembly Smart Contracts for the Cosmos SDK.**

## Packages

The following packages are maintained here:

| Crate            | Usage                | Download                                                                                                                            | Docs                                                                                    |
| ---------------- | -------------------- | ----------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------- |
| cosmwasm-crypto  | Internal only        | [![cosmwasm-crypto on crates.io](https://img.shields.io/crates/v/cosmwasm-crypto.svg)](https://crates.io/crates/cosmwasm-crypto)    | [![Docs](https://docs.rs/cosmwasm-crypto/badge.svg)](https://docs.rs/cosmwasm-crypto)   |
| cosmwasm-derive  | Internal only        | [![cosmwasm-derive on crates.io](https://img.shields.io/crates/v/cosmwasm-derive.svg)](https://crates.io/crates/cosmwasm-derive)    | [![Docs](https://docs.rs/cosmwasm-derive/badge.svg)](https://docs.rs/cosmwasm-derive)   |
| cosmwasm-schema  | Contract development | [![cosmwasm-schema on crates.io](https://img.shields.io/crates/v/cosmwasm-schema.svg)](https://crates.io/crates/cosmwasm-schema)    | [![Docs](https://docs.rs/cosmwasm-schema/badge.svg)](https://docs.rs/cosmwasm-schema)   |
| cosmwasm-std     | Contract development | [![cosmwasm-std on crates.io](https://img.shields.io/crates/v/cosmwasm-std.svg)](https://crates.io/crates/cosmwasm-std)             | [![Docs](https://docs.rs/cosmwasm-std/badge.svg)](https://docs.rs/cosmwasm-std)         |
| cosmwasm-storage | Contract development | [![cosmwasm-storage on crates.io](https://img.shields.io/crates/v/cosmwasm-storage.svg)](https://crates.io/crates/cosmwasm-storage) | [![Docs](https://docs.rs/cosmwasm-storage/badge.svg)](https://docs.rs/cosmwasm-storage) |
| cosmwasm-vm      | Host environments    | [![cosmwasm-vm on crates.io](https://img.shields.io/crates/v/cosmwasm-vm.svg)](https://crates.io/crates/cosmwasm-vm)                | [![Docs](https://docs.rs/cosmwasm-vm/badge.svg)](https://docs.rs/cosmwasm-vm)           |

## Overview

To get that contract to interact with a system needs many moving parts. To get
oriented, here is a list of the various components of the CosmWasm ecosystem:

**Standard library:**

This code is compiled into Wasm bytecode as part of the smart contract.

- [cosmwasm-std](https://github.com/CosmWasm/cosmwasm/tree/main/packages/std) -
  A crate in this workspace. Provides the bindings and all imports needed to
  build a smart contract.
- [cosmwasm-storage](https://github.com/CosmWasm/cosmwasm/tree/main/packages/storage) -
  A crate in this workspace. This optional addition to `cosmwasm-std` includes
  convenience helpers for interacting with storage.
- [cw-storage-plus](https://github.com/CosmWasm/cosmwasm-plus/tree/main/packages/storage-plus) -
  A crate in `cosmwasm-plus`, which fills the same role as `cosmwasm-storage`,
  but with much more powerful types supporting composite primary keys, secondary
  indexes, automatic snapshotting, and more. This is newer and a bit less stable
  than `cosmwasm-storage` but used in most modern contracts.

**Building contracts:**

- [cosmwasm-template](https://github.com/CosmWasm/cosmwasm-template) - A
  starter-pack to get you quickly building your custom contract compatible with
  the cosmwasm system.
- [cosmwasm-plus](https://github.com/CosmWasm/cosmwasm-plus) - Some sample
  contracts for use and inspiration. These provide usable primitives and
  interfaces for many use cases, such as fungible tokens, NFTs, multisigs,
  governance votes, staking derivatives, and more. Look in `packages` for docs
  on the various standard interfaces, and `contracts` for the implementations.
  Please submit your contract or interface via PR.
- [rust-optimizer](https://github.com/cosmwasm/rust-optimizer) - A docker image
  and scripts to take your Rust code and produce the smallest possible Wasm
  output, deterministically. This is designed both for preparing contracts for
  deployment as well as validating that a given deployed contract is based on
  some given source code, allowing a
  [similar contract verification algorithm](https://medium.com/coinmonks/how-to-verify-and-publish-on-etherscan-52cf25312945)
  as Etherscan.
- [serde-json-wasm](https://github.com/CosmWasm/serde-json-wasm) - A custom json
  library, forked from `serde-json-core`. This provides an interface similar to
  `serde-json`, but without ay floating-point instructions (non-deterministic)
  and producing builds around 40% of the code size.

**Executing contracts:**

- [cosmwasm-vm](https://github.com/CosmWasm/cosmwasm/tree/main/packages/vm) - A
  crate in this workspace. Uses the [wasmer](https://github.com/wasmerio/wasmer)
  engine to execute a given smart contract. Also contains code for gas metering,
  storing, and caching wasm artifacts.
- [wasmvm](https://github.com/CosmWasm/wasmvm) - High-level go bindings to all
  the power inside `cosmwasm-vm`. Easily allows you to upload, instantiate and
  execute contracts, making use of all the optimizations and caching available
  inside `cosmwasm-vm`.
- [wasmd](https://github.com/CosmWasm/wasmd) - A basic Cosmos SDK app to host
  WebAssembly smart contracts. It can be run as is, or you can import the
  `x/wasm` module from it and use it in your blockchain. It is designed to be
  imported and customized for other blockchains, rather than forked.

## Creating a Smart Contract

You can see some examples of contracts under the `contracts` directory, which
you can look at. They are simple and self-contained, primarily meant for testing
purposes, but that also makes them easier to understand.

You can also look at [cosmwasm-plus](https://github.com/CosmWasm/cosmwasm-plus)
for examples and inspiration on more production-like contracts and also how we
call one contract from another. If you are working on DeFi or Tokens, please
look at the `cw20`, `cw721` and/or `cw1155` packages that define standard
interfaces as analogues to some popular ERC designs. (`cw20` is also inspired by
`erc777`).

If you want to get started building you own contract, the simplest way is to go
to the [cosmwasm-template](https://github.com/CosmWasm/cosmwasm-template)
repository and follow the instructions. This will give you a simple contract
along with tests, and a properly configured build environment. From there you
can edit the code to add your desired logic and publish it as an independent
repo.

We also recommend you review our [documentation site](https://docs.cosmwasm.com)
which contains a few tutorials to guide you in building your first contracts. We
also do public workshops on various topics about once a month. You can find
[past recordings under the "Videos" section](https://cosmwasm.com/resources), or
[join our Discord server](https://docs.cosmwasm.com/chat) to ask for help.

## Minimum Supported Rust Version (MSRV)

See [Minimum Supported Rust Version (MSRV)](./docs/MSRV.md).

## API entry points

WebAssembly contracts are basically black boxes. The have no default entry
points, and no access to the outside world by default. To make them useful, we
need to add a few elements.

If you haven't worked with WebAssembly before, please read an overview on
[how to create imports and exports](./EntryPoints.md) in general.

### Exports

The required exports provided by the cosmwasm smart contract are:

```rust
// signal for 0.16 compatibility
extern "C" fn interface_version_7() -> () {}

// copy memory to/from host, so we can pass in/out Vec<u8>
extern "C" fn allocate(size: usize) -> u32;
extern "C" fn deallocate(pointer: u32);

// main contract entry points
extern "C" fn instantiate(env_ptr: u32, info_ptr: u32, msg_ptr: u32) -> u32;
extern "C" fn execute(env_ptr: u32, info_ptr: u32, msg_ptr: u32) -> u32;
extern "C" fn query(env_ptr: u32, msg_ptr: u32) -> u32;
```

Contracts may also implement one or more of the following to extend their
functionality:

```rust
// in-place contract migrations
extern "C" fn migrate(env_ptr: u32, info_ptr: u32, msg_ptr: u32) -> u32;

// support submessage callbacks
extern "C" fn reply(env_ptr: u32, msg_ptr: u32) -> u32;

// expose privileged entry points to Cosmos SDK modules, not external accounts
extern "C" fn sudo(env_ptr: u32, msg_ptr: u32) -> u32;

// and to write an IBC application as a contract, implement these:
extern "C" fn ibc_channel_open(env_ptr: u32, msg_ptr: u32) -> u32;
extern "C" fn ibc_channel_connect(env_ptr: u32, msg_ptr: u32) -> u32;
extern "C" fn ibc_channel_close(env_ptr: u32, msg_ptr: u32) -> u32;
extern "C" fn ibc_packet_receive(env_ptr: u32, msg_ptr: u32) -> u32;
extern "C" fn ibc_packet_ack(env_ptr: u32, msg_ptr: u32) -> u32;
extern "C" fn ibc_packet_timeout(env_ptr: u32, msg_ptr: u32) -> u32;
```

`allocate`/`deallocate` allow the host to manage data within the Wasm VM. If
you're using Rust, you can implement them by simply
[re-exporting them from cosmwasm::exports](https://github.com/CosmWasm/cosmwasm/blob/v0.6.3/contracts/hackatom/src/lib.rs#L5).
`instantiate`, `execute` and `query` must be defined by your contract.

### Imports

The imports provided to give the contract access to the environment are:

```rust
// This interface will compile into required Wasm imports.
// A complete documentation those functions is available in the VM that provides them:
// https://github.com/CosmWasm/cosmwasm/blob/0.7/lib/vm/src/instance.rs#L43
//
extern "C" {
    fn db_read(key: u32) -> u32;
    fn db_write(key: u32, value: u32);
    fn db_remove(key: u32);

    // scan creates an iterator, which can be read by consecutive next() calls
    #[cfg(feature = "iterator")]
    fn db_scan(start: u32, end: u32, order: i32) -> u32;
    #[cfg(feature = "iterator")]
    fn db_next(iterator_id: u32) -> u32;

    fn addr_validate(source_ptr: u32) -> u32;
    fn addr_canonicalize(source: u32, destination: u32) -> u32;
    fn addr_humanize(source: u32, destination: u32) -> u32;

    /// Verifies message hashes against a signature with a public key, using the
    /// secp256k1 ECDSA parametrization.
    /// Returns 0 on verification success, 1 on verification failure, and values
    /// greater than 1 in case of error.
    fn secp256k1_verify(message_hash_ptr: u32, signature_ptr: u32, public_key_ptr: u32) -> u32;

    /// Verifies a message against a signature with a public key, using the
    /// ed25519 EdDSA scheme.
    /// Returns 0 on verification success, 1 on verification failure, and values
    /// greater than 1 in case of error.
    fn ed25519_verify(message_ptr: u32, signature_ptr: u32, public_key_ptr: u32) -> u32;

    /// Verifies a batch of messages against a batch of signatures and public keys, using the
    /// ed25519 EdDSA scheme.
    /// Returns 0 on verification success, 1 on verification failure, and values
    /// greater than 1 in case of error.
    fn ed25519_batch_verify(messages_ptr: u32, signatures_ptr: u32, public_keys_ptr: u32) -> u32;

    /// Executes a query on the chain (import). Not to be confused with the
    /// query export, which queries the state of the contract.
    fn query_chain(request: u32) -> u32;

    /// Evaporates a specified amount of gas (1 evaporate = 1 sdk gas)
    fn gas_evaporate(evaporate: u32) -> u32;
}

```

(from
[imports.rs](https://github.com/CosmWasm/cosmwasm/blob/0.7/src/imports.rs))

You could actually implement a WebAssembly module in any language, and as long
as you implement these functions, it will be interoperable, given the JSON data
passed around is the proper format.

Note that these `*c_void` pointers refers to a Region pointer, containing the
offset and length of some Wasm memory, to allow for safe access between the
caller and the contract:

```rust
/// Refers to some heap allocated data in Wasm.
/// A pointer to an instance of this can be returned over FFI boundaries.
///
/// This struct is crate internal since the VM defined the same type independently.
#[repr(C)]
pub struct Region {
    pub offset: u32,
    /// The number of bytes available in this region
    pub capacity: u32,
    /// The number of bytes used in this region
    pub length: u32,
}
```

(from
[memory.rs](https://github.com/CosmWasm/cosmwasm/blob/main/src/memory.rs#L7-L13))

## Implementing the Smart Contract

If you followed the [instructions above](#Creating), you should have a runable
smart contract. You may notice that all of the Wasm exports are taken care of by
`lib.rs`, which should shouldn't need to modify. What you need to do is simply
look in `contract.rs` and implement `instantiate` and `execute` functions,
defining your custom `InstantiateMsg` and `ExecuteMsg` structs for parsing your
custom message types (as json):

```rust
#[entry_point]
pub fn instantiate(
  deps: DepsMut,
  env: Env,
  info: MessageInfo,
  msg: InstantiateMsg,
) -> Result<Response, ContractError> {}

#[entry_point]
pub fn execute(
  deps: DepsMut,
  env: Env,
  info: MessageInfo,
  msg: ExecuteMsg,
) -> Result<Response, ContractError> {}

#[entry_point]
pub fn query(deps: Deps, env: Env, msg: QueryMsg) -> Result<Binary, ContractError> {}

#[entry_point]
pub fn migrate(deps: DepsMut, env: Env, msg: MigrateMsg) -> Result<Response, ContractError> {}
```

The low-level `c_read` and `c_write` imports are nicely wrapped for you by a
`Storage` implementation (which can be swapped out between real Wasm code and
test code). This gives you a simple way to read and write data to a custom
sub-database that this contract can safely write as it wants. It's up to you to
determine which data you want to store here:

```rust
pub trait Storage {
  fn get(&self, key: &[u8]) -> Option<Vec<u8>>;
  fn set(&mut self, key: &[u8], value: &[u8]);
  fn remove(&mut self, key: &[u8]);

  // and for iterating over a range of values
  #[cfg(feature = "iterator")]
  fn range<'a>(
    &'a self,
    start: Option<&[u8]>,
    end: Option<&[u8]>,
    order: Order,
  ) -> Box<dyn Iterator<Item = Pair> + 'a>;
}
```

## Testing the Smart Contract (rust)

For quick unit tests and useful error messages, it is often helpful to compile
the code using native build system and then test all code except for the
`extern "C"` functions (which should just be small wrappers around the real
logic).

If you have non-trivial logic in the contract, please write tests using rust's
standard tooling. If you run `cargo test`, it will compile into native code
using the `debug` profile, and you get the normal test environment you know and
love. Notably, you can add plenty of requirements to `[dev-dependencies]` in
`Cargo.toml` and they will be available for your testing joy. As long as they
are only used in `#[cfg(test)]` blocks, they will never make it into the
(release) Wasm builds and have no overhead on the production artifact.

Note that for tests, you can use the `MockStorage` implementation which gives a
generic in-memory hashtable in order to quickly test your logic. You can see a
[simple example how to write a test](https://github.com/CosmWasm/cosmwasm/blob/81b6702d3994c8c34fb51c53176993b7e672860b/contracts/hackatom/src/contract.rs#L70-L88)
in our sample contract.

## Testing the Smart Contract (wasm)

You may also want to ensure the compiled contract interacts with the environment
properly. To do so, you will want to create a canonical release build of the
`<contract>.wasm` file and then write tests in with the same VM tooling we will
use in production. This is a bit more complicated but we added some tools to
help in
[cosmwasm-vm](https://github.com/CosmWasm/cosmwasm/tree/main/packages/vm) which
can be added as a `dev-dependency`.

You will need to first compile the contract using `cargo wasm`, then load this
file in the integration tests. Take a
[look at the sample tests](https://github.com/CosmWasm/cosmwasm/blob/main/contracts/hackatom/tests/integration.rs)
to see how to do this... it is often quite easy to port a unit test to an
integration test.

## Production Builds

The above build process (`cargo wasm`) works well to produce wasm output for
testing. However, it is quite large, around 1.5 MB likely, and not suitable for
posting to the blockchain. Furthermore, it is very helpful if we have
reproducible build step so others can prove the on-chain wasm code was generated
from the published rust code.

For that, we have a separate repo,
[rust-optimizer](https://github.com/CosmWasm/rust-optimizer) that provides a
[docker image](https://hub.docker.com/r/CosmWasm/rust-optimizer/tags) for
building. For more info, look at
[rust-optimizer README](https://github.com/CosmWasm/rust-optimizer/blob/master/README.md#usage),
but the quickstart guide is:

```sh
docker run --rm -v "$(pwd)":/code \
  --mount type=volume,source="$(basename "$(pwd)")_cache",target=/code/target \
  --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
  cosmwasm/rust-optimizer:0.11.4
```

It will output a highly size-optimized build as `contract.wasm` in `$CODE`. With
our example contract, the size went down to 126kB (from 1.6MB from
`cargo wasm`). If we didn't use serde-json, this would be much smaller still...

## Benchmarking

You may want to compare how long the contract takes to run inside the Wasm VM
compared to in native rust code, especially for computationally intensive code,
like hashing or signature verification.

Here's how to benchmark your CosmWasm contracts:

### Native Rust Benchmarking

1. Add the `criterion` crate to your `dev-dependencies` in `Cargo.toml`:
   ```toml
   [dev-dependencies]
   criterion = "0.4"
   ```

2. Create a `benches` directory in your project root and add a benchmark file (e.g., `benches/my_benchmark.rs`):
   ```rust
   use criterion::{black_box, criterion_group, criterion_main, Criterion};
   use your_contract::contract::{execute, instantiate}; // Import your contract functions
   use your_contract::msg::{ExecuteMsg, InstantiateMsg};
   use cosmwasm_std::testing::{mock_dependencies, mock_env, mock_info};

   fn instantiate_benchmark(c: &mut Criterion) {
       let mut deps = mock_dependencies();
       let env = mock_env();
       let info = mock_info("creator", &[]);
       let msg = InstantiateMsg { /* your params here */ };

       c.bench_function("instantiate", |b| {
           b.iter(|| instantiate(black_box(&mut deps), black_box(env.clone()), black_box(info.clone()), black_box(msg.clone())))
       });
   }

   fn execute_benchmark(c: &mut Criterion) {
       // Set up contract state first by instantiating
       let mut deps = mock_dependencies();
       let env = mock_env();
       let info = mock_info("creator", &[]);
       let init_msg = InstantiateMsg { /* your params here */ };
       let _ = instantiate(&mut deps, env.clone(), info.clone(), init_msg).unwrap();

       // Now benchmark execute
       let exec_msg = ExecuteMsg::YourAction { /* params */ };
       c.bench_function("execute_your_action", |b| {
           b.iter(|| execute(black_box(&mut deps), black_box(env.clone()), black_box(info.clone()), black_box(exec_msg.clone())))
       });
   }

   criterion_group!(benches, instantiate_benchmark, execute_benchmark);
   criterion_main!(benches);
   ```

3. Run the benchmarks with:
   ```sh
   cargo bench
   ```

### Wasm VM Benchmarking

To benchmark in the Wasm VM environment:

1. Add `cosmwasm-vm` to your `dev-dependencies`:
   ```toml
   [dev-dependencies]
   cosmwasm-vm = { version = "1.3", features = ["stargate"] }
   ```

2. Create a benchmark file in your `tests` directory:
   ```rust
   #[cfg(test)]
   mod vm_benchmarks {
       use cosmwasm_std::{Addr, Coin, Empty};
       use cosmwasm_vm::{
           testing::{mock_backend, mock_env, mock_info, MockApi, MockQuerier, MockStorage},
           Instance, InstanceOptions, Module,
       };
       use std::time::Instant;

       #[test]
       fn benchmark_wasm_execution() {
           // Compile contract
           let wasm = std::fs::read("./target/wasm32-unknown-unknown/release/your_contract.wasm")
               .expect("Failed to read Wasm file");

           // Create VM instance
           let backend = mock_backend(&[]);
           let options = InstanceOptions {
               gas_limit: 100_000_000,
               print_debug: false,
           };
           let module = Module::from_code(&wasm).unwrap();
           let mut instance = Instance::from_module(
               &module,
               backend,
               options,
               MockStorage::default(),
               MockApi::default(),
               MockQuerier::default(),
           )
           .unwrap();

           // Prepare instantiate message
           let info = mock_info("creator", &[]);
           let instantiate_msg = r#"{"your_param": "value"}"#;

           // Benchmark instantiate
           let start = Instant::now();
           let _res = instance.instantiate(
               &mock_env("creator", &[]),
               &info,
               instantiate_msg.as_bytes(),
           ).unwrap();
           let duration = start.elapsed();
           println!("Instantiate execution time: {:?}", duration);

           // Prepare execute message
           let execute_msg = r#"{"your_action": {"param": "value"}}"#;

           // Benchmark execute
           let start = Instant::now();
           let _res = instance.execute(
               &mock_env("user", &[]),
               &info,
               execute_msg.as_bytes(),
           ).unwrap();
           let duration = start.elapsed();
           println!("Execute execution time: {:?}", duration);
       }
   }
   ```

3. Build your Wasm contract:
   ```sh
   RUSTFLAGS='-C link-arg=-s' cargo wasm --release
   ```

4. Run the benchmark:
   ```sh
   cargo test vm_benchmarks --release -- --nocapture
   ```

### Comparing Results

When comparing the results, keep in mind:

1. The Wasm VM adds overhead for serialization/deserialization of data
2. The VM has gas metering which adds additional overhead
3. The compilation flags used for Wasm optimization can significantly affect performance

For a comprehensive comparison, record multiple runs for each benchmark to account for variance.

## Developing

The ultimate auto-updating guide to building this project is the CI
configuration in `.circleci/config.yml`.

For manually building this repo locally during development, here are a few
commands. They assume you use a stable Rust version by default and have a
nightly toolchain installed as well.

**Workspace**

```sh
./devtools/check_workspace.sh
```

**Contracts**

| Step | Description                      | Command                                |
| ---- | -------------------------------- | -------------------------------------- |
| 1    | fast checks, rebuilds lock files | `./devtools/check_contracts_fast.sh`   |
| 2    | medium fast checks               | `./devtools/check_contracts_medium.sh` |
| 3    | slower checks                    | `./devtools/check_contracts_full.sh`   |
