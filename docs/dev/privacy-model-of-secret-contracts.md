# Privacy Model of Secret Contracts

Secret Contracts are based on CosmWasm v0.10, but they have additional privacy properties that can only be found on Secret Network.

If you're a contract developer, you might want to first catch up on [developing Secret Contracts](./developing-secret-contracts.md).  
For an in depth look at the Secret Network encryption specs, visit [here](../protocol/encryption-specs.md#encryption).

Secret Contract developers must always consider the trade-off between privacy, user experience, performance and gas usage.

- [Privacy Model of Secret Contracts](#privacy-model-of-secret-contracts)
- [Verified Values During Contract Execution](#verified-values-during-contract-execution)
  - [Tx Parameter Verification](#tx-parameter-verification)
- [`Init` and `Handle`](#init-and-handle)
  - [Inputs](#inputs)
  - [State operations](#state-operations)
  - [API calls](#api-calls)
  - [Outputs](#outputs)
    - [Return value of `init`](#return-value-of-init)
    - [Return value of `handle`](#return-value-of-handle)
    - [Logs and Messages (Same for `init` and `handle`)](#logs-and-messages-same-for-init-and-handle)
    - [Errors](#errors)
      - [Contract errors](#contract-errors)
      - [Contract panic](#contract-panic)
      - [External errors (VM or interaction with the blockchain)](#external-errors-vm-or-interaction-with-the-blockchain)
- [Query](#query)
  - [Inputs](#inputs-1)
  - [API calls](#api-calls-1)
  - [Outputs](#outputs-1)
    - [Return value of `query`](#return-value-of-query)
    - [Errors](#errors-1)
      - [Contract errors](#contract-errors-1)
      - [Contract panic](#contract-panic-1)
      - [External errors (VM or interaction with the blockchain)](#external-errors-vm-or-interaction-with-the-blockchain-1)
- [External query](#external-query)
- [Data leakage attacks by analyzing metadata of contract usage](#data-leakage-attacks-by-analyzing-metadata-of-contract-usage)
  - [Differences in input sizes](#differences-in-input-sizes)
  - [Differences in state key sizes](#differences-in-state-key-sizes)
  - [Differences in state value sizes](#differences-in-state-value-sizes)
  - [Differences in state accessing order](#differences-in-state-accessing-order)
  - [Differences in output return values size](#differences-in-output-return-values-size)
  - [Differences in output messages/callbacks](#differences-in-output-messagescallbacks)
  - [Differences in output events](#differences-in-output-events)
  - [Differences in output types - success vs. error](#differences-in-output-types---success-vs-error)

# Verified Values During Contract Execution

During execution, some contracts may want to use "external-data" - meaning data that is generated outside of the enclave and sent into the enclave - such as the tx sender address, the funds sent with the tx, block height, etc..
As these parameters get sent to the enclave, they can theoretically be tampered with, and an attacker might send false data.
Thus, relying on such data might be risky.

As an example, let's say we are implementing an admin interface for a contract, i.e. functionality that is open only for a predefined address.
In that case, we want to know that the `env.message.sender` parameter that is given during contract execution is legit, then we want to check that `env.message.sender == predefined_address` and provide admin functionality if that condition is met.
If the `env.message.sender` parameter can be tampered with - we effectively can't rely on it and cannot implement the admin interface.

## Tx Parameter Verification

Some parameters are easier to verify, but for others it is less trivial to do so. Exact details about individual parameters are detailed further in this document.

The parameter verification method depends on the contract caller:

- If the contract is called by a transaction (i.e. someone sends a compute tx) we use the already-signed transaction and verify it's data inside the enclave. More specifically:
  - Verify that the signed data and the signature bytes are self consistent.
  - Verify that the parameters sent to the enclave matches with the signed data.
- If the contract is called by another contract (i.e. we don't have a signed tx to rely on) we create a callback signature (which can only be created inside the enclave), effectively signing the parameters sent to the next contract:
  - Caller contract creates `callback_signature` based on parameters it sends, passes it on to the next contract.
  - Receiver contract creates `callback_signature` based on the parameter it got.
  - Receiver contract verifies that the signature it created matches the signature it got from the caller.
  - For the specifics, visit the [encryption specs](../protocol/encryption-specs.md#Output).

# `Init` and `Handle`

`init` is the constructor of a contract. This function is called only once in the lifetime of the contract.  
`handle` is a regular execute transaction within a contract.

- They have a read and write access to the storage (state) of the contract.
- The fact that `init` or `handle` was invoked is public.
- They are metered by gas and incur fees according to the gas price of the sending node.
- Access control: Can use `env.message.sender`.

## Inputs

Inputs that are encrypted are known only to the transaction sender and to the contract.

| Input                    | Type                     | Encrypted? | Trusted? | Notes |
| ------------------------ | ------------------------ | ---------- | -------- | ----- |
| `env.block.height`       | `u64`                    | No         | No       |       |
| `env.block.time`         | `u64`                    | No         | No       |       |
| `env.block.chain_id`     | `String`                 | No         | No       |       |
| `env.message.sent_funds` | `Vec<Coin>`              | No         | No       |       |
| `env.message.sender`     | `CanonicalAddr`          | No         | Yes      |       |
| `env.contract.address`   | `CanonicalAddr`          | No         | Yes      |       |
| `env.contract_code_hash` | `String`                 | No         | Yes      |       |
| `msg`                    | `InitMsg` or `HandleMsg` | Yes        | Yes      |       |

Legend:

- `Trusted = No` means this data is easily forgeable. If an attacker wants to take its node offline and replay old inputs, they can pass a legitimate user input and false `env.block` input. Therefore, this data by itself cannot be trusted in order to reveal secrets or change the state of secrets.

## State operations

The state of the contract is only known to the contract itself.

The fact that `deps.storage.get`, `deps.storage.set` or `deps.storage.remove` were invoked from inside `init` is public.

| Operation                       | Field   | Encrypted? | Notes |
| ------------------------------- | ------- | ---------- | ----- |
| `value = deps.storage.get(key)` | `key`   | Yes        |       |
| `value = deps.storage.get(key)` | `value` | Yes        |       |
| `deps.storage.set(key,value)`   | `key`   | Yes        |       |
| `deps.storage.set(key,value)`   | `value` | Yes        |       |
| `deps.storage.remove(key)`      | `key`   | Yes        |       |

## API calls

| Operation                              | Private invocation? | Private data? | Notes                  |
| -------------------------------------- | ------------------- | ------------- | ---------------------- |
| `deps.storage.get()`                   | No                  | Yes           |                        |
| `deps.storage.set()`                   | No                  | Yes           |                        |
| `deps.storage.remove()`                | No                  | Yes           |                        |
| `deps.api.canonical_address()`         | Yes                 | Yes           |                        |
| `deps.api.human_address()`             | Yes                 | Yes           |                        |
| `deps.querier.query()`                 | No                  | Only `msg`    | Query another contract |
| `deps.querier.query_balance()`         | No                  | No            |                        |
| `deps.querier.query_all_balances()`    | No                  | No            |                        |
| `deps.querier.query_validators()`      | No                  | No            |                        |
| `deps.querier.query_bonded_denom()`    | No                  | No            |                        |
| `deps.querier.query_all_delegations()` | No                  | No            |                        |
| `deps.querier.query_delegation()`      | No                  | No            |                        |

Legend:

- `Private invocation = Yes` means the request never exits SGX and thus an attacker cannot know it even occurred.
- `Private invocation = No` & `Private data = Yes` means an attacker can know that the contract used this API but cannot know the input parameters or return values.

## Outputs

Outputs that are encrypted are only known to the transaction sender and to the contract.

### Return value of `init`

The return value of `init` is the new `contract_address`. It is not encrypted.

| Output             | Type        | Encrypted? | Notes |
| ------------------ | ----------- | ---------- | ----- |
| `contract_address` | `HumanAddr` | No         |       |

### Return value of `handle`

The return value of `handle` is called `data`. It is encrypted.

| Output | Type     | Encrypted? | Notes |
| ------ | -------- | ---------- | ----- |
| `data` | `Binary` | Yes        |       |

### Logs and Messages (Same for `init` and `handle`)

Logs (or events) is a list of key-value pair. The keys and values are encrypted, but the list structure itself is not encrypted.

| Output         | Type                   | Encrypted? | Notes                                      |
| -------------- | ---------------------- | ---------- | ------------------------------------------ |
| `log`          | `Vec<{String,String}>` | No         | Structure not encrypted, data is encrypted |
| `log[i].key`   | `String`               | Yes        |                                            |
| `log[i].value` | `String`               | Yes        |                                            |

Messages are actions that will be taken after the current execution and will all be part of the current transaction.  
Types of messages:

- `CosmosMsg::Custom`
- `CosmosMsg::Bank::Send`
- `CosmosMsg::Staking::Delegate`
- `CosmosMsg::Staking::Undelegate`
- `CosmosMsg::Staking::Withdraw`
- `CosmosMsg::Staking::Redelegate`
- `CosmosMsg::Wasm::Instantiate`
- `CosmosMsg::Wasm::Execute`

| Output        | Type                           | Encrypted? | Notes                                             |
| ------------- | ------------------------------ | ---------- | ------------------------------------------------- |
| `messages`    | `Vec<CosmosMsg>`               | No         | Structure not encrypted, data sometimes encrypted |
| `messages[i]` | `CosmosMsg::Bank`              | No         |                                                   |
| `messages[i]` | `CosmosMsg::Custom`            | No         |                                                   |
| `messages[i]` | `CosmosMsg::Staking`           | No         |                                                   |
| `messages[i]` | `CosmosMsg::Wasm::Instantiate` | No         | Only the `msg` field inside is encrypted          |
| `messages[i]` | `CosmosMsg::Wasm::Execute`     | No         | Only the `msg` field inside is encrypted          |

`Wasm` messages are additional contract calls to be invoked right after the current call.

| Type of `CosmosMsg::Wasm::*` message | Field                | Type        | Encrypted? | Notes |
| ------------------------------------ | -------------------- | ----------- | ---------- | ----- |
| `Instantiate`                        | `code_id`            | `u64`       | No         |       |
| `Instantiate`                        | `callback_code_hash` | `String`    | No         |       |
| `Instantiate`                        | `msg`                | `Binary`    | Yes        |       |
| `Instantiate`                        | `send`               | `Vec<Coin>` | No         |       |
| `Instantiate`                        | `label`              | `String`    | No         |       |
| `Execute`                            | `contract_addr`      | `HumanAddr` | No         |       |
| `Execute`                            | `callback_code_hash` | `String`    | No         |       |
| `Execute`                            | `msg`                | `Binary`    | Yes        |       |
| `Execute`                            | `send`               | `Vec<Coin>` | No         |       |

### Errors

Contract execution can result in multiple types of errors.  
The fact that the contract returned an error is public.

#### Contract errors

A contract can choose to return an `StdError`. The error message is encrypted.

Types of `StdError`:

- `GenericErr`
- `InvalidBase64`
- `InvalidUtf8`
- `NotFound`
- `NullPointer`
- `ParseErr`
- `SerializeErr`
- `Unauthorized`
- `Underflow`

#### Contract panic

If a contract receives a panic (exception) during its execution, the error message is not encrypted and will always be `Execution error: Enclave: the contract panicked`.

Contract developers should test their contracts rigorously and make sure they can never panic.

#### External errors (VM or interaction with the blockchain)

A `VMError` occurs when there's an error during the contract's execution but outside the contract's code.  
In this case the error message is not encrypted as well.

Some examples of `VMErrors`:

- Memory allocation errors (The contract tried to allocate too much)
- Contract out of gas
- Got out of gas while accessing storage
- Passing null pointers from the contract to the VM (E.g. `read_db(null)`)
- Trying to write to read-only storage (E.g. inside a `query`)
- Passing a faulty message to the blockchain (Trying to send fund you don't have, trying to callback to a non-existing contract)

# Query

`query` is an execution of a contract on the node of the query sender.

- It doesn't affect transactions on-chain.
- It has read-only access to the storage (state) of the contract.
- The fact that `query` was invoked is known only to the executing node. And to whoever monitors your internet traffic, in case the executing node is on your local machine.
- Queries are metered by gas but don't incur fees. The executing node decides its gas limit for queries.
- Access control: Cannot use `env.message.sender` as it's not a transaction. Can use pre-configured passwords or API keys that have been stored in state previously by `init` and `handle`.

## Inputs

Inputs that are encrypted and known only to the query sender and to the contract. In `query` we don't have an `env` like we do in `init` and `handle`.

| Input | Type       | Encrypted? | Trusted? | Notes |
| ----- | ---------- | ---------- | -------- | ----- |
| `msg` | `QueryMsg` | Yes        | Yes      |       |

Note that `Trusted = No` means this data is easily forgeable. An attacker can take its node offline and replay old inputs. This data that is `Trusted = No` by itself cannot be trusted in order to reveal secrets. This is more applicable to `init` and `handle`, but know that an attacker can replay the input `msg` to its offline node. Although `query` cannot change the contract's state and the attacker cannot decrypt the query output, the attacker might be able to deduce private information by monitoring output sizes at different times. See [differences in output return values size](#differences-in-output-return-values-size) to learn more about this kind of attack and how to mitigate it.

## API calls

| Operation                              | Private invocation? | Private data? | Notes                  |
| -------------------------------------- | ------------------- | ------------- | ---------------------- |
| `deps.storage.get()`                   | No                  | Yes           |                        |
| `deps.api.canonical_address()`         | Yes                 | Yes           |                        |
| `deps.api.human_address()`             | Yes                 | Yes           |                        |
| `deps.querier.query()`                 | No                  | Only `msg`    | Query another contract |
| `deps.querier.query_balance()`         | No                  | No            |                        |
| `deps.querier.query_all_balances()`    | No                  | No            |                        |
| `deps.querier.query_validators()`      | No                  | No            |                        |
| `deps.querier.query_bonded_denom()`    | No                  | No            |                        |
| `deps.querier.query_all_delegations()` | No                  | No            |                        |
| `deps.querier.query_delegation()`      | No                  | No            |                        |

Legend:

- `Private invocation = Yes` means the request never exits SGX and thus an attacker cannot know it even occurred. Only applicable if the executing node is remote.
- `Private invocation = No` & `Private data = Yes` means an attacker can know that the contract used this API but cannot know the input parameters or return values. Only applicable if the executing node is remote.

## Outputs

Outputs that are encrypted are only known to the query sender and to the contract.

### Return value of `query`

The return value of `query` is similar to `data` in `handle`. It is encrypted.

| Output | Type     | Encrypted? | Notes |
| ------ | -------- | ---------- | ----- |
| `data` | `Binary` | Yes        |       |

### Errors

Contract execution can result in multiple types of errors.  
The fact that the contract returned an error is public.

#### Contract errors

A contract can choose to return an `StdError`. The error message is encrypted.

Types of `StdError`:

- `GenericErr`
- `InvalidBase64`
- `InvalidUtf8`
- `NotFound`
- `NullPointer`
- `ParseErr`
- `SerializeErr`
- `Unauthorized`
- `Underflow`

#### Contract panic

If a contract receives a panic (exception) during its execution, the error message is not encrypted and will always be `Execution error: Enclave: the contract panicked`.

Contract developers should test their contracts rigorously and make sure they can never panic.

#### External errors (VM or interaction with the blockchain)

A `VMError` occurs when there's an error during the contract's execution but outside of the contract's code.  
In this case the error message is not encrypted as well.

Some examples of `VMErrors`:

- Memory allocation errors (The contract tried to allocate too much)
- Contract out of gas
- Got out of gas while accessing storage
- Passing null pointers from the contract to the VM (E.g. `read_db(null)`)
- Trying to write to read-only storage
- Passing a faulty message to the blockchain (Trying to send fund you don't have, trying to callback to a non-existing contract)

# External query

External `query` is an execution of a contract from another contract in the middle of its run.

- Can be called from another `init`, `handle` or `query`.
- It has read-only access to the storage (state) of the contract.
- `init` & `handle`: The fact that external `query` was invoked public.
- `query`: The fact that `query` was invoked is known only to the executing node. And to whoever monitors your internet traffic, in case the executing node is on your local machine.
- External `query` is metered by the gas limit of the caller contract.
- Access control: Cannot use `env.message.sender`, just like `query`.

Types of external `query`:

| Operation                               | Private invocation? | Private data? | Notes                  |
| --------------------------------------- | ------------------- | ------------- | ---------------------- |
| `Bank::BankQuery::Balance`              | No                  | No            |                        |
| `Bank::BankQuery::AllBalances`          | No                  | No            |                        |
| `Staking::StakingQuery::BondedDenom`    | No                  | No            |                        |
| `Staking::StakingQuery::AllDelegations` | No                  | No            |                        |
| `Staking::StakingQuery::Delegation`     | No                  | No            |                        |
| `Staking::StakingQuery::Validators`     | No                  | No            |                        |
| `Wasm::WasmQuery::Smart`                | No                  | Only `msg`    | Query another contract |

Legend:

- `Private invocation = Yes` means the request never exits SGX and thus an attacker cannot know it even occurred. Only applicable if the executing node is remote.
- `Private invocation = No` & `Private data = Yes` means an attacker can know that the contract used this API but cannot know the input parameters or return values. Only applicable if the executing node is remote.

External queries of type `WasmQuery` work exactly like [Queries](#query), except that if an external query of type `WasmQuery` is invoked from `init` or `handle` it is executed on-chain, so it is exposed to monitoring by every node in the Secret Network.

# Data leakage attacks by analyzing metadata of contract usage

Depending on the contract's implementation, an attacker might be able to de-anonymization information about the contract and its clients. Contract developers must consider all the following scenarios and more, and implement mitigations in case some of these attack vectors can compromise privacy aspects of their application.

In all the following scenarios, assume that an attacker has a local full node in its control. They cannot break into SGX, but they can tightly monitor and debug every other aspect of the node, including trying to feed old transactions directly to the contract inside SGX (replay). Also, though it's encrypted, they can also monitor memory (size), CPU (load) and disk usage (read/write timings and sizes) of the SGX chip.

For encryption, the Secret Network is using [AES-SIV](https://tools.ietf.org/html/rfc5297), which does not pad the ciphertext. This means it leaks information about the plaintext data, specifically about its size, though in most cases it's more secure than other padded encryption schemes. Read more about the encryption specs [in here](protocol/encryption-specs.md).

Most of the below examples talk about an attacker revealing which function was executed on the contract, but this is not the only type of data leakage that an attacker might target.

Secret Contract developers must analyze the privacy model of their contract - What kind of information must remain private and what kind of information, if revealed, won't affect the operation of the contract and its users. **Analyze what it is that you need to keep private and structure your Secret Contract's boundaries to protect that.**

## Differences in input sizes

An example input API for a contract with 2 `handle` functions:

```rust
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum HandleMsg {
    Send {
        amount: u8,
    },
    Transfer {
        amount: u8,
    },
}
```

This means that the inputs for transactions on this contract would look like:

1. `{"send":{"amount":123}}`
2. `{"transfer":{"amount":123}}`

These inputs are encrypted, but by looking at their size an attacker can guess which function has been called by the user.

A quick fix for this issue might be renaming `Transfer` to `Tsfr`:

```rust
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum HandleMsg {
    Send {
        amount: u8,
    },
    Tsfr {
        amount: u8,
    },
}
```

Now an attacker wouldn't be able to tell which function was called:

1. `{"send":{"amount":123}}`
2. `{"tsfr":{"amount":123}}`

Be creative. :rainbow:

Another point to consider. If the attacker had additional knowledge, for example that `send.amount` is likely smaller than `100` and `tsfr.amount` is likely bigger than `100`, then they might still guess with some probability which function was called:

1. `{"send":{"amount":55}}`
2. `{"tsfr":{"amount":123}}`

Note that a client side solution can also be applied, but this is considered a very bad practice in infosec, as you cannot guarantee control of the client. E.g. you could pad the input to the maximum possible in this contract before encrypting it on the client side:

1. `{"send":{ "amount" : 55 } }`
2. `{"transfer":{"amount":123}}`

Again, this is very not recommended as you cannot guarantee control of the client!

## Differences in state key sizes

Contracts' state is stored on-chain inside a key-value store, thus the `key` must remain constant between calls. This means that if a contract uses storage keys with different sizes, an attacker might find out information about the execution of a contract.

Let's see an example for a contract with 2 `handle` functions:

```rust
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum HandleMsg {
    Send { amount: u8 },
    Tsfr { amount: u8 },
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    _env: Env,
    msg: HandleMsg,
) -> HandleResult {
    match msg {
        HandleMsg::Send { amount } => {
            deps.storage.set(b"send", &amount.to_be_bytes());
            Ok(HandleResponse::default())
        }
        HandleMsg::Tsfr { amount } => {
            deps.storage.set(b"transfer", &amount.to_be_bytes());
            Ok(HandleResponse::default())
        }
    }
}
```

By looking at state write operation, an attacker can guess which function was called based on the size of the key that was used to write to storage:

1. `send`
2. `transfer`

Again, some quick fixes for this issue might be:

1. Renaming `transfer` to `tsfr`.
2. Padding `send` to have the same length as `transfer`: `sendsend`.

```rust
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum HandleMsg {
    Send { amount: u8 },
    Tsfr { amount: u8 },
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    _env: Env,
    msg: HandleMsg,
) -> HandleResult {
    match msg {
        HandleMsg::Send { amount } => {
            deps.storage.set(b"sendsend", &amount.to_be_bytes());
            Ok(HandleResponse::default())
        }
        HandleMsg::Tsfr { amount } => {
            deps.storage.set(b"transfer", &amount.to_be_bytes());
            Ok(HandleResponse::default())
        }
    }
}
```

Be creative. :rainbow:

## Differences in state value sizes

Very similar to the state key sizes case, if a contract uses storage values with predictably different sizes, an attacker might find out information about the execution of a contract.

Let's see an example for a contract with 2 `handle` functions:

```rust
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum HandleMsg {
    Send { amount: u8 },
    Tsfr { amount: u8 },
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    _env: Env,
    msg: HandleMsg,
) -> HandleResult {
    match msg {
        HandleMsg::Send { amount } => {
            deps.storage.set(
                b"sendsend",
                format!("Sent amount: {}", amount).as_bytes(),
            );
            Ok(HandleResponse::default())
        }
        HandleMsg::Tsfr { amount } => {
            deps.storage.set(
                b"transfer",
                format!("Transfered amount: {}", amount).as_bytes(),
            );
            Ok(HandleResponse::default())
        }
    }
}
```

By looking at state write operation, an attacker can guess which function was called based on the size of the value that was used to write to storage:

1. `Sent amount: 123`
2. `Transferred amount: 123`

Again, some quick fixes for this issue might be:

1. Changing the `Transferred` string to `Tsfr`.
2. Padding `Sent` to have the same length as `Transferred`: `SentSentSen`.

```rust
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum HandleMsg {
    Send { amount: u8 },
    Tsfr { amount: u8 },
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    _env: Env,
    msg: HandleMsg,
) -> HandleResult {
    match msg {
        HandleMsg::Send { amount } => {
            deps.storage.set(
                b"sendsend",
                format!("Sent amount: {}", amount).as_bytes(),
            );
            Ok(HandleResponse::default())
        }
        HandleMsg::Tsfr { amount } => {
            deps.storage.set(
                b"transfer",
                format!("Tsfr amount: {}", amount).as_bytes(),
            );
            Ok(HandleResponse::default())
        }
    }
}
```

Be creative. :rainbow:

## Differences in state accessing order

An attacker can monitor requests from Smart Contracts to the API that the Secret Network exposes for contracts. So while `key` and `value` are encrypted in `read_db(key)` and `write_db(key,value)`, it is public knowledge that `read_db` or `write_db` were called.

Let's see an example for a contract with 2 `handle` functions:

```rust
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum HandleMsg {
    Hey {},
    Bye {},
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    _env: Env,
    msg: HandleMsg,
) -> HandleResult {
    match msg {
        HandleMsg::Hey {} => {
            deps.storage.get(b"hi");
            deps.storage.set(b"hi", b"bye");
            deps.storage.get(b"hi");
            Ok(HandleResponse::default())
        }
        HandleMsg::Bye {} => {
            deps.storage.set(b"hi", b"bye");
            deps.storage.get(b"hi");
            deps.storage.get(b"hi");
            Ok(HandleResponse::default())
        }
    }
}
```

By looking at the order of state operation, an attacker can guess which function was called.

1. `read_db()`, `write_db()`, `read_deb()` => `Hey` was called.
2. `write_db()`, `read_db()`, `read_deb()` => `Bye` was called.

This use case might be more difficult to solve, as it is highly depends on functionality, but an example solution would be to redesign the storage accessing patterns a bit to include one big read in the start of each function and one big write in the end of each function.

```rust
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum HandleMsg {
    Hey {},
    Bye {},
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    _env: Env,
    msg: HandleMsg,
) -> HandleResult {
    match msg {
        HandleMsg::Hey {} => {
            deps.storage.get(b"data");
            deps.storage.set(b"data", b"a: hey b: bye");
            Ok(HandleResponse::default())
        }
        HandleMsg::Bye {} => {
            deps.storage.get(b"data");
            deps.storage.set(b"data", b"a: bye b: hey");
            Ok(HandleResponse::default())
        }
    }
}
```

Now by looking at the order of state operation, an attacker cannot guess which function was called. It's always `read_db()` then `write_db()`.

Note that this might affect gas usage for the worse (reading/writing data that isn't necessary to this function) or for the better (fewer reads and writes), so there's always a trade-off between privacy, user experience, performance and gas usage.

Be creative. :rainbow:

## Differences in output return values size

Secret Contracts can have return values that are decryptable only by the contract and the transaction sender.

Very similar to previous cases, if a contract uses return values with different sizes, an attacker might find out information about the execution of a contract.

Let's see an example for a contract with 2 `handle` functions:

```rust
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum HandleMsg {
    Send { amount: u8 },
    Tsfr { amount: u8 },
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    msg: HandleMsg,
) -> HandleResult {
    match msg {
        HandleMsg::Send { amount } => Ok(HandleResponse {
            messages: vec![],
            log: vec![],
            data: Some(Binary::from(amount.to_be_bytes().to_vec())),
        }),
        HandleMsg::Tsfr { amount } => Ok(HandleResponse {
            messages: vec![],
            log: vec![],
            data: Some(Binary::from(format!("amount: {}", amount).as_bytes())),
        }),
    }
}
```

By looking at the encrypted output, an attacker can guess which function was called based on the size of the return value:

1. 1 byte (uint8): `123`
2. 11 bytes (formatted string): `amount: 123`

Again, a quick fix will be to padd the shorter case to be as long as the longest case (assuming it's harder to shrink the longer case):

```rust
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum HandleMsg {
    Send { amount: u8 },
    Tsfr { amount: u8 },
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    msg: HandleMsg,
) -> HandleResult {
    match msg {
        HandleMsg::Send { amount } => Ok(HandleResponse {
            messages: vec![],
            log: vec![],
            data: Some(Binary::from(format!("padding {}", amount).as_bytes())),
        }),
        HandleMsg::Tsfr { amount } => Ok(HandleResponse {
            messages: vec![],
            log: vec![],
            data: Some(Binary::from(format!("amount: {}", amount).as_bytes())),
        }),
    }
}
```

Note that `"padding "` and `"amount: "` have the same UTF-8 size of 8 bytes.

Be creative. :rainbow:

## Differences in output messages/callbacks

Secret Contracts can output messages to be executed right after, in the same transaction as the current execution.have out that are decryptable only by the contract and the transaction sender.

Very similar to previous cases, if a contract output mesasges that are different or with different structures, an attacker might find out information about the execution of a contract.

Let's see an example for a contract with 2 `handle` functions:

```rust
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum HandleMsg {
    Send { amount: u8, to: HumanAddr },
    Tsfr { amount: u8, to: HumanAddr },
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    deps: &mut Extern<S, A, Q>,
    env: Env,
    msg: HandleMsg,
) -> HandleResult {
    match msg {
        HandleMsg::Send { amount, to } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Bank(BankMsg::Send {
                from_address: deps.api.human_address(&env.contract.address).unwrap(),
                to_address: to,
                amount: vec![Coin {
                    denom: "uscrt".into(),
                    amount: Uint128(amount.into()),
                }],
            })],
            log: vec![],
            data: None,
        }),
        HandleMsg::Tsfr { amount, to } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Staking(StakingMsg::Delegate {
                validator: to,
                amount: Coin {
                    denom: "uscrt".into(),
                    amount: Uint128(amount.into()),
                },
            })],
            log: vec![],
            data: None,
        }),
    }
}
```

Those outputs are plaintext as they are fowarded to the Secret Network for processing. By looking at these two outputs, an attacker will know which function was called based on the type of messages - `BankMsg::Send` vs. `StakingMsg::Delegate`.

Some messages are partially encrypted, like `Wasm::Instantiate` and `Wasm::Execute`, but only the `msg` field is encrypted, so differences in `contract_addr`, `callback_code_hash`, `send` can reveal unintended data, as well as the size of `msg` which is encrypted but can reveal data the same way as previos examples.

```rust
#[derive(Serialize, Deserialize, Clone, Debug, PartialEq, JsonSchema)]
#[serde(rename_all = "snake_case")]
pub enum HandleMsg {
    Send { amount: u8 },
    Tsfr { amount: u8 },
}

pub fn handle<S: Storage, A: Api, Q: Querier>(
    _deps: &mut Extern<S, A, Q>,
    _env: Env,
    msg: HandleMsg,
) -> HandleResult {
    match msg {
        HandleMsg::Send { amount } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
/*plaintext->*/ contract_addr: "secret108j9v845gxdtfeu95qgg42ch4rwlj6vlkkaety".into(),
/*plaintext->*/ callback_code_hash:
                     "cd372fb85148700fa88095e3492d3f9f5beb43e555e5ff26d95f5a6adc36f8e6".into(),
/*encrypted->*/ msg: Binary(
                     format!(r#"{{\"aaa\":{}}}"#, amount)
                         .to_string()
                         .as_bytes()
                         .to_vec(),
                 ),
/*plaintext->*/ send: Vec::default(),
            })],
            log: vec![],
            data: None,
        }),
        HandleMsg::Tsfr { amount } => Ok(HandleResponse {
            messages: vec![CosmosMsg::Wasm(WasmMsg::Execute {
/*plaintext->*/ contract_addr: "secret1suct80ctmt6m9kqmyafjl7ysyenavkmm0z9ca8".into(),
/*plaintext->*/ callback_code_hash:
                    "e67e72111b363d80c8124d28193926000980e1211c7986cacbd26aacc5528d48".into(),
/*encrypted->*/ msg: Binary(
                    format!(r#"{{\"bbb\":{}}}"#, amount)
                        .to_string()
                        .as_bytes()
                        .to_vec(),
                ),
/*plaintext->*/ send: Vec::default(),
            })],
            log: vec![],
            data: None,
        }),
    }
}
```

More scenarios to be mindful of:

- Ordering of messages (E.g. `Bank` and then `Staking` vs. `Staking` and `Bank`)
- Size of the encrypted `msg` field
- Number of messages (E.g. 3 `Execute` vs. 2 `Execute`)

Again, be creative if that's affecting your secrets. :rainbow:

## Differences in output events

Output events:

- "Push notifications" for GUIs with SecretJS
- To make the tx searchable on-chain

Examples:

- number of logs
- size of logs
- ordering of logs (short,long vs. long,short)

## Differences in output types - success vs. error

If a contract returns an `StdError`, the output looks like this:

```json
{
  "Error": "<encrypted>"
}
```

Otherwise the output looks like this:

```json
{
  "Ok": "<encrypted>"
}
```

Therefore similar to previous examples, an attacker might guess what happned in an execution. E.g. if a contract have only a `send` function, if an error was returned an attacker can know that the `msg.sender` tried to send funds to someone unknown and the `send` didn't went through.
