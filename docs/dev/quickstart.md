# Secret Contracts Quickstart

Get up and running on Secret Network with a local docker environment, as well as testnet (holodeck), to start working with Secret Contracts.

To learn more about secret contracts, please visit our [documentation page](https://build.scrt.network/dev/secret-contracts.html).

<details>
  <summary>Topics covered on this page</summary>

  - [Setup the Local Developer Testnet](#setup-the-local-developer-testnet)
  - [Setup Secret Contracts](#setup-secret-contracts)
  - [Create Initial Smart Contract](#create-initial-smart-contract)
  - [Deploy Smart Contract to local Testnet](#deploy-smart-contract-to-our-local-testnet)
  - [Instantiate the Smart Contract](#instantiate-the-smart-contract)
  - Migrating to Testnet
    * [Deploy to the Holodeck Testnet](#deploy-to-the-holodeck-testnet)
    * [Get SCRT from faucet](#get-some-scrt-from-the-faucet)
    * [Store the Secret Contract](#store-the-secret-contract-on-holodeck)
  - Secret Contracts
    * [Secret Contracts 101](#secret-contracts-101)
    * [Secret Contract code explanation](#secret-contract-code-explanation)
    * [Secret Contracts - Advanced](#secret-contracts---advanced)
  - [SecretJS](#secretjs)
  - [Keplr integration](#keplr-integration)
  - Other resources
    * [Privacy model of secret contracts](#privacy-model-of-secret-contracts)
    * [Tutorials from Secret Network community](#tutorials-from-secret-network-community)
    * [CosmWasm resources](#cosmwasm-resources)
  
</details>

## Setup the Local Developer Testnet

The developer blockchain is configured to run inside a docker container. Install [Docker](https://docs.docker.com/install/) for your environment (Mac, Windows, Linux).

Open a terminal window and change to your project directory.
Then start SecretNetwork, labelled _secretdev_ from here on:

```bash
docker run -it --rm \
 -p 26657:26657 -p 26656:26656 -p 1337:1337 \
 --name secretdev enigmampc/secret-network-sw-dev
```

**NOTE**: The _secretdev_ docker container can be stopped by CTRL+C

![](docker-run.png)

At this point you're running a local SecretNetwork full-node. Let's connect to the container so we can view and manage the secret keys:

**NOTE**: In a new terminal
```bash
docker exec -it secretdev /bin/bash
```

The local blockchain has a couple of keys setup for you (similar to accounts if you're familiar with Truffle Ganache). The keys are stored in the `test` keyring backend, which makes it easier for local development and testing.

```bash
secretcli keys list --keyring-backend test
````

![](secretcli-keys-list.png)

`exit` when you are done

## Setup Secret Contracts

In order to setup Secret Contracts on your development environment, you will need to:
- install Rust (you don't need to be a Rust expert to build Secret Contracts and you can check out the Rust book, rustlings course, examples to learn more at https://www.rust-lang.org/learn)
- install the Rust dependencies
- create your first project

The Rust dependencies include the Rust compiler, cargo (_package manager_), toolchain and a package to generate projects (you can check out the Rust book, rustlings course, examples and more at https://www.rust-lang.org/learn).


1. Install Rust

More information about installing Rust can be found here: https://www.rust-lang.org/tools/install.

```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source $HOME/.cargo/env
```

2. Add rustup target wasm32 for both stable and nightly

```bash
rustup default stable
rustup target list --installed
rustup target add wasm32-unknown-unknown

rustup install nightly
rustup target add wasm32-unknown-unknown --toolchain nightly
```

3. If using linux, install the standard build tools:
```bash
apt install build-essential
```

4. Run cargo install cargo-generate

Cargo generate is the tool you'll use to create a smart contract project (https://doc.rust-lang.org/cargo).

```bash
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

### Generate the smart contract project

```bash
cargo generate --git https://github.com/enigmampc/secret-template --name mysimplecounter
```

The git project above is a cosmwasm smart contract template that implements a simple counter. The contract is created with a parameter for the initial count and allows subsequent incrementing.

Change directory to the project you created and view the structure and files that were created.

```bash
cd mysimplecounter
```

The generate creates a directory with the project name and has this structure:

```
Cargo.lock	Developing.md	LICENSE		Publishing.md	examples	schema		tests
Cargo.toml	Importing.md	NOTICE		README.md	rustfmt.toml	src
```

### Compile

Use the following command to compile the smart contract which produces the wasm contract file.

```bash
cargo wasm
```

### Unit Tests 

#### Run unit tests

```bash
RUST_BACKTRACE=1 cargo unit-test
```

#### Integration Tests

The integration tests are under the `tests/` directory and run as:

```bash
cargo integration-test
```

#### Generate Msg Schemas

We can also generate JSON Schemas that serve as a guide for anyone trying to use the contract, to specify which arguments they need.

Auto-generate msg schemas (when changed):

```bash
cargo schema
```


### Deploy Smart Contract to our local Testnet

Before deploying or storing the contract on a testnet, you need to run the [secret contract optimizer](https://hub.docker.com/r/enigmampc/secret-contract-optimizer).

#### Optimize compiled wasm

```bash
docker run --rm -v "$(pwd)":/contract \
  --mount type=volume,source="$(basename "$(pwd)")_cache",target=/code/target \
  --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
  enigmampc/secret-contract-optimizer  
```
The contract wasm needs to be optimized to get a smaller footprint. Cosmwasm notes state the contract would be too large for the blockchain unless optimized. This example contract.wasm is 1.8M before optimizing, 90K after.

This creates a zip of two files:
- contract.wasm
- hash.txt

#### Store the Smart Contract

```bash
# First lets start it up again, this time mounting our project's code inside the container.
docker run -it --rm \
 -p 26657:26657 -p 26656:26656 -p 1337:1337 \
 -v $(pwd):/root/code \
 --name secretdev enigmampc/secret-network-sw-dev
 ```

Upload the optimized contract.wasm.gz:

```bash
docker exec -it secretdev /bin/bash

cd code

secretcli tx compute store contract.wasm.gz --from a --gas 1000000 -y --keyring-backend test
```

#### Querying the Smart Contract and Code

List current smart contract code
```bash
secretcli query compute list-code
[
  {
    "id": 1,
    "creator": "secret1zy80x04d4jh4nvcqmamgjqe7whus5tcw406sna",
    "data_hash": "D98F0CA3E8568B6B59772257E07CAC2ED31DD89466BFFAA35B09564B39484D92",
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
INIT='{"count": 100000000}'
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

secretcli query compute query $CONTRACT '{"get_count": {}}'
```

And we can increment our counter
```bash
secretcli tx compute execute $CONTRACT '{"increment": {}}' --from a --keyring-backend test
```

### Deploy to the Holodeck Testnet

Holodeck is the official Secret Network testnet. To deploy your contract to the testnet follow these steps:

1. Install and configure the Secret Network Light Client
2. Get some SCRT from the faucet
3. Store the Secret Contract on Holodeck
4. Instantiate your Secret Contract

#### Install and Configure the Secret Network Light Client

If you don't have the latest `secretcli`, using these [steps](https://github.com/enigmampc/SecretNetwork/blob/master/docs/testnet/install_cli.md) to download the 
CLI and add its location to your PATH.

Before deploying your contract make sure it's configured to point to an existing RPC node. You can also use the testnet bootstrap node. Set the `chain-id` to 
`holodeck-2`. Below we've also got a config setting to point to the `test` keyring backend which allows you to interact with the testnet and your contract 
without providing an account password each time.

```bash
secretcli config node http://bootstrap.secrettestnet.io:26657

secretcli config chain-id holodeck-2

secretcli config trust-node true

secretcli config keyring-backend test
```

*NOTE*: To reset your `keyring-backend`, use `secretcli config keyring-backend os`.

#### Get some SCRT from the faucet

Create a key for the Holodeck testnet that you'll use to get SCRT from the faucet, store and instantiate the contract, and other testnet transactions.

```bash
secretcli keys add <your account alias>
```

This will output your address, a 45 character-string starting with `secret1...`. Copy/paste it to get some testnet SCRT from 
[the faucet](https://faucet.secrettestnet.io/). 
Continue when you have confirmed your account has some SCRT in it.

#### Store the Secret Contract on Holodeck

Next upload the compiled, optimized contract to the testnet.

```bash
secretcli tx compute store contract.wasm.gz --from <your account alias> -y --gas 1000000 --gas-prices=1.0uscrt
```

The result is a transaction hash (txhash). Query it to see the `code_id` in the logs, which you'll use to create an instance of the contract.

```bash
secretcli query tx <txhash>
```

#### Instantiate your Secret Contract

To create an instance of your contract on Holodeck set the `CODE_ID` value below to the `code_id` you got by querying the txhash.

```bash
INIT='{"count": 100000000}'
CODE_ID=<code_id>
secretcli tx compute instantiate $CODE_ID "$INIT" --from <your account alias> --label "my counter" -y
```

You can use the testnet explorer [Transactions](http://explorer.secrettestnet.io/transactions) tab to view the contract instantiation.

## Secret Contracts 101

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

The `msg.rs` file is where the InitMsg parameters are specified (like a constructor), the types of Query (GetCount) and Handle[r] (Increment) messages, and any custom structs for each query response.

```rust
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
### Secret Contract code explanation

Use [this link](https://github.com/enigmampc/SecretSimpleVote/blob/master/src/contract.rs) to a see a sample voting contract and a line by line description of everything you need to know


### Unit Tests

Unit tests are coded in the `contract.rs` file itself:

```rust
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

## Secret Toolkit
[Secret Toolkit](https://github.com/enigmampc/secret-toolkit) is a collection of Rust packages that contain common tools used in development of Secret Contracts running on the Secret Network.

#### Calling other Contracts
[Secret Toolkit](https://github.com/enigmampc/secret-toolkit) contains very helpful tools that can be used to call other contracts from your own.  [Here](https://github.com/baedrik/SCRT-sealed-bid-auction/blob/master/CALLING_OTHER_CONTRACTS.md) is a guide on how to call other contracts from your own using the `InitCallback`, `HandleCallback`, and `Query` traits defined in the [utils package](https://github.com/enigmampc/secret-toolkit/tree/master/packages/utils).

If you are specifically wanting to call Handle functions or Queries of [SNIP-20 token contracts](https://github.com/enigmampc/snip20-reference-impl), there are individually named functions you can use to make it even simpler than using the generic traits.  These are located in the [SNIP-20 package](https://github.com/enigmampc/secret-toolkit/tree/master/packages/snip20).

## Secret Contracts - Advanced
Use [this link](https://github.com/baedrik/SCRT-sealed-bid-auction) for a sealed-bid (secret) auction contract that makes use of [SNIP-20](https://github.com/enigmampc/snip20-reference-impl) and a walkthrough of the contract

### Tutorials from Secret Network community
Visit [this link](https://learn.figment.io/network-documentation/secret) for all tutorials about Secret Network

### CosmWasm resources
Smart Contracts in the Secret Network based based on CosmWasm. Therefore, for troubleshooting and additional context, CosmWasm documentation may be very useful. Here are some of the links we relied on in putting together this guide:

- [cosmwasm repo](https://github.com/CosmWasm/cosmwasm)
- [cosmwasm docs](https://docs.cosmwasm.com/) 
