# Development Roadmap

Currently, the live Secret Network supports [staking](../validators-and-full-nodes/secret-nodes.md), [transactions](transactions.md), and [governance](governance.md) activities.

The next version of Secret Network is expected to offer Secret Contract functionality. Our subsequent milestones are:

- [x] Enable cosmwasm-based contracts to be deployed on Secret Network testnet
- [x] Enable cosmwasm-based contracts to be deployed within Intel SGX enclaves (the TEE that the Secret Network will initially use) on the Secret Network testnet
- [x] Enable key-sharing protocol for encryption and decryption of state, as well as encryption and decryption of input/output data between clients and Validators. This is referred to as the `compute` module, which is specific to the Secret Network.

Read more about the completion of [milestone 3 of 3](https://blog.scrt.network/secret-contracts-update-milestone-3-of-3-is-complete)!

The above milestones constitute the R&D work required to enable Secret Contracts. After these steps are completed, Enigma will submit a proposal to the Secret Network blockchain that proposes to upgrade the network to enable Secret Contracts. Validators will have to vote on this submission and approve it prior to implementation.

- [ ] Proposal submitted to Secret Network
