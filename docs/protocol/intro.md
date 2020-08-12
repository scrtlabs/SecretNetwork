# Secret Network Protocol

The Secret Network Protocol is run by all validators in the network. The protocol consists of two main components: trusted and untrusted sections. The untrusted section is responsible for all untrusted code used by validators in the network, including Cosmos-SDK code, tendermint code, and the creation of enclaves (trusted components). The trusted component (the enclave) is responsible for executing CosmWasm based contracts, executing any functions designated within these contracts, as well as reading and writing (encrypted data) to Tendermint state during execution. The Secret Network’s `x/compute` model bridges these two sections, and enables access to the trusted component.

Secret Network enables validators to perform computation over private data. This involves a number of processes, including:

- [Registration](encryption-specs.md#new-node-registration): the process of a new validator joining the network
- [Encryption / Decryption](encryption-specs.md): both of network-wide state keys, as well as encryption keys for input and output of specific computations
- [Computation](components.md#secret-contracts): executing secret contract code

The Secret Network performs computations in a Trusted Execution Environment. More precisely, the Secret Network currently uses [Intel® Software Guard Extensions](https://en.wikipedia.org/wiki/Software_Guard_Extensions) (Intel® SGX), which is set of instructions that increase the security of application code and data, giving them more protection from disclosure or modification.

As requested by SGX technology, the Secret Network Protocol code is partitioned into trusted and untrusted parts. The trusted part contains sensitive data and it is executed inside an “enclave,” which is a term referring to the secure component of a Trusted Execution Environment. The untrusted part includes the rest of the code, along with code for managing the enclave (load and destroy).

![enclave](../.vuepress/public/diagrams/enclave.png)

Figure: diagram of Core, trusted and untrusted components

### Trusted Core

- The trusted part of the core Secret Network protocol is responsible for execution of the secret contracts.
- This part is also responsible for SGX-specific mechanisms: Remote Attestation and Sealing (see subsections). The core protocol uses [Rust SGX SDK](https://github.com/apache/incubator-teaclave-sgx-sdk) to call Intel® SGX C++ APIs.
- The trusted part is able to make read/write calls from Tendermint state at any point during execution. Note: During contract execution you can only change the state of the contract you're executing. You can query other contracts (i.e. run code that can't change the state of another contract) synchronously, but you can only queue calls to other contracts and requests for transactions. Those operations will happen after the contract has finished running. This is intentional as it prevents a lot of bugs, like the reentrancy bugs that plague ethereum.
- The enclave only stores the seed. The enclave may potentially also store the local node's (sk_node,pk_node) key pair, though this is not yet decided.

### Untrusted Core

- The untrusted part of the core Secret Network protocol is responsible for running the Cosmos SDK and Tendermint.
- The untrusted part also contains code for managing the enclave (load and destroy).
- The untrusted part can call the CosmWasm module and kick off secret contract execution within the enclave

### Sealing

Trusted Execution Environments are essentially stateless. To preserve the information that’s stored in an enclave, it must be explicitly sent outside the enclave to untrusted memory. SGX provides a capability called [data sealing](https://software.intel.com/en-us/blogs/2016/05/04/introduction-to-intel-sgx-sealing) which encrypts enclave data in the enclave using an encryption key that is derived from the CPU. This encrypted data block can only be decrypted, or unsealed, on the same system. This SGX-specific method for storing data is not used to store computation input/output data in the Secret Network. It is used to store the enclave’s signing key.

We seal the signing key because this key is created during the remote attestation process. We do not want the enclave to be required to perform [remote attestation](encryption-specs.md#new-node-registration) between each computation. If the enclave fails for some reason, and the key is lost, the worker would be obligated to go through the remote attestation process again. The only way to store persistent data from the enclave is through sealing.

## Registration and Cryptography

The Secret Network enables Secret Contracts through the use of both encryption protocols and trusted execution environments (TEEs). The reliability of TEEs is verified through a process known as remote attestation, which is required for a node to participate in the network. The Secret Network uses both symmetric and asymmetric encryption protocols. Specifically, asymmetric cryptography is used for achieving consensus and sharing secrets between nodes and users, whereas symmetric cryptography is used for input/output encryption with users of secret contracts, as well as internal contract state encryption. For a more extensive discussion of our cryptography approaches, see our [encryption specs](encryption-specs.md).

Secret Network Protocol uses ECDH key exchange mechanism between users and validators. This process involves the user, the Secret Blockchain, as well as the trusted component of the Secret Protocol. It is initiated any time a transaction is sent from the user to the Secret Contract.
