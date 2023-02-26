This proposal proposes that the chain elect to do a software upgrade to the v1.3.0 software version of the Secret Network codebase on block **3,343,000**, which is estimated to occur on **Wednesday, 11 May 2022 at 2:00PM UTC**. Block times have high variance, so please monitor the chain for more precise time estimates. You can monitor the ETA at [https://www.mintscan.io/secret/blocks/3343000](https://www.mintscan.io/secret/blocks/3343000).

## Upgrade highlights

This upgrade adds the following features:

- Backport API from CosmWasm v1:
  - `ed25519_verify()`
  - `ed25519_batch_verify()`
  - `secp256k1_verify()`
  - `secp256k1_recover_pubkey()`
- Add new secret CosmWasm API:
  - `ed25519_sign()`
  - `secp256k1_sign()`
- Upgrade cosmos-sdk to v0.45.4:
  - Add the `secretd rollback` command
  - Add the `~/.secretd/.compute` directory to state sync
- Upgrade the ibc-go module to v3.0.0:
  - Add interchain accounts (ICS27) host ([list of allowed messages](https://github.com/scrtlabs/SecretNetwork/blob/56fdaef168ee7d078514d87a356d4176e4b6df32/app/upgrades/v1.3/upgrades.go#L35-L55))

Full changelog: [CHANGELOG.md](https://github.com/scrtlabs/SecretNetwork/blob/master/CHANGELOG.md#130)

## Getting prepared for the upgrade

See [https://docs.scrt.network/shockwave-alpha-upgrade-secret-4.html](https://docs.scrt.network/shockwave-alpha-upgrade-secret-4.html) for upgrade instructions.
