# Secret Network Randomness Feature Documentation For Secret Contracts

## Introduction
Secret Network is a privacy-preserving blockchain platform that enables secure and private transactions and computations using secret contracts. The new randomness feature allows developers to access random numbers in their CosmWasm contracts, enhancing the capabilities of the platform.

## Background
The randomness feature is provided by the Secret Network and is accessible within Secret Contracts through the Env struct. It includes an optional random field, which contains a random number as a Binary type. The random field is only available when the "random" feature is enabled.

## Use Cases
Randomness is essential in many applications, including:

* Gaming and gambling platforms, where fair and unpredictable outcomes are crucial
* Cryptographic systems that require secure random keys or nonces
* Randomized algorithms for various use cases, such as distributed systems or optimization problems

## Getting Started

To use the randomness feature in your Secret contract, follow these steps:

### Enable the "random" feature
In your Cargo.toml file, add the "random" feature to the cosmwasm-std dependency:


```toml
[dependencies.cosmwasm-std]
git = "https://github.com/scrtlabs/cosmwasm"
rev = "8ee395ba033c392d7170c971df97f085edaed2d9"
package = "secret-cosmwasm-std"
features = ["random"]
```
*Note this will change as the feature gets merged into the main-line branch*

### Consume the random number
In your contract, import the necessary dependencies:


```rust
use cosmwasm_std::{Binary, Env, MessageInfo, Response, Result};
```

In the contract's entry point (e.g., execute, instantiate, or query), you can access the random number from the env parameter:


```rust
pub fn execute(env: Env, _info: MessageInfo, _msg: T) -> Result<Response<T>> {
    if let Some(random) = env.block.random {
        // Use the random number
    }

    // Your contract logic
}
```

The env and block_info structures are defined as:

```rust
pub struct Env {
    pub block: BlockInfo,
    pub contract: ContractInfo,
    pub transaction: Option<TransactionInfo>,
}

pub struct BlockInfo {
    /// The height of a block is the number of blocks preceding it in the blockchain.
    pub height: u64,
    pub time: Timestamp,
    pub chain_id: String,
    #[cfg(feature = "random")]
    #[serde(skip_serializing_if = "Option::is_none")]
    pub random: Option<Binary>,
}
```

Where the random is 32 bytes long and base64 encoded.

## Example: Lottery Contract
Below is a simple lottery contract that uses the randomness feature:

```rust 
use cosmwasm_std::{
    Binary, Env, MessageInfo, Response, Result, Addr, Deps, DepsMut, ContractResult, Timestamp,
};

pub fn instantiate(
    deps: DepsMut,
    env: Env,
    info: MessageInfo,
    msg: InstantiateMsg,
) -> Result<Response> {
    // Your contract initialization logic
}

pub fn execute(
    deps: DepsMut,
    env: Env,
    info: MessageInfo,
    msg: ExecuteMsg,
) -> Result<Response> {
    match msg {
        ExecuteMsg::Participate { .. } => participate(deps, env, info),
        ExecuteMsg::DrawWinner => draw_winner(deps, env, info),
    }
}

fn participate(
    deps: DepsMut,
    env: Env,
    info: MessageInfo,
) -> Result<Response> {
    // Your logic for participating in the lottery
}

fn draw_winner(
    deps: DepsMut,
    env: Env,
    info: MessageInfo,
) -> Result<Response> {
    // Check if a random number is available
    let random_number = match env.block.random {
        Some(random) => random,
        None => return Err(StdError::generic_err("Randomness not available")),
    };

    // Your logic for drawing a winner using the random_number

    // Return a response
    Ok(Response::new().add_attribute("winner", winner))
}
```

In this example, we have a simple lottery contract with two actions: Participate and DrawWinner. Users can participate in the lottery, and when the DrawWinner action is called, a random number is generated using the random field from the Env struct. The contract uses this random number to select a winner fairly and unpredictably.

Note that this example is simplified for illustrative purposes and may not cover all necessary aspects of a real-world lottery contract, such as handling funds or preventing unauthorized access to certain actions.

## Using Randomness with LocalSecret

LocalSecret is a tool that allows you to run a local Secret Network on your machine for testing and development purposes. To use the new randomness feature with LocalSecret, you can leverage the localsecret:v1.9.0-beta.1-random Docker image, which supports the "random" feature for CosmWasm contracts.

Here are the steps to use the randomness feature with LocalSecret:

### Run LocalSecret with Docker

To run the local Secret Network using the Docker image, execute the following command:

```bash
docker run -it --rm -p 26657:26657 -p 26656:26656 -p 1317:1317 --name secretdev ghcr.io/scrtlabs/localsecret:v1.9.0-beta.1-random
```

This command will start the local Secret Network using the localsecret:v1.9.0-beta.1-random Docker image, which has the randomness feature enabled by default.

### Deploy and interact with your Secret Contract
You can now deploy your Secret Contract that uses the randomness feature to your local Secret Network. Make sure you have enabled the "random" feature in your contract's Cargo.toml file, as mentioned in the previous sections of this document.
Use the Secret Network tooling, such as secretcli or secretjs, to deploy and interact with your contract on the local network.
