# Developing Secret Contracts

Secret Contacts are based on CosmWasm v0.10.

- [Developing Secret Contracts](#developing-secret-contracts)
- [IDEs](#ides)
- [Personal Secret Network for Secret Contract development](#personal-secret-network-for-secret-contract-development)
- [Init](#init)
- [Handle](#handle)
- [Query](#query)
- [Inputs](#inputs)
- [APIs](#apis)
- [State](#state)
- [Some libraries/crates considerations](#some-librariescrates-considerations)
- [Randomness](#randomness)
  - [Roll your own](#roll-your-own)
    - [Poker deck shuffling example](#poker-deck-shuffling-example)
  - [Use an external oracle](#use-an-external-oracle)
- [Outputs](#outputs)
- [External query](#external-query)
- [Compiling](#compiling)
- [Storing and deploying](#storing-and-deploying)
- [Making your contract's code verified on exporers](#making-your-contracts-code-verified-on-exporers)
- [Testing](#testing)
- [Debugging](#debugging)
- [Building secret apps with SecretJS](#building-secret-apps-with-secretjs)
  - [Wallet integration](#wallet-integration)
- [Differences from CosmWasm](#differences-from-cosmwasm)

# IDEs

Secret Contracts are developed with the [Rust](https://www.rust-lang.org/) programming language and compiled to [WASM](https://webassembly.org/) binaries.

These IDEs are known to work very well for developing Secret Contracts:

- [CLion](https://www.jetbrains.com/clion/)
- [VSCode](https://code.visualstudio.com/) with the [rust-analyzer](https://rust-analyzer.github.io/) extension

# Personal Secret Network for Secret Contract development

TODO docker example

# Init

`init` is the constructor of your contract. This function is called only once in the lifetime of the contract.

Example Invocation from `secretcli`:

```bash
secretcli tx compute instantiate "$CODE_ID" "$INPUT_MSG" --label "$UNIQUE_LABEL" --from "$MY_KEY"
```

Example Invocation from `SecretJS`:

```js
// TODO
```

# Handle

`handle` is the implementation of execute transactions.

Example Invocation from `secretcli`:

```bash
secretcli tx compute execute "$CONTRACT_ADDRESS" "$INPUT_ARGS" --from "$MY_KEY" # Option A
secretcli tx compute execute --label "$LABEL" "$INPUT_ARGS" --from "$MY_KEY"    # Option B
```

Example Invocation from `SecretJS`:

```js
// TODO
```

# Query

`query` is the implementation of read-only queries. Queries run over the current blockchain state but don't incur fees and don't have access to `msg.sender`. They are still metered by a gas limit that is set on the executing node.

Example Invocation from `secretcli`:

```bash
secretcli q compute query "$CONTRACT_ADDRESS" "$INPUT_ARGS"
```

Example Invocation from `SecretJS`:

```js
// TODO
```

# Inputs

# APIs

# State

# Some libraries/crates considerations

- `bincode2` instead of `bincode` for serializing data.
- `serde_json_wasm` instead of `serde_json` for serializing data.
- `bech32` instead of `deps.api.canonical_address` and `deps.api.human_address`, as they only support `secret` prefix (E.g. not `secretvaloper` for staking use-cases).

# Randomness

## Roll your own

### Poker deck shuffling example

1. When joining a room, [each player sends a secret number](https://github.com/enigmampc/SecretHoldEm/blob/4f67c469bb4a0f53522c7ad069e54ae5c1effb6b/contract/src/contract.rs#L172).
2. Once the room is full, all secrets are combined with sha256 to [create a random seed](https://github.com/enigmampc/SecretHoldEm/blob/4f67c469bb4a0f53522c7ad069e54ae5c1effb6b/contract/src/contract.rs#L349-L355).
3. With that seed, the [deck is shuffled](https://github.com/enigmampc/SecretHoldEm/blob/4f67c469bb4a0f53522c7ad069e54ae5c1effb6b/contract/src/contract.rs#L356-L357).
4. Each round a [game counter is incremented](https://github.com/enigmampc/SecretHoldEm/blob/4f67c469bb4a0f53522c7ad069e54ae5c1effb6b/contract/src/contract.rs#L602-L614), and along with the players' secrets is used to create a new seed for re-shuffling the deck.
5. On the frondend side, [SecretJS is used to generate a secure random number](https://github.com/enigmampc/SecretHoldEm/blob/4f67c469bb4a0f53522c7ad069e54ae5c1effb6b/gui/src/App.js#L334-L354) and sends it as a secret when a player joins the table. A random number is not really necessary, and every secret number would work just as well.
6. As long as at least one player is not colluding with the rest, and by properties of sha256, the seeds for shuffling the deck are known only to the contract and to no one else. If all players are colliding then they might as well play with open hands. :joy:

## Use an external oracle

No implementation exists yet, but it's not that hard to implement.

For example:

1. Have a `handle` function `input_entropy` for users to send entropy in.
2. `input_entropy` will have a storage key named `seed`.
3. On each input to `input_entropy`, `seed = hash(seed + input)`.
4. Have another `handle` function `get_random_number` for users to get a random number.
5. `get_random_number` must also add to the entropy pool, otherwise consecutive `get_random_number` calls will output the same random number. For example `seed = hash(seed + msg.sender + block.height + ...)`.
6. `get_random_number` will just return the `hash(seed)` or some other non-reversible derivative of it, and update the `seed` with the new entropy like described in the previous point.
7. Have `get_random_number` also callback to the caller contract with the random number.
8. You can even have a cron job to send data from [random.org](https://www.random.org/) to `input_entropy`.

This exmaple has a much worse UX than rolling your own randomness, but at least contracts won't have to rely on users to send entropy and also won't take the risk of messing up the implementation.

# Outputs

# External query

# Compiling

# Storing and deploying

# Making your contract's code verified on exporers

# Testing

# Debugging

# Building secret apps with SecretJS

A Secret App, or a SApp, is a DApp with computational and data privacy.
A Secret App is usually comprised of the following components:

- A Secret Contract deployed on the Secret Network
- A frontend app built with a JavaScript framework (E.g. ReactJS, VueJS, AngularJS, etc.)
- The frontend app connects to the Secret Network using SecretJS,
- SecretJS interacts with a REST API exposed by nodes in the Secret Network. The REST API/HTTPS server is commonly referred to as LCD Server (Light Client Daemon :shrug:). Usually by connecting SecretJS with a wallet, the wallet handles the interactions with the LCD server.

## Wallet integration

Still not implemented in wallets. Can implement a local wallet but this will probably won't be needed anymore after 2020.

# Differences from CosmWasm

Secret Contacts are based on CosmWasm v0.10, but in order to preserve privacy, they diverge in functionality in some cases.

- `code_hash` in callbacks
- access to the current contract's `code_hash` via `env.contract_code_hash`
- contract labels are unique, thus mandatory on callback to `init`
- `migrate` and `admin` for contracts is not allowed
- iterator (`db_scan`, `db_next`) on contract state keys is not allowed
- `cosmwasm_std` changes...
