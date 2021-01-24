---
title : 'Secret Contracts'
---
# Secret Contracts

Secret Contracts are the first implementation of general purpose privacy preserving computations on public blockchain. While similar to Ethereum smart contracts in design, Secret Contracts work with encrypted data (inputs, encrypted outputs and encrypted state). These privacy guarantees are made possible by a decentralized network of validators, who run Secret Contracts execution inside Trusted Execution Environments (TEEs).

Secret Contracts are Rust based smart contracts that compile to WebAssembly. Secret Contracts, which are based on [Go-CosmWasm](https://github.com/enigmampc/SecretNetwork/tree/master/go-cosmwasm), introduce the _compute_ module that runs inside the TEE to enable secure data processing (inputs, outputs and contract state.

![architecture](https://user-images.githubusercontent.com/15679491/99459758-9a44c580-28fc-11eb-9af2-82479bbb2d23.png)

Next, we will go through steps to:
- install Rust
- install the Rust dependencies
- create your first project

The Rust dependencies include the Rust compiler, cargo (_package manager_), toolchain and a package to generate projects. You can check out the Rust book, rustlings course, examples and more [here](https://www.rust-lang.org/learn).

1. Install Rust

More information about installing Rust can be found here: https://www.rust-lang.org/tools/install.

```
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source $HOME/.cargo/env
```

**Update the rust compiler**

In case rust is installed already, make sure to update the rust compiler.

```
rustup update
```

2. Add rustup target wasm32 for both stable and nightly

```
rustup default stable
rustup target list --installed
rustup target add wasm32-unknown-unknown

rustup install nightly
rustup target add wasm32-unknown-unknown --toolchain nightly
```

3. If using linux, install the standard build tools:
```
apt install build-essential
```

4. Run cargo install cargo-generate

[Cargo generate](https://docs.rs/crate/cargo-generate/) is the tool you'll use to create a secret contract project.

```
cargo install cargo-generate --features vendored-openssl
```

### Create your first Secret Contract

1. generate the initial project
2. compile the secret contract
3. run unit tests
4. optimize the wasm contract bytecode to prepare for deployment
5. deploy the secret contract to your local Secret Network
6. instantiate it with contract parameters

#### Generate the Secret Contract Project

```
cargo generate --git https://github.com/enigmampc/secret-template --name mysimplecounter
```

The git project above is a secret contract template that implements a simple counter. The contract is created with a parameter for the initial count and allows subsequent incrementing.

Change directory to the project you created and view the structure and files that were created.

```
cd mysimplecounter
```

The generate creates a directory with the project name and has this structure:

```
Cargo.lock	Developing.md	LICENSE		Publishing.md	examples	schema		tests
Cargo.toml	Importing.md	NOTICE		README.md	rustfmt.toml	src
```


As an example secret contract, `mysimplecounter`, handles a state keeping track of a number which may be incremented by any address, but only reset by the creator.

The `src` folder contains the following files:

##### `contract.rs` 

This file contains functions which define the available contract operations. The functions which all secret contracts contain will be: `init`, `handle`, and `query`. 

- `init` is called once at instantiation of the secret contract with 3 parameters: `deps`, `env`, and `msg`. The internal state (`State` struct imported from `state.rs`)of the secret contract is initialized with these parameters.

```rust
pub fn init<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
    msg: InitMsg,
) -> StdResult<InitResponse> {
    let state = State {
        count: msg.count,
        owner: deps.api.canonical_address(&env.message.sender)?,
    };

    config(&mut deps.storage).save(&state)?;

    Ok(InitResponse::default())
}
```

[`deps`](https://github.com/enigmampc/SecretNetwork/blob/master/cosmwasm/packages/std/src/traits.rs) and [`env`](https://github.com/enigmampc/SecretNetwork/blob/master/cosmwasm/packages/std/src/types.rs) are structs `Extern` and `Env` imported from [cosmwasm_std](https://github.com/enigmampc/SecretNetwork/tree/master/cosmwasm/packages/std)

`deps` contains all external dependencies of the contract.

```rust
pub struct Extern<S: Storage, A: Api, Q: Querier> {
    pub storage: S,
    pub api: A,
    pub querier: Q,
}
```

`env` contains external state information of the contract.

```rust
pub struct Env {
    pub block: BlockInfo,
    pub message: MessageInfo,
    pub contract: ContractInfo,
    pub contract_key: Option<String>,
    #[serde(default)]
    pub contract_code_hash: String,
}
```

   - `BlockInfo` defines the current block height, time, and chain-id. 
   - `MessageInfo` defines the address which instantiated the contract and possibly funds sent to the contract at instantiation. 
   - `ContractInfo` is the address of the contract instance.
   - `contract_key` is the code-id used when instantiating the contract
   - `contract_code_hash` is the hex encoded hash of the code. This is used by Secret Network to harden against replaying the contract. It is used to bind the request to a destination contract in a stronger way than just the contract address which can be faked


`msg` is the `InitMsg` struct imported from `msg.rs`. In this case, it defines the initial state of the counter.
```rust
pub struct InitMsg {
    pub count: i32,
}
```

The return value of `init`(if there are no errors) is an `InitResponse`.
```rust
pub struct InitResponse<T = Empty>
where
    T: Clone + fmt::Debug + PartialEq + JsonSchema,
{
    pub messages: Vec<CosmosMsg<T>>,
    pub log: Vec<LogAttribute>,
}
```

- `handle` handles all incoming transactions. These computations occur on-chain. 

The `msg` parameter is the `HandleMsg` struct imported from `msg.rs`. This struct defines the available operations, while the callable functions(`try_increment` and `try_reset`) are defined in `contract.rs`. 

```rust
pub fn handle<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
    msg: HandleMsg,
) -> StdResult<HandleResponse> {
    match msg {
        HandleMsg::Increment {} => try_increment(deps, env),
        HandleMsg::Reset { count } => try_reset(deps, env, count),
    }
}
```

- `query` messages are off-chain computations. 

The `msg` parameter is the `QueryMsg` struct imported from `msg.rs`. This struct defines the available operations, while the callable function(`query_count`) is defined in `contract.rs`. 

```rust
pub fn query<S: Storage, A: Api, Q: Querier>(
    deps: &Extern<S, A, Q>,
    msg: QueryMsg,
) -> StdResult<Binary> {
    match msg {
        QueryMsg::GetCount {} => to_binary(&query_count(deps)?),
    }
}
```

##### `state.rs`

The internal state is defined in this file, by the `State` struct. 

```rust
pub struct State {
    pub count: i32,
    pub owner: CanonicalAddr,
}
```

The state is saved in a [`Storage`](https://github.com/enigmampc/SecretNetwork/blob/master/cosmwasm/packages/std/src/traits.rs#L42-L72) struct.

This `Storage` struct is wrapped in a [`Singleton` and `ReadonlySingleton`](https://github.com/enigmampc/SecretNetwork/tree/master/cosmwasm/packages/storage#singleton). To learn more about the different types of storage, read the documentation for [cosmwasm_storage](https://github.com/enigmampc/SecretNetwork/tree/master/cosmwasm/packages/storage). 

```rust
pub fn config<S: Storage>(storage: &mut S) -> Singleton<S, State> {
    singleton(storage, CONFIG_KEY)
}

pub fn config_read<S: Storage>(storage: &S) -> ReadonlySingleton<S, State> {
    singleton_read(storage, CONFIG_KEY)
}
```

In this example, the state contains an integer `count` and the `owner` of the contract. `owner` is an instance of the [`CanonicalAddr`](https://github.com/enigmampc/SecretNetwork/blob/master/cosmwasm/packages/std/src/addresses.rs#L56-L88) struct. `Storage` and `CanonicalAddr` are imported from [cosmwasm_std](https://github.com/enigmampc/SecretNetwork/tree/master/cosmwasm/packages/std).

##### `msg.rs`

Contract computations are defined as messages. The [JsonSchema](https://docs.rs/schemars/0.8.0/schemars/trait.JsonSchema.html) for these messages is defined in `msg.rs`. 

The `InitMsg` struct describes the `msg` parameter passed to the contract `init` function in `contract.rs`. 

`HandleMsg` and `QueryMsg` are enums representing possible transaction or query computations. These enums are used to describe the `msg` parameter for `handle` and `query` functions in `contract.rs`. 

```rust
pub enum HandleMsg {
    Increment {},
    Reset { count: i32 },
}

pub enum QueryMsg {
    GetCount {},
}
```

Calls to `handle` and `query` can optionally provide a response. These response messages are defined as structs. In this example, the `query_count` function in `contract.rs` returns the `CountResponse` struct. 
```rust
pub struct CountResponse {
    pub count: i32,
}
```

##### `lib.rs`

This file is essentially boilerplate that you shouldn't need to modify. It provides the necessary abstraction to interact with the wasmi VM and compile the contract to wasm.

#### Compile the Secret Contract

Use the following command to compile the Secret Contract, which produces the wasm contract file.

```
cargo wasm
```

#### Run Unit Tests

*Tests in this template currently fail unless you have SGX enabled.*

```
RUST_BACKTRACE=1 cargo unit-test
```

#### Integration Tests

The integration tests are under the `tests/` directory and run as:

```
cargo integration-test
```

#### Generate Msg Schemas

We can also generate JSON Schemas that serve as a guide for anyone trying to use the contract, to specify which arguments they need.

Auto-generate msg schemas (when changed):

```
cargo schema
```
