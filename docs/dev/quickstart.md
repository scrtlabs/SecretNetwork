# Secret Contract Dev Guide

Get up and running on Secret Network testnet (holodeck) to start working with Secret Contracts.

Secret Contracts are written in Rust and based on CosmWasm. The module is referred to as `compute` in the Secret Network

Learn more about the [privacy model](https://github.com/SecretFoundation/SecretWebsite/blob/master/content/developers/secret-contract-devs/privacy-model-of-secret-contracts.md) of Secret Contracts.

## Setup Light Client

*   install [secretcli](https://github.com/enigmampc/SecretNetwork/blob/master/docs/testnet/install_cli.md)
    
*   use [version 1.0.0](https://github.com/chainofsecrets/SecretNetwork/releases/tag/v1.0.0) for testnet
    
*   configure secretcli
    
    ```
    secretcli config node http://bootstrap.secrettestnet.io:26657
    
    secretcli config chain-id holodeck-2
    
    secretcli config trust-node true
    
    ```

## Setup Dev Environment

Secret contracts are based on [CosmWasm 0.10](https://www.cosmwasm.com) ([docs](https://docs.cosmwasm.com)) which is the de facto standard for smart contracts in the Cosmos blockchain ecosystem. Secret Network implements a `compute` module used to store, query and instantiate secret contracts for decentralized, secure computation. These privacy-preserving smart contracts run inside secure enclaves or Trusted Execution Environments (TEEs), in which encrypted contract data (inputs & state) is processsed. Once stored on the blockchain, a contract has to be created (or instantiated) in order to execute its methods. If you're familiar with Solidity, you can think of this like migrating solidity code using Truffle, which handles the deployment of smart contracts on Ethereum.

CosmWasm is kind of like the EVM in Ethereum; however, CosmWasm enables multi-chain smart contracts using the [Inter-Blockchain Communication Protocol](https://cosmos.network/ibc) (IBC). For now, Secret Contracts (and other CosmWasm-based smart contracts) are written in the Rust programming language.

Next, we will show you how to get started with Rust, if you haven’t already. The Rust programming language is reliable, performant, and it has a wonderful community!

*   install Rust
*   install the Rust dependencies
*   create your first project

The Rust dependencies include the Rust compiler, cargo (_package manager_), toolchain and a package to generate projects (you can check out the Rust book, rustlings course, examples and more at [Rust-Lang.org](https://www.rust-lang.org/learn).

### 1.  Install Rust:

More information about installing Rust can be found here:
https://www.rust-lang.org/tools/install

```
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source $HOME/.cargo/env

```

### 2.  Add rustup target wasm32 for both stable and nightly:

```
rustup default stable
rustup target list --installed
rustup target add wasm32-unknown-unknown

rustup install nightly
rustup target add wasm32-unknown-unknown --toolchain nightly

```

### 3.  If using linux, install the standard build tools:

```
apt install build-essential

```

### 4.  Run cargo install cargo-generate:

Cargo generate is the tool you'll use to create a smart contract project:
https://doc.rust-lang.org/cargo)

```
cargo install cargo-generate --features vendored-openssl

```

## Tools for Secret Contract Devs

### Testnet Explorer

https://explorer.secrettestnet.io

### Testnet Faucet

https://faucet.secrettestnet.io

### Secret API

https://secretapi.io ([more info](https://blog.scrt.network/secret-api))


## Create Initial Secret Contract

To create the smart contract you'll:

* Generate Your Project
* Compile a Secret Contract
* Deploy on Testnet
* Instantiate With Parameters

### 1. Generate

```
cargo generate --git https://github.com/enigmampc/secret-template --name mysimplecounter

```

The git project above is a CosmWasm smart contract template that implements a simple counter. The contract is created with a parameter for the initial count and allows subsequent incrementing.

Change directory to the project you created and view the structure and files that were created:

```
cd mysimplecounter

```

The generate command creates a directory with the project name with the following structure:

```
Cargo.lock	Developing.md	LICENSE		Publishing.md	examples	schema		tests
Cargo.toml	Importing.md	NOTICE		README.md	rustfmt.toml	src

```

### 2. Compile

Use the following command to compile the Secret Contract, producing the wasm contract file. The Makefile uses wasm-opt, a WebAssembly optimizer.

```
npm i -g wasm-opt

make
```

### 3. Deploy

Upload the optimized contract.wasm to _holodeck_ :

```
secretcli tx compute store contract.wasm.gz --from <your account alias> -y --gas 1000000 --gas-prices=1.0uscrt

```

The result is a txhash ~ if you query it, you can see the code_id in the logs. In our case, it's 45. We need the code_id to instantiate the contract:

```
secretcli q tx 86FCA39283F0BD80A1BE42288506C47041BE2FEE5F6DB13F4652CD5594B5D875

```
### Verify Storage

The following command lists any secret contract code:

`secretcli query compute list-code`

```
[
{
"id": 45,
"creator": "secret1ddhvtztgr9kmtg2sr5gjmz60rhnqv8vwm5wjuh",
"data_hash": "E15E697E5EB2144C1BF697F1127EDF1C4322004DA7F032209D2D445BCAE46FE0",
"source": "",
"builder": ""
}
]
```


### 4. Instantiate

To create an instance of this project we must also provide some JSON input data, including a starting count. You should change the label to be something unique, which can be referenced by label instead of contract address for convenience:


```
INIT"{\"count\": 100000000}"
CODE_ID93
secretcli tx compute instantiate $CODE_ID "$INIT" --from <your account alias> --label "my counter" -y

```

With the contract now initialized, we can find its address

```
secretcli query compute list-contract-by-code $CODE_ID

```

Our instance is `secret1htxt8p8lu0v53ydsdguk9pc0x9gg060k7x4qxr`

### Usage

Query the contract state:

```
CONTRACTsecret1tss72nzwqzverru7fy5s49czqepmvdgwdz3gcx

secretcli query compute query $CONTRACT "{\"get_count\": {}}"

```

Increment the counter:

```
secretcli tx compute execute $CONTRACT "{\"increment\": {}}" --from <your account alias>

```

## Secret Contracts

###  Project Structure

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

The `msg.rs` file is where the InitMsg parameters are specified (like a constructor), the types of Query (GetCount) and Handle\[r\] (Increment) messages, and any custom structs for each query response.

```
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
pub struct InitMsg {
    pub count: i32,
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all  "lowercase")]
pub enum HandleMsg {
    Increment {},
    Reset { count: i32 },
}

#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all  "lowercase")]
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

###  Unit Tests

Unit tests are coded in the `contract.rs` file itself:

```
#[cfg(test)]
mod tests {
    use super::*;
    use cosmwasm::errors::Error;
    use cosmwasm::mock::{dependencies, mock_env};
    use cosmwasm::serde::from_slice;
    use cosmwasm::types::coin;

    #[test]
    fn proper_initialization() {
        let mut deps  dependencies(20);

        let msg  InitMsg { count: 17 };
        let env  mock_env(&deps.api, "creator", &coin("1000", "earth"), &[]);

        // we can just call .unwrap() to assert this was a success
        let res  init(&mut deps, env, msg).unwrap();
        assert_eq!(0, res.messages.len());

        // it worked, let's query the state
        let res  query(&deps, QueryMsg::GetCount {}).unwrap();
        let value: CountResponse  from_slice(&res).unwrap();
        assert_eq!(17, value.count);
    }

```

## Resources

Smart Contracts in the Secret Network use cosmwasm. Therefore, for troubleshooting and additional context, cosmwasm documentation may be very useful. Here are some of the links we relied on in putting together this guide:

*   [cosmwasm repo](https://github.com/CosmWasm/cosmwasm)
*   [cosmwasm starter pack - project template](https://github.com/CosmWasm/cosmwasm-template)
*   [Setting up a local "testnet"](https://www.cosmwasm.com/docs/getting-started/using-the-sdk)
*   [cosmwasm docs](https://www.cosmwasm.com/docs/intro/overview)
