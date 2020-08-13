# The privacy model of Secret Contracts

Secret Contracts are based on CosmWasm v0.10, but have additional privacy properties that can only be found in the Secret Network.

If you're a contract developer, you might want to first catch up on [developing Secret Contracts](developing-secret-contracts.md).  
For an in depth look at the Secret Network encryption specs, visit [here](protocol/encryption-specs.md).

Secret Contract developers must always consider the trade-off between privacy, user experience, performance and gas usage.

- [The privacy model of Secret Contracts](#the-privacy-model-of-secret-contracts)
- [Init](#init)
  - [Inputs to `init`](#inputs-to-init)
  - [State operations from `init`](#state-operations-from-init)
  - [API calls from `init`](#api-calls-from-init)
  - [Outputs](#outputs)
- [Handle](#handle)
- [Query](#query)
- [Inputs](#inputs)
  - [Encrypted](#encrypted)
  - [Not encrypted](#not-encrypted)
  - [What inputs can be trusted](#what-inputs-can-be-trusted)
  - [What inputs cannot be trusted](#what-inputs-cannot-be-trusted)
- [State](#state)
- [External query](#external-query)
  - [Encrypted](#encrypted-1)
  - [Not encrypted](#not-encrypted-1)
- [Outputs](#outputs-1)
  - [Encrypted](#encrypted-2)
  - [Not encrypted](#not-encrypted-2)
- [Data leakage attacks by detecting patterns in contract usage](#data-leakage-attacks-by-detecting-patterns-in-contract-usage)
  - [Differences in input sizes](#differences-in-input-sizes)
  - [Differences in state key sizes](#differences-in-state-key-sizes)
  - [Differences in state value sizes](#differences-in-state-value-sizes)
  - [Differences in state accessing order](#differences-in-state-accessing-order)
  - [Differences in output return values size](#differences-in-output-return-values-size)
  - [Differences in the amounts of output messages/callbacks](#differences-in-the-amounts-of-output-messagescallbacks)
  - [Differences in sizes of output messages/callbacks](#differences-in-sizes-of-output-messagescallbacks)
  - [Differences in the orders of output messages/callbacks](#differences-in-the-orders-of-output-messagescallbacks)
  - [Differences in the amounts of output logs/events](#differences-in-the-amounts-of-output-logsevents)
  - [Differences in sizes of output logs/events](#differences-in-sizes-of-output-logsevents)
  - [Differences in the orders of output logs/events](#differences-in-the-orders-of-output-logsevents)

# Init

`init` is the constructor of your contract. This function is called only once in the lifetime of the contract.

The fact that `init` was invoked is public.

## Inputs to `init`

Inputs that are encrypted are only known to the tx sender and to the contract.

| Input                    | Type            | Encrypted? | Trusted? | Notes |
| ------------------------ | --------------- | ---------- | -------- | ----- |
| `env.block.height`       | `u64`           | No         | No       |       |
| `env.block.time`         | `u64`           | No         | No       |       |
| `env.block.chain_id`     | `String`        | No         | No       |       |
| `env.message.sender`     | `CanonicalAddr` | No         | Yes      |       |
| `env.message.sent_funds` | `Vec<Coin>`     | No         | Yes      |       |
| `env.contract.address`   | `CanonicalAddr` | No         | Yes      |       |
| `env.contract_code_hash` | `String`        | No         | Yes      |       |
| `msg`                    | `InitMsg`       | Yes        | Yes      |       |

## State operations from `init`

The state of the contract is only known to the contract itself.

The fact that `deps.storage.get`, `deps.storage.set` or `deps.storage.remove` were invoked from inside `init` is public.

| Operation                       | Field   | Encrypted? | Notes |
| ------------------------------- | ------- | ---------- | ----- |
| `value = deps.storage.get(key)` | `key`   | Yes        |       |
| `value = deps.storage.get(key)` | `value` | Yes        |       |
| `deps.storage.set(key,value)`   | `key`   | Yes        |       |
| `deps.storage.set(key,value)`   | `value` | Yes        |       |
| `deps.storage.remove(key)`      | `key`   | Yes        |       |

## API calls from `init`

| Operation                              | Private invokation? | Private data? | Notes                  |
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

- `Private invokation = Yes` means the request never exits SGX and thus an attacker cannot know it even occured.
- `Private invokation = No` & `Private data = Yes` means an attacker can know that the contract used this API but cannot know the input parameters or return values.

## Outputs

Outputs that are encrypted are only known to the tx sender and to the contract.

`contract_address` is the "return value" of `init`, and is public.

| Output             | Type        | Encrypted? | Notes                                         |
| ------------------ | ----------- | ---------- | --------------------------------------------- |
| `contract_address` | `HumanAddr` | No         | The "return value", not visible inside `init` |

Logs (or events) is a list of key-value pair. The keys and values are encrypted, but the list struture itself is not encrypted.

| Output         | Type                   | Encrypted? | Notes                                      |
| -------------- | ---------------------- | ---------- | ------------------------------------------ |
| `log`          | `Vec<{String,String}>` | No         | Structure not encrypted, data is encrypted |
| `log[i].key`   | `String`               | Yes        |                                            |
| `log[i].value` | `String`               | Yes        |                                            |

Messages are actions that will be taken after this contract call and will all be commited inside the current transaction. Types of messages:

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
| `messages`    | `Vec<CosmosMsg>>`              | No         | Structure not encrypted, data sometimes encrypted |
| `messages[i]` | `CosmosMsg::Bank`              | No         |                                                   |
| `messages[i]` | `CosmosMsg::Custom`            | No         |                                                   |
| `messages[i]` | `CosmosMsg::Staking`           | No         |                                                   |
| `messages[i]` | `CosmosMsg::Wasm::Instantiate` | No         | Only the `msg` field is encrypted inside          |
| `messages[i]` | `CosmosMsg::Wasm::Execute`     | No         | Only the `msg` field is encrypted inside          |

`Wasm` messages are additional contract calls to be invoked right after the current call is done.

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

# Handle

# Query

- Read-only access to state
- Metered by gas, but does not incurs fees
- Can be used to implement getters and decide what each user can see
- No msg.sender
- Access control: use passwords instead of checking for msg.sender
- Passwords can be generated for users via `Init` or `Handle`.

# Inputs

## Encrypted

- `msg` - Only known to the tx sender and the contract

## Not encrypted

- `msg.sender`
- funds sent

## What inputs can be trusted

- tx sender
- funds sent
- `msg`

## What inputs cannot be trusted

- `block.height`

# State

- Only known to that specific contract

# External query

## Encrypted

## Not encrypted

# Outputs

## Encrypted

- Only known to the tx sender and the contract

## Not encrypted

# Data leakage attacks by detecting patterns in contract usage

Depending on the contract's implementation, an attacker might be able to de-anonymization information about the contract and its clients. Contract developers need to consider all the following scenarios and more, and implement mitigation in case that some of these attack vectors can worsen the privacy aspect of their app.

In all the following scenarios, assume that an attacker has a local full node in its control. They cannot break into SGX, but they can tightly monitor and debug every other aspect of the node, including trying to feed old transactions directly to the contract inside SGX (replay). Also, though it's encrypted, they can also monitor memory (size), CPU (load) and disk usage (read/write timings and sizes) of the SGX chip.

For encryption, the Secret Network is using (AES-SIV)[https://tools.ietf.org/html/rfc5297], which does not pad the ciphertext. This means it leaks information about the plaintext data, specifically what is its size, though in most cases it's more secure than other padded encryption schemes. Read more about the encryption specs [in here](protocol/encryption-specs.md).

Most of the below examples talk about an attacker revealing which function was executed on the contract, but this is not the only type of data leakage that an attacker might target.

Secret Contract developers must analyze the privacy model of their contract - What kind of information must remain private and what kind of information, if revealed, won't affect the operation of the contract and its users.

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

This means that the inputs for txs on this contract would look like:

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

An attacker can monitor requests from Smart Contracts to the API that the Secret Network exposes for Contracts. So while `key` and `value` are encrypted in `read_db(key)` and `write_db(key,value)`, it is public knowledge that `read_db` or `write_db` were called.

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

## Differences in the amounts of output messages/callbacks

## Differences in sizes of output messages/callbacks

## Differences in the orders of output messages/callbacks

## Differences in the amounts of output logs/events

## Differences in sizes of output logs/events

## Differences in the orders of output logs/events
