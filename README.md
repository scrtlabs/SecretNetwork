![Secret Network](/logo.png)

<p align="center">
Secret Network secures the decentralized web
</p>

# The Romulus Upgrade

This is the Romulus Upgrade for the Secret Network as specified in the mainnet _Romulus Upgrade Signal_ proposal:

[Proposal 13](https://secretscan.io/governance/proposal/13)


```bash
This proposal is intended to set a time to upgrade the Secret Network. This upgrade will rename the current enigma 
prefix to secret in order to rebrand the address format, additionally it will bring other improvements such as the 
addition of a new module.

The proposed time for this upgrade is approximately 5am PT Wednesday June 17th 2020.

The proposed time approximately corresponds to block 1,794,500

All additions can be transparently reviewed in the following GitHub repo : https://github.com/chainofsecrets/TheRomulusUpgrade

Note: The original date was moved from the 10th to the 17th to allow an additional week for testing.
```

## Secret Network Version

Below is the version information for the Secret Network.

```bash
{
  "name": "SecretNetwork",
  "server_name": "secretd",
  "client_name": "secretcli",
  "version": "0.2.0-199-gcb314b9",
  "commit": "cb314b96aeff45b572e2aaaeca86ceb9aa16dac9",
  "build_tags": "ledger",
  "go": "go version go1.14.4 linux/amd64"
}
```

You can check that you have the right release by doing:

```bash
secretcli version --long | jq .
```

# Upgrade Instructions

### The Romulus Upgrade is scheduled for June 17th, 2020, Wednesday at 5:00am PST, 8:00am EST, 12:00pm UTC

## Summary

Chain of Secrets (CoS) will lead the Romulus Upgrade and you can follow along in these Rocket Chat channels:

	https://chat.scrt.network/channel/mainnet-validators
	https://chat.scrt.network/channel/romulus-upgrade

### Step 1 - Gracefully Halt the `enigma-1` Chain

All validators will full nodes will be restarted with a flag to stop the chain at the block height of *1,794,500*. The agreed upon
block height should be reached at approximately 7:30am PST, 10:30am EST, 2:30pm UTC. This step ensures that all nodes are stopped gracefully
at the same block height.

### Step 2 - Upgrade Genesis

CoS will export the genesis state and modify the chain id from `enigma-1` to `secret-1`, and converting all addresses to the new `secret` format.

The tokenswap parameters will be added to the exported genesis file.

### Step 3 - Setup Secret Network Binaries

All validators and those running full nodes will then install the `secretnetwork` release and perform configuration steps.

### Step 4 - Setup the Node/Validator

Initialize the node and import config files. Set the new genesis file and validate the checksum.

### Step 5 - Start the new Secret Node! :tada:

This is where the `secret-node` is enabled and started. Once 2/3 of online voting power comes online we'll be seeing blocks streaming.

### Step 6 - Import Wallet Keys

In this step the `enigmacli` keys are imported into `secretcli`.


## Romulus Upgrade Instructions

For detailed steps follow the [Romulus Upgrade instructions)](/docs/upgrades/romulus-upgrade-instructions.md).

Wishing much success on the day of the upgrade!!


# What is Secret Network?

Want to build a better internet? Solve for privacy.

Secret Network is a blockchain-based, open-source protocol that lets anyone perform computations on encrypted data, bringing privacy to smart contracts and public blockchains. Our mission: improve the adoption and usability of decentralized technologies, for the benefit of all.

Mainnet is out! Get the latest release at https://github.com/enigmampc/SecretNetwork/releases/latest.

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0) [![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v2.0%20adopted-ff69b4.svg)](CODE_OF_CONDUCT.md)

# Community

- Homepage: https://scrt.network
- Forum: https://forum.scrt.network
- Discord: https://discord.com/invite/SJK32GY
- Blog: https://blog.scrt.network
- Twitter: https://twitter.com/SecretNetwork
- Main Chat: https://chat.scrt.network/channel/general
- Telegram Channel: https://t.me/SCRTnetwork
- Community Secret Nodes Telegram: https://t.me/secretnodes

# Block Explorers

Secret Network is secured by the SCRT coin (Secret), which is used for fees, staking, and governance. Transactions, validators, governance proposals, and more can be viewed using the following Secret Network block explorers:

- [Cashmaney](https://explorer.cashmaney.com)
- [SecretScan](https://secretscan.io)
- [Puzzle](https://puzzle-staging.secretnodes.org/enigma/chains/enigma-1)

# Wallets

- [Ledger Nano S and Ledger Nano X](/docs/ledger-nano-s.md)
- [Math Wallet](https://mathwallet.org/web/enigma)

# Implementation Discussions

- [An Update on the Encryption Protocol](https://forum.scrt.network/t/an-update-on-the-encryption-protocol/1641)
- [Secret Contracts on Secret Network](https://forum.scrt.network/t/secret-contracts-on-enigma-blockchain/1284)
- [Network key management/agreement](https://forum.scrt.network/t/network-key-management-agreement/1324)
- [Input/Output/State Encryption/Decryption protocol](https://forum.scrt.network/t/input-output-state-encryption-decryption-protocol/1325)
- [Why the Cosmos move doesn’t mean we’re leaving Ethereum](https://forum.scrt.network/t/why-the-cosmos-move-doesnt-mean-were-leaving-ethereum/1301)
- [(Dev discussion/Issue) WASM implementation](https://forum.scrt.network/t/dev-discussion-issue-wasm-implementation/1303)

# Secret Network REST Providers

- https://api.chainofsecrets.org

# Docs

- [Install the `secretcli` light client (Windows, Mac & Linux)](/docs/light-client-mainnet.md)
- [How to use the `secretcli` light client](/docs/secretcli.md)
- [How to participate in on-chain governance](docs/using-governance.md)
- [How to run a full node on mainnet](/docs/validators-and-full-nodes/run-full-node-mainnet.md)
- [How to run an LCD server](/docs/lcd-server-example.service)
- [Ledger Nano S (and X) support](/docs/ledger-nano-s.md)
- [How to join as a mainnet validator](/docs/validators-and-full-nodes/join-validator-mainnet.md)
- [How to backup a validator](/docs/validators-and-full-nodes/backup-a-validator.md)
- [How to migrate a validator to a new machine](/docs/validators-and-full-nodes/migrate-a-validator.md)
- [How to verify software releases](/docs/verify-releases.md)
- [How to setup SGX on your machine](/docs/dev/setup-sgx.md)

# Upgrades
- [How to upgrade to the Secret Network (rebranding)](/docs/upgrades/howto-secretnetwork-rebranding.md)

# Archive

- [For Secret Network developers](/docs/dev/for-secret-network-devs.md)
- [How to be a mainnet genesis validator](/docs/genesis/genesis-validator-mainnet.md)

# License

SecretNetwork is free software: you can redistribute it and/or modify it under the terms of the [GNU Affero General Public License](LICENSE) as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version. The GNU Affero General Public License is based on the GNU GPL, but has an additional term to allow users who interact with the licensed software over a network to receive the source for that program.
