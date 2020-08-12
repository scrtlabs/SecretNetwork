# Validators

Secret Network is secured by a coordinated group of validators (current maximum: 50) using a Byzantine fault tolerant delegated proof-of-stake consensus engine, [Tendermint](https://tendermint.com). These validators stake their own SCRT coins and coins from delegators in order to earn rewards by successfully running the protocol, verifying transactions, and proposing blocks to the chain. If they fail to maintain a consistent and honest node, they will be slashed and coins will be deducted from their account.

It is possible for anyone who holds SCRT to become a Secret Network validator or delegator, and thus participate in both staking and governance processes. If and when the network upgrades to integrate secret contract functionality, validators will be required to run nodes equipped with the latest version of Intel SGX. For information on running a node, delegating, staking, and voting, please see the walkthrough below and visit our [governance documentation](../protocol/governance.md). Here is a [list of compatible hardware](https://github.com/ayeks/SGX-hardware) (not maintained by Enigma or the Secret Network community).

## Walkthrough

1. [Install SGX](setup-sgx.md)
2. [Run a full node](run-full-node-mainnet.md)
3. [Be a validator](join-validator-mainnet.md)
