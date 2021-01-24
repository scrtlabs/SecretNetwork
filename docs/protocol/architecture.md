# Network Architecture

The Secret Network facilitates the execution of code (Secret Contracts) with strong correctness and privacy guarantees. In Secret Contracts, data itself is concealed from the nodes that execute computations (also known as "private computation"). This allows developers to include sensitive data in their smart contracts without moving off-chain to centralized (and less secure) systems, thus allowing for truly private and scalable decentralized applications. Secret Network is a proof-of-stake blockchain built on top of the Cosmos SDK, using [Tendermint consensus](https://docs.tendermint.com/master/introduction/what-is-tendermint.html#consensus-overview). [Governance](https://github.com/enigmampc/cosmos-sdk/blob/master/x/gov/spec/README.md), [staking](https://github.com/enigmampc/cosmos-sdk/blob/master/x/staking/spec/README.md), [bank](https://github.com/enigmampc/cosmos-sdk/tree/master/x/bank/spec), and [compute](https://github.com/enigmampc/SecretNetwork/tree/master/x/compute) modules are currently enabled. 

A Secret Contract, written in Rust, is the fundamental innovation of the Secret Network. Secret Contracts are enabled by the `compute` module, which is unique to the Secret Network. These contracts execute over data which is kept encrypted from nodes, developers, users, and everyone else, while the results of these computations are trusted and verifiable. For application developers, the Secret Contract is the most important feature of the network.

The following process describes, step by step, how a contract is submitted and a computation performed on the Secret Network:

1. Developers write and deploy Secret Contracts to the Secret Network
2. Validators run full nodes and execute Secret Contracts
3. Users submit transactions to Secret Contracts (on-chain), which can include encrypted data inputs.
4. Validators receive encrypted data from users, and execute the Secret Contract.
5. During Secret Contract execution:
   - Encrypted inputs are decrypted inside a Trusted Execution Environment.
   - Requested functions are executed inside a Trusted Execution Environment.
   - Read/Write state from Tendermint can be performed (state is always encrypted when at rest, and is only decrypted within the Trusted Execution Environment).
   - Outputs are encrypted.
   - In summary, at all times, data is carefully always encrypted when outside the Trusted Compute Base (TCB) of the TEE.
6. The Block-proposing validator proposes a block containing the encrypted outputs and updated encrypted state.
7. At least 2/3 participating validators achieve consensus on the encrypted output and state.
8. The encrypted output and state is committed on-chain.

A Secret Contract’s code is always deployed publicly on-chain, so that users and developers know exactly what code will be executed on data that they submit. This is important: without knowing what that code does, users cannot trust it with their encrypted data. However, the data that is submitted is encrypted, so it cannot be read by a developer, anyone observing the chain, or anyone running a node. If the behavior of the code is also trusted (which is possible to achieve because it is recorded on chain), a user of Secret Contracts obtains strong privacy guarantees.

This encrypted data can only be accessed from within the “trusted execution environment”, or enclave, that the `compute` module requires each validator to run. The computation indicated by the Secret Contract is then performed, within this trusted enclave, over the decrypted data. When the computation is completed, the output is encrypted and recorded on-chain. There are various types of outputs that can be expected. These include:

- An updated contract state (i.e., the user’s data should update the state or be stored for future computations)
- A computation result encrypted for the transaction sender (i.e., a result should be returned privately to the sender)
- Callbacks to other contracts (i.e., a contract is called conditional on the outcome of a Secret Contract function)
- Send Messages to other modules (i.e., for sending value transfer messages that depend on the outcome of a computation). See [from go-cosmwasm code](https://github.com/enigmampc/SecretNetwork/blob/master/go-cosmwasm/types/msg.go#L63-L69)

The Secret Network’s `compute` module currently requires that validators run nodes with Intel SGX chips (enclaves). These enclaves contain signing keys that are generated within the enclave. For more details on how enclaves function and are verified, see [intel SGX](sgx.md).

![enclave](../images/diagrams/enclave.png)

Diagram: Trusted and Untrusted aspects of Secret Network code. `compute` enables go-cosmwasm with encryption to be executed within the trusted component of the enclave.

Nodes join the network through a remote attestation process that is outlined in the section about [new node registration](encryption-specs.md#new-node-registration). In short, the network shares a true random seed accessed through this registration process. This seed is generated inside the Trusted Execution Environment of the bootstrap node, which is identical to other nodes, but is the first node that joins the network. All other keys are derived from this seed in a CSPRNG way. The nodes use asymmetric encryption for agreeing non-interactively on shared symmetric keys with the users, then, symmetric encryption is used for encrypting and decrypting input and output data from users, as well as the internal contract state. For more information on the cryptography used within Secret Network, review our [encryption specs](encryption-specs.md).
