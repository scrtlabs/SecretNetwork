# The privacy model of Secret Contracts

- [The privacy model of Secret Contracts](#the-privacy-model-of-secret-contracts)
  - [Init](#init)
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
  - [Outputs](#outputs)
    - [Encrypted](#encrypted-2)
    - [Not encrypted](#not-encrypted-2)
  - [De-anonymization attack by patterns of Contract usage](#de-anonymization-attack-by-patterns-of-contract-usage)
    - [The input size](#the-input-size)
    - [State read size](#state-read-size)
    - [State write size](#state-write-size)
    - [State access order](#state-access-order)
    - [Output data field size](#output-data-field-size)
    - [Number of output messages/callbacks](#number-of-output-messagescallbacks)
    - [The size of output messages/callbacks](#the-size-of-output-messagescallbacks)
    - [The order of output messages/callbacks](#the-order-of-output-messagescallbacks)
    - [Number of output logs/events](#number-of-output-logsevents)
    - [The size of output logs/events](#the-size-of-output-logsevents)
    - [The order of output logs/events](#the-order-of-output-logsevents)

## Init

## Handle

## Query

- Read-only access to state
- Metered by gas, but does not incure fees
- Can be used to implement getters and decide what each user can see
- No msg.sender
- Access control: use passwords instead of checking for msg.sender
- Passwords can be generated for users via `Init` or `Handle`.

## Inputs

### Encrypted

- msg

### Not encrypted

- msg.sender
- funds sent

### What inputs can be trusted

- tx sender
- funds sent
- msg

### What inputs cannot be trusted

- Block height

## State

## External query

### Encrypted

### Not encrypted

## Outputs

### Encrypted

### Not encrypted

## De-anonymization attack by patterns of Contract usage

Depending on the contract's implementation, an attacker might be able to de-anonymization information about the contract and its clients. Contract developers need to consider all of the following scenarios and more, and implement mitigation in case that some of these attack vectors can worsen the privacy aspect of their app.

In all of the following scenarios, assume that an attacker has a local full node in its control. They cannot break into SGX, but they can thightly motinor and debug every other aspect of the node, including trying to feed old transactions directly to the contract inside SGX (replay). Also, though it's encrypted, they can also monitor memory (size), CPU (load) and disk usage (read/write timings and sizes) of the SGX chip.

For encryption, the Secret Network is using (AES-SIV)[https://tools.ietf.org/html/rfc5297], which does not pad the ciphertext. This means it leaks information about the plaintext data, specifically what is its size, though in most aspects it's more secure than other padded encryption schemes. Read more about the encryption specs [in here](protocol/encryption-specs.md).

### The input size

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

This means the the inputs for txs on this contract would look like:

1. `{"send":{"amount":123}}`
2. `{"transfer":{"amount":123}}`

These^ inputs are encrypted, but by looking at their size an attacker can guess which function has been called by the user.

A quick fix for this issue can be renamig `Transfer` to `Tsfr`:

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

Altough if the attacker would know for example that `send.amount` is usually smaller than `100` and `tsfr.amount` is likley bigger than `100`, then they could still guess with some probability which function was called:

1. `{"send":{"amount":55}}`
2. `{"tsfr":{"amount":123}}`

Note that a client side solution can also be appled, but this is considered a very bad practice in infosec, as you cannot guarantee control of the client. E.g. you could pad the input to the maximum possible in this contract before encrypting it on the client side:

1. `{"send":{"amount":55} }`
2. `{"transfer":{"amount":123}}`

Again, this is very not recomended!

### State read size

### State write size

### State access order

### Output data field size

### Number of output messages/callbacks

### The size of output messages/callbacks

### The order of output messages/callbacks

### Number of output logs/events

### The size of output logs/events

### The order of output logs/events

```

```
