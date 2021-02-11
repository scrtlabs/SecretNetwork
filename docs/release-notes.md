## Release notes

### Current Release - v1.0.5

- Added debug prints in `cosmwasm_std`. These can be used by importing
  `cosmwasm_std::debug_print` which is both a `println!`-like macro, and a
  function that takes `String` and `&str` objects. By default, this function
  and macro don't do anything, but if the `debug-print` feature is enabled
  in `cosmwasm_std`, the calls will trigger a log in `enigmampc/secret-network-sw-dev`.

## Previous releases

### v1.0.0

- Secret Contracts

### v0.8.0

Release candidate running 99% of the code which is in the v1.0.0 mainnet release. The primary goal of this version was for node runners and validators 
to be able to test their setups on a configuration which will mirror the mainnet release.

#### Contracts

- Upgraded CosmWasm to v0.10
- Added ability to vote on governance proposals from secret contracts
- Added ability to query staking rewards from secret contracts
- Added ability to query current interest rates from secret contracts

#### Registration Process

- Added even more details in attestation. You will now be notified of the exact advisories that will cause the registration to fail, and possible remedies.

#### Network

- `SW_HARDENING_AND_CONFIGURATION_NEEDED` is now an acceptable attestation status for _mainnet_ for the advisory "INTEL-SA-00219"

#### CLI

- Added `secret20` commands to interact with contracts which implement the `secret20` standard. This is highly experimental

#### Bug fixes

- Fixed an issue where running `init-enclave` twice will reset registraion parameters
- Fixed queries using `--label` instead of contract address in `secretcli q compute query`

#### General

- Made the world a better place
- Security and stablility fixes

### v0.7.0

We are currently on our second testnet, and the first open testnet release since the launch of the Secret Network.

This release includes full Secret Contract functionality, stability, network configuratin, and many other changes.

#### Known Issues

- Running `init-enclave` twice will reset registraion parameters and cause you to have to wipe and re-register the node
- `--label` doesn't work for `secretcli q compute query`
- A contract trying to call staking queries may function incorrectly 

#### CLI

- The default coin type for HD derivation in the CLI has changed from 118 (ATOM) to 529 (SCRT). To revert to the previous scheme,
  use the flag `--legacy-hd-path`. Due to the way the cosmos ledger app functions, this flag must be used when adding ledger keys.

- You can now use `--label` to execute contracts instead of using a contract address. Use with the flag `--label` and omit the contract address
  (not available for queries on this release)

- The default `--gas-prices` for the CLI is set to `1uscrt`

- Fixed a bug that caused the flag `--generate-only` to function incorrectly

- Added new secretd commands:
  - `secretd check-enclave` - will check the enclave status. Use this to check if SGX is working
  - `secretd reset-enclave` - will delete registration specific files. Use when reseting a node using `unsafe-reset-all` or when debugging the registration process

#### Network

- `SW_HARDENING_AND_CONFIGURATION_NEEDED` is now an acceptable attestation status _for this testnet_

- The default `min-gas-prices` for validators is set by default to **1uscrt**

- Increased `timeout_precommit` to 2s from 1s to allow more time to collect precommits from validators for long executions

- Set max gas per block to 10,000,000 gas

- Added an option to set max query gas allowed per node. The default is set at 3,000,000 gas. You can find this value in the `~/.sceretd/config/app.toml`

#### Contracts

- This version will require any previous contracts to be recompiled using the "v0.7.0" branch & updates to the NPM package

- Added functionality for `external queries`. This allows a contract to query the chain state, or another contract mid-execution

- Changed the Wasm message types to include a new field: `callback_code_hash`. This field must include the code hash of
  any contracts you wish to send a message to, from another contract
  
- Added the field `contract_code_hash` to `env` passed into `handle`, `init` and `query` methods

- Added Staking for Secret Contracts! You can now perform staking operations directly from Secret Contracts

#### Registration Process

- Added more detail in on-chain verification. You should now see the exact reason for reigstration failure returned

- Added local verification during attestation. `Platform Okay!` will be printed out if the platform is compatable with the
  target network, as well as additional detail if patching is required

- Registration service url switched to `register.pub.testnet.enigma.co:26666`

#### General

- Changed the .deb installer to choose the user that runs the command rather than the terminal owner
- Security and stablility fixes

### v0.5.0

Our initial testnet release.

#### Network

- Added SGX as a requirement for all network nodes. To sync with the network, a node must go through the registration process

#### CLI

- Added the `compute` module, which is used to interact with Secret Contracts
- Added the `register` module, which is used to authenticate new nodes before they can sync with the network
- Replaced the standard cosmwasm command `query tx contract-state smart` with `query tx query`

#### Contracts

- Added initial Secret Contract functionality! This release does not yet contain external querying, or staking
- Published SecretJS to NPM for usage in browsers and node.js apps

#### General

- Created Azure images for easy deployment. These are available as quickstart templates, and will be available on the marketplace in the future
