# Intel SGX

## Overview

Intel’s Software Guard Extensions (SGX) are a form of Trusted Execution Environment (TEE) that the Secret Network will use. SGX chips are found in most Intel hardware products. From the perspective of users and most application developers, these SGX chips work like black boxes for data. This means no one - neither the device owner nor system operator, nor an observer of the Secret Network - can see what is happening inside that memory space. The Secret Network currently uses Intel SGX enclaves because they provide strong cryptographic guarantees.

Enclaves contain their own private signing/attestation key which is generated within the enclave. No-one has access to it outside of the enclave. It follows that data can only be signed with this key as part of the specified instruction set running in an enclave. For more details on key generation and management within enclaves, see our section about [encryption](/protocol/encryption-specs.md). For our purposes, the attestation key is only used once upon registration. Following that process new keys are provisioned to the enclave and used to communicate with the network, as described in more detail below.

Enclaves also go through a detailed registration and attestation process. Specifically, the attestation process which each validator running an SGX enclave must go through ensures the following assertions regarding privacy and correctness:
* the application’s identity, 
* its intactness (that it has not been tampered with), 
* that it is running securely within an enclave on an Intel SGX enabled platform.

For more detailed information on the Intel SGX remote attestation process, see the below section on the attestation process. 

## Why SGX

Intel SGX is one of the most widely available, and widely used implementations of Trusted Execution Environments. We have selected this technology for the initial version of the Secret Network for two main reasons: 
1. Usability: SGX is more performant and more flexible than other solutions for privacy-preserving computation. The Secret Network is building a platform for decentralized, general purpose private computation. This requires a privacy solution that can enable a wide-range of use cases. It also requires computations to be on par, performance-wise, with non-privacy preserving computation, so that speed does not interfere with application usability. 
2. Security: Because SGX is one of the most widely adopted technologies for TEEs, it is also battle-hardened. Attacks are often theoretical, executed in laboratory settings, and are rapidly addressed by Intel. Many high-value targets exist which have not been compromised. No privacy solution is 100% secure, but we believe the security guarantees provided by Intel SGX are adequate for a wide range of use-cases.

## SGX Updates

The Secret Network uses validators who are equipped with Intel SGX. Upon registration, validators will be required to have the latest compatible version of Intel SGX. When significant updates are released, the network may enforce upgrades via a governance proposal and accompanying Secret Network code update (hard fork).

## Remote Attestation

Remote attestation, an advanced feature of Intel SGX, is the process of proving that an enclave has been established in a secure hardware environment. This means that a remote party can verify that the right application is running inside an enclave on an Intel SGX enabled platform. Remote attestation provides verification for three things: the application’s identity, its intactness (that it has not been tampered with), and that it is running securely within an enclave on an Intel SGX enabled platform. Attestation is necessary in order to make remote access secure, since very often the enclave’s contents may have to be accessed remotely, not from the same platform [[1]]

The attestation process consists of seven stages, encompassing several actors, namely the service provider (referred to as a challenger) on one platform; and the application, the application’s enclave, the Intel-provided Quoting Enclave (QE) and Provisioning Enclave (PvE) on another platform. A separate entity in the attestation process is Intel Attestation Service (IAS), which carries out the verification of the enclave [[1]][[2]][[3]].

In short, the seven stages of remote attestation comprise of making a remote attestation request
(stage 1), performing a local attestation (stages 2-3), converting the local attestation to a remote
attestation (stages 4-5), returning the remote attestation to the challenger (stage 6) and verifying
the remote attestation (stage 7) [[1]][[3]].

Intel Remote Attestation also includes the establishment of a secure communication session between the service provider and the application. This is analogous to how the familiar TLS handshake includes both authentication and session establishment.

[1]: https://courses.cs.ut.ee/MTAT.07.022/2017_spring/uploads/Main/hiie-report-s16-17.pdf
[2]: https://software.intel.com/en-us/articles/innovative-technology-for-cpu-based-attestation-and-sealing
[3]: https://software.intel.com/content/www/us/en/develop/download/intel-sgx-intel-epid-provisioning-and-attestation-services.html