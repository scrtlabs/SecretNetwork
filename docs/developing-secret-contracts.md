# Developing Secret Contracts

Secret Contacts are based on CosmWasm v0.10.

- [Developing Secret Contracts](#developing-secret-contracts)
- [IDE](#ide)
- [Personal Secret Network for Secret Contract development](#personal-secret-network-for-secret-contract-development)
- [Init](#init)
- [Handle](#handle)
- [Query](#query)
- [Inputs](#inputs)
- [APIs](#apis)
- [State](#state)
  - [Serializing libraries that are known to work](#serializing-libraries-that-are-known-to-work)
- [Outputs](#outputs)
- [External query](#external-query)
- [Compiling](#compiling)
- [Storing and deploying](#storing-and-deploying)
- [Testing](#testing)
- [Debugging](#debugging)
- [Building secret apps with SecretJS](#building-secret-apps-with-secretjs)
  - [Wallet integration](#wallet-integration)
- [Differences from CosmWasm](#differences-from-cosmwasm)

# IDE

Secret Contracts are developed with the [Rust](https://www.rust-lang.org/) programming language and are compiled to [WASM](https://webassembly.org/) binaries.
These IDEs are known to work very well for developing Secret Contracts:

- [CLion](https://www.jetbrains.com/clion/)
- [VSCode](https://code.visualstudio.com/) with the [rust-analyzer](https://rust-analyzer.github.io/) extension

# Personal Secret Network for Secret Contract development

# Init

`init` is the constructor of your contract. This function is called only once in the lifetime of the contract.

```bash
secretcli tx compute instantiate "$CODE_ID" "$INPUT_MSG" --label "$UNIQUE_LABEL" --from "$MY_KEY"
```

# Handle

`handle` is the implementation of execute transactions.

```bash
secretcli tx compute execute "$CONTRACT_ADDRESS" "$INPUT_ARGS" --from "$MY_KEY" # Option A
secretcli tx compute execute --label "$LABEL" "$INPUT_ARGS" --from "$MY_KEY"    # Option B
```

# Query

`query` is the implementation of read-only queries. Queries run over the current blockchain state but don't incur fees and don't have access to `msg.sender`. They are still metered by a gas limit that is set on the executing node.

```bash
secretcli q compute query "$CONTRACT_ADDRESS" "$INPUT_ARGS"
```

# Inputs

# APIs

# State

## Serializing libraries that are known to work

- `serde_json_wasm` instead of `serde_json`
- `bincode2` instead of `bincode`

# Outputs

# External query

# Compiling

# Storing and deploying

# Testing

# Debugging

# Building secret apps with SecretJS

A Secret App, or a SApp, is is a DApp with computational and data privacy.
A Secret App is usually comprised of the following components:

- A Secret Contract deployed on the Secret Network
- A frontend app built with a JavaScript framework (E.g. ReactJS, VueJS, AngularJS, etc.)
- The frontend app connects to the Secret Network using SecretJS,
- SecretJS interacts with a REST API exposed by nodes in the Secret Network. The REST API/HTTPS server is commonly refered to as LCD Server (Light Client Daemon :shrug:). Usually by connecting SecretJS with a wallet, the wallet handles the interactions with the LCD server.

## Wallet integration

Still not implemented in wallets. Can implement a local wallet but this will probably won't be needed anymore after 2020.

# Differences from CosmWasm

Secret Contacts are based on CosmWasm v0.10, but in order to preserve privacy, they diverge in functionality in some cases.

- `code_hash` in callbacks
- contract labels are unique, thus mandatory on callback to `init`
- `migrate` and `admin` for contracts is not allowed
- iterator (`db_scan`, `db_next`) on contract state keys is not allowed
- `cosmwasm_std` changes...
