# Development Roadmap

Currently, the live Secret Network supports [staking](/validators-and-full-nodes/secret-nodes.md), [transactions](/protocol/transactions.md), and [governance](/protocol/governance.md) activities. 

The next version of Secret Network is expected to offer secret contract functionality. Our subsequent milestones are:
- [X] Enable cosmwasm-based contracts to be deployed on Secret Network testnet
- [X] Enable cosmwasm-based contracts to be deployed within Intel SGX enclaves (the TEE that the Secret Network will initially use) on the Secret Network testnet
- [X] Enable key-sharing protocol for encryption and decryption of state, as well as encryption and decryption of input/output data between clients and Validators. This is referred to as the `compute` module, which is specific to the Secret Network.

The above milestones constitute the R&D work required to enable secret contracts. When these steps are completed, Enigma will submit a proposal to the Secret Network blockchain that proposes to upgrade the network to enable secret contracts. Validators will have to vote on this submission and approve it prior to implementation.
- [ ] Proposal submitted to Secret Network