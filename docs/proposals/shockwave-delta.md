This proposal proposes that the chain elect to do a software upgrade to the v1.4 software version of the Secret Network codebase on block **5,309,200**, which is estimated to occur on **Wednesday, September 21, 2022 at 2:00PM UTC**. Block times have high variance, so please monitor the chain for more precise time estimates. ETA monitor can be found at [mintscan.io/secret/blocks/5309200](https://www.mintscan.io/secret/blocks/5309200).

## Upgrade highlights

This upgrade adds the following features:

- CosmWasm v1
- Bump WASM gas costs
  - Base WASM invocation from 10k to 100k
  - WASM chain access 2k per access
- Add support for EIP191 signatures (MetaMask compatible data)
- Ledger support for Authz & Feegrant
- Revert Chain of Secrets tombstone state and restore slashed funds to all delegators

Full changelog: [CHANGELOG.md](https://github.com/scrtlabs/SecretNetwork/blob/master/CHANGELOG.md#140)

### CosmWasm v1

CosmWasm v1 is added alongside the already existing CosmWasm v0.10.

- All on-chain existing v0.10 contract will keep function just the same.
- It's still possible to code & deploy new v0.10 contracts.
- v1 and v0.10 contract can call & query each other.
- v1 contracts have encrypted inputs, outputs & state just like v0.10 contracts.
- Inbound IBC packets can be encrypted or plaintext (support IBC apps like transfer & ICA).
- Outbound IBC packets are plaintext.

For more info about CosmWasm see [docs.cosmwasm.com](https://docs.cosmwasm.com).

### Bump WASM gas costs

- Bump WASM invocation cost from 10k gas to 100k gas
- Set WASM chain access as 2k gas per request
  - read from storage
  - write to storage
  - query the chain or other contracts

### Add support for EIP191 signatures (MetaMask compatible data)

This will allow easier onboarding of users coming from EVM chains.

- Before: [before.png](https://gateway.pinata.cloud/ipfs/QmeCGJksrse5UbAcawa3wyJMU5q6i56hNg9pQE7ZJMjMJ4)
- After: [after.png](https://gateway.pinata.cloud/ipfs/Qme1QCqeSEUTvDYn2QrncfyEN6uz88RogykwbvuPqXYtzR)

### Ledger support for Authz & Feegrant

This will allow ledger uses to use dApps like [REStake](https://restake.app) & [Yieldmos](https://www.yieldmos.com).

### Revert Chain of Secrets tombstone state

On block 5,181,126, the Secret Network halted for 5.5 hours (from 2022-09-12 15:06:04 UTC to 2022-09-12 20:41:54 UTC). While helping to recover network operations, the Chain of Secrets validator double signed block 5,181,126. As a result, all Chain of Secrets delegators incurred a 5% fine (slash), and the Chain of Secrets validator got tombstoned (jailed forever).

SCRT Labs believes that this was an honest mistake and not byzantine behavior, thus in the new binary we added code to remedy this situation:

- Revert the tombstone state of the Chain of Secrets validator
- Re-mint all burned funds + 9 days of APR compensation
- Funds will be restored for all delegators as bonded to the Chain of Secrets validator

More information about the math can be found here: [https://github.com/scrtlabs/SecretNetwork/blob/efe43e1890e8605e78df6164147f3a0a43ef7b83/app/upgrades/v1.4/records.go](https://github.com/scrtlabs/SecretNetwork/blob/efe43e1890e8605e78df6164147f3a0a43ef7b83/app/upgrades/v1.4/records.go).

The halt post-mortem is being worked on and will be release in the coming days.

## Getting prepared for the upgrade

See [docs.scrt.network](https://docs.scrt.network/secret-network-documentation/post-mortems-upgrades/upgrade-instructions/shockwave-delta) for upgrade instructions.
