# Secret Contract Dev Guide

This repository can be used to get up and running on a local Secret Network developer testnet (secretdev) to start working with CosmWasm-based smart contracts (soon to be Secret Contracts).

A few important notes:

- smart contracts in this repo are a precursor to Secret Contracts, which enable data privacy
- smart contracts are written in Rust and based on CosmWasm, and the module is referred to as `compute` in the Secret Network
- these CosmWasm-based smart contracts should be reusable and easily modified once we incorporate data privacy

## Setup the Local Developer Testnet

The developer blockchain is configured to run inside a docker container. Install [Docker](https://docs.docker.com/install/) for your environment (Mac, Windows, Linux).

Open a terminal window and change to your project directory.
Then start SecretNetwork, labelled _secretdev_ from here on:

```
$ docker run -it --rm \
 -p 26657:26657 -p 26656:26656 -p 1317:1317 \
 --name secretdev enigmampc/secret-network-bootstrap-sw:latest
```

**NOTE**: The _secretdev_ docker container can be stopped by CTRL+C

![](../images/images/docker-run.png)

At this point you're running a local SecretNetwork full-node. Let's connect to the container so we can view and manage the secret keys:

**NOTE**: In a new terminal

```
docker exec -it secretdev /bin/bash
```

The local blockchain has a couple of keys setup for you (similar to accounts if you're familiar with Truffle Ganache). The keys are stored in the `test` keyring backend, which makes it easier for local development and testing.

```
secretcli keys list --keyring-backend test
```

![](../images/images/secretcli_keys_list.png)

`exit` when you are done

## Setup Secret Contracts (CosmWasm)

Secret Contracts are based on [CosmWasm](https://www.cosmwasm.com) which is implementated on various Cosmos SDK blockchains. The CosmWasm smart contracts are like Ethereum's smart contracts except they can be used on other networks using the [Inter-Blockchain Protocol](https://cosmos.network/ibc) (IBC). CosmWasm smart contracts are written in the Rust language.

The Secret Network has a _compute_ module that we'll use to store, query and instantiate the smart contract. Once stored on the blockchain the smart contract has to be created (or instantiated) in order to execute its methods. This is similar to doing an Ethereum `migrate` using truffle which handles the deployment and creation of a smart contract.

Eventually the smart contracts will become Secret Contracts (in a future blockchain upgrade) running in an SGX enclave (Trusted Execution Environment) where computations are performed on the encrypted contract data (i.e. inputs, state).

Next we'll walkthrough steps to:

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

[Cargo generate](https://doc.rust-lang.org/cargo) is the tool you'll use to create a smart contract project.

```
cargo install cargo-generate --features vendored-openssl
```

## Create Initial Smart Contract

To create the smart contract you'll:

- generate the initial project
- compile the smart contract
- run unit tests
- optimize the wasm contract bytecode to prepare for deployment
- deploy the smart contract to your local SecretNetwork
- instantiate it with contract parameters

Generate the smart contract project

```
cargo generate --git https://github.com/enigmampc/secret-template --name mysimplecounter
```

The git project above is a cosmwasm smart contract template that implements a simple counter. The contract is created with a parameter for the initial count and allows subsequent incrementing.

Change directory to the project you created and view the structure and files that were created.

```
cd mysimplecounter
```

The generate creates a directory with the project name and has this structure:

```
Cargo.lock	Developing.md	LICENSE		Publishing.md	examples	schema		tests
Cargo.toml	Importing.md	NOTICE		README.md	rustfmt.toml	src
```

## Compile

Use the following command to compile the smart contract which produces the wasm contract file.

```
cargo wasm
```

## Unit Tests (NB Tests in this template currently fail unless you have SGX enabled)

Run unit tests

```
RUST_BACKTRACE=1 cargo unit-test
```

## Integration Tests

The integration tests are under the `tests/` directory and run as:

```
cargo integration-test
```

## Generate Msg Schemas

We can also generate JSON Schemas that serve as a guide for anyone trying to use the contract, to specify which arguments they need.

Auto-generate msg schemas (when changed):

```
cargo schema
```

## Deploy Smart Contract

Before deploying or storing the contract on the testnet, need to run the cosmwasm optimizer.

### Optimize compiled wasm

```
docker run --rm -v "$(pwd)":/code \
  --mount type=volume,source="$(basename "$(pwd)")_cache",target=/code/target \
  --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
  cosmwasm/rust-optimizer:0.8.0
```

The contract wasm needs to be optimized to get a smaller footprint. Cosmwasm notes state the contract would be too large for the blockchain unless optimized. This example contract.wasm is 1.8M before optimizing, 90K after.

The optimization creates two files:

- contract.wasm
- hash.txt

### Store the Smart Contract on our local Testnet

```
# First lets start it up again, this time mounting our project's code inside the container.
docker run -it --rm \
 -p 26657:26657 -p 26656:26656 -p 1317:1317 \
 -v $(pwd):/root/code \
 --name secretdev enigmampc/secret-network-bootstrap-sw:latest
```

Upload the optimized contract.wasm to _secretdev_ :

```
docker exec -it secretdev /bin/bash

cd code

secretcli tx compute store contract.wasm --from a --gas auto -y --keyring-backend test
```

### Querying the Smart Contract and Code

List current smart contract code

```
secretcli query compute list-code
[
  {
    "id": 1,
    "creator": "enigma1klqgym9m7pcvhvgsl8mf0elshyw0qhruy4aqxx",
    "data_hash": "0C667E20BA2891536AF97802E4698BD536D9C7AB36702379C43D360AD3E40A14",
    "source": "",
    "builder": ""
  }
]
```

### Instantiate the Smart Contract

At this point the contract's been uploaded and stored on the testnet, but there's no "instance."
This is like `discovery migrate` which handles both the deploying and creation of the contract instance, except in Cosmos the deploy-execute process consists of 3 steps rather than 2 in Ethereum. You can read more about the logic behind this decision, and other comparisons to Solidity, in the [cosmwasm documentation](https://www.cosmwasm.com/docs/getting-started/smart-contracts). These steps are:

1. Upload Code - Upload some optimized wasm code, no state nor contract address (example Standard ERC20 contract)
2. Instantiate Contract - Instantiate a code reference with some initial state, creates new address (example set token name, max issuance, etc for my ERC20 token)
3. Execute Contract - This may support many different calls, but they are all unprivileged usage of a previously instantiated contract, depends on the contract design (example: Send ERC20 token, grant approval to other contract)

To create an instance of this project we must also provide some JSON input data, a starting count.

```bash
INIT="{\"count\": 100000000}"
CODE_ID=1
secretcli tx compute instantiate $CODE_ID "$INIT" --from a --label "my counter" -y --keyring-backend test
```

With the contract now initialized, we can find its address

```bash
secretcli query compute list-contract-by-code 1
```

Our instance is secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg

We can query the contract state

```bash
CONTRACT=secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg
secretcli query compute contract-state smart $CONTRACT "{\"get_count\": {}}"
```

And we can increment our counter

```bash
secretcli tx compute execute $CONTRACT "{\"increment\": {}}" --from a --keyring-backend test
```

## Smart Contract

### Project Structure

The source directory (`src/`) has these files:

```
contract.rs  lib.rs  msg.rs  state.rs
```

The developer modifies `contract.rs` for contract logic, contract entry points are `init`, `handle` and `query` functions.

`init` in the Counter contract initializes the storage, specifically the current count and the signer/owner of the instance being initialized.

We also define `handle`, a generic handler for all functions writing to storage, the counter can be incremented and reset. These functions are provided the storage and the environment, the latter's used by the `reset` function to compare the signer with the contract owner.

Finally we have `query` for all functions reading state, we only have `query_count`, returning the counter state.

The rest of the contract file is unit tests so you can confidently change the contract logic.

The `state.rs` file defines the State struct, used for storing the contract data, the only information persisted between multiple contract calls.

The `msg.rs` file is where the InitMsg parameters are specified (like a constructor), the types of Query (GetCount) and Handle[r](Increment) messages, and any custom structs for each query response.

```rs
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct InitMsg {
    pub count: i32,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum HandleMsg {
    Increment {},
    Reset { count: i32 },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum QueryMsg {
    // GetCount returns the current count as a json-encoded number
    GetCount {},
}

// We define a custom struct for each query response
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct CountResponse {
    pub count: i32,
}

```

### Unit Tests

Unit tests are coded in the `contract.rs` file itself:

```rs
#[cfg(test)]
mod tests {
    use super::*;
    use cosmwasm::errors::Error;
    use cosmwasm::mock::{dependencies, mock_env};
    use cosmwasm::serde::from_slice;
    use cosmwasm::types::coin;

    #[test]
    fn proper_initialization() {
        let mut deps = dependencies(20);

        let msg = InitMsg { count: 17 };
        let env = mock_env(&deps.api, "creator", &coin("1000", "earth"), &[]);

        // we can just call .unwrap() to assert this was a success
        let res = init(&mut deps, env, msg).unwrap();
        assert_eq!(0, res.messages.len());

        // it worked, let's query the state
        let res = query(&deps, QueryMsg::GetCount {}).unwrap();
        let value: CountResponse = from_slice(&res).unwrap();
        assert_eq!(17, value.count);
    }
```

## Resources

Smart Contracts on the Secret Network use cosmwasm. Therefore, for troubleshooting and additional context, cosmwasm documentation may be very useful. Here are some of the links we relied on in putting together this guide:

- [cosmwasm repo](https://github.com/CosmWasm/cosmwasm)
- [cosmwasm starter pack - project template](https://github.com/CosmWasm/cosmwasm-template)
- [Setting up a local "testnet"](https://www.cosmwasm.com/docs/getting-started/using-the-sdk)
- [cosmwasm docs](https://www.cosmwasm.com/docs/intro/overview)

# What's next?

- [CosmWasm JS](../dev/cosmwasm-js.md)
- [Frontend development](../dev/frontend.md)
