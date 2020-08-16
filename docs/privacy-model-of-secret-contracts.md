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
- [Errors](#errors)
  - [Contract errors](#contract-errors)
  - [VM errors](#vm-errors)
- [Data leakage attacks by detecting patterns in contract usage](#data-leakage-attacks-by-detecting-patterns-in-contract-usage)
  - [Differences in input sizes](#differences-in-input-sizes)
  - [Differences in state key sizes](#differences-in-state-key-sizes)
  - [Differences in state value sizes](#differences-in-state-value-sizes)
  - [Differences in state accessing order](#differences-in-state-accessing-order)
  - [Differences in output return values size](#differences-in-output-return-values-size)
  - [Differences in output messages/callbacks](#differences-in-output-messagescallbacks)
  - [Differences in the amounts of output logs/events](#differences-in-the-amounts-of-output-logsevents)
  - [Differences in output types - ok vs. error](#differences-in-output-types---ok-vs-error)

# Init

`init` is the constructor of your contract. This function is called only once in the lifetime of the contract.

The fact that `init` was invoked is public.

## Inputs to `init`

Inputs that are encrypted are only known to the transaction sender and to the contract.

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

Legend:

- `Trusted = No` means this data is easily forgeable. If an attacker wants to take its node off-chain and replay old inputs, they can pass a legitimate user input and false `env.block` input. Therefore, this data by itself cannot be trusted in order to reveal secret or change the state of secrets.

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

Messages are actions that will be taken after this contract call and will all be committed inside the current transaction. Types of messages:

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

- `msg` - Only known to the transaction sender and the contract

## Not encrypted

- `msg.sender`
- funds sent

## What inputs can be trusted

- transaction sender
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

- Only known to the transaction sender and the contract

## Not encrypted

# Errors

## Contract errors

- Encrypted

## VM errors

- Not Encrypted

# Data leakage attacks by detecting patterns in contract usage

Depending on the contract's implementation, an attacker might be able to de-anonymization information about the contract and its clients. Contract developers must consider all the following scenarios and more, and implement mitigations in case some of these attack vectors can compromise privacy aspects of their application.

In all the following scenarios, assume that an attacker has a local full node in its control. They cannot break into SGX, but they can tightly monitor and debug every other aspect of the node, including trying to feed old transactions directly to the contract inside SGX (replay). Also, though it's encrypted, they can also monitor memory (size), CPU (load) and disk usage (read/write timings and sizes) of the SGX chip.

For encryption, the Secret Network is using [AES-SIV](https://tools.ietf.org/html/rfc5297), which does not pad the ciphertext. This means it leaks information about the plaintext data, specifically what is its size, though in most cases it's more secure than other padded encryption schemes. Read more about the encryption specs [in here](protocol/encryption-specs.md).

Most of the below examples talk about an attacker revealing which function was executed on the contract, but this is not the only type of data leakage that an attacker might target.

Secret Contract developers must analyze the privacy model of their contract - What kind of information must remain private and what kind of information, if revealed, won't affect the operation of the contract and its users. **Analyze what it is that you need to keep private and structure your Secret Contract's boundries to protect that.**

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

Those outputs are plaintext as they are fowarded to the Secret Network for processing. By looking at these two outputs, an attacker will know which function was called based on the type of messages - `BankMsg::Send` vs `StakingMsg::Delegate`.

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
- Number of messages (E.g. 3 `Execute` vs 2 `Execute`)

Again, be creative if that's affecting your secrets. :rainbow:

## Differences in the amounts of output logs/events

- number of logs
- size of logs
- ordering of logs (short,long vs. long,short)

## Differences in output types - ok vs. error
