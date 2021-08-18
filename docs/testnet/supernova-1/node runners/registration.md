### Terms

#### Attestation Certificate
This is a self-signed X.509 certificate that contains a signed report by Intel, and the SGX enclave. 
The report contains both a report that the enclave is genuine, a code hash, and a signature of the creator of the enclave.

#### Seed
this is a parameter that is shared between all enclaves on the network in order to garuantee determinisic calculations. 
When a node authenticates succesfully, the network encrypts the seed and shares it with the node. Protocol internals are described [here](https://github.com/enigmampc/SecretNetwork/blob/master/docs/protocol/encryption-specs.md)  

## Background

This section will explain node registration in the Secret Network. If you just care about installation you can just follow the setup guides and ignore this document.
If, however, you want to learn more about what's going on behind the scenes here read on.

In order to verify that each node on the Secret Network is running a valid SGX node, we use a process that we call registration.
Essentially, it is the process of authenticating with the network. 

__The process is unique and bound to the node CPU__. It needs to be performed for each node, and you cannot migrate registration parameters between nodes. 
The process essentially creates a binding between the processor and the blockchain node, so that they can work together.

For this reason, the setup will be slightly more complex than what you might be familiar with from other blockchains in the Cosmos ecosystem. 

The registration process is made up of three main steps:

1. Enclave verification with Intel Attestation Service - this step creates an [_attestation certificate_](#attestation-certificate) that we will use to 
validate the node
2. On-chain network verification - Broadcast of the [_attestation certificate_](#attestation-certificate) to the network. The network will verify that 
the certificate is signed by Intel, and that the enclave code running is identical to what is currently running on the network.
This means that running an enclave that is differs by 1 byte will be impossible.
3. Querying the network for the [_encrypted seed_](#seed) and starting the node

At the end of this process (if it is successful) the network will output an _encrypted seed_ (unique to this node), which is required for our node to start. 
After decryption inside the enclave, the result is a seed that is known to all enclaves on the network, and is the source of determinism between all network nodes.

For a deeper dive into the protocol see the [protocol documentation](https://github.com/enigmampc/SecretNetwork/blob/master/docs/protocol/encryption-specs.md#node-startup)

```
Note: Due to the way rust and C code are compiled recompilation of the enclave code is non deterministic, and will be rejected during the attestation process.
This feature is refered to as a reproducable build, and is a feature that will be included in future releases.
```

## Prerequisites

To register your node, you will need:

* RPC address of an already active node. You can use `bootstrap.secrettestnet.io:26657`, or any other node that exposes RPC services.

* Account with some SCRT

## Instructions

#### Initialize secret enclave

This will perform initialization, and remote attestation (with intel IAS). Make sure SGX is enabled and running 
or this step might fail. 

`secretd init-enclave`

If `init-enclave` was succssful, you should see `attestation_cert.der` created. This is the _attestation certificate_ which we will 
need for the next step.

#### Check your certificate is valid

`PUBLIC_KEY=$(secretd parse attestation_cert.der 2> /dev/null | cut -c 3- )`
`echo $PUBLIC_KEY`

Should return your 64 character registration key if it was successful.

You can use the command `secretd parse <certificate_file>` to validate the file, and print the public key of the node.
This public key is what the network will use to encrypt the seed, so only your enclave can decrypt it.

Note: This step will locally verify the certificate only, and will not check the enclave status or the code hash of the enclave. 
Authetnication with the network may still fail due to either of those causes.

#### Register your node on-chain

`secretcli tx register auth <path/to/attestation_cert.der> --node <rpc_service> --from <your account>`

You can check the result of this transaction using 

`secretcli q tx <TX_HASH>` 

#### Get your [_encrypted seed_](#seed) from the network

If the above step was successful, you should now be able to query the blockchain for your encrypted seed.

`SEED=$(secretcli query register seed "$PUBLIC_KEY" --node <rpc_service> | cut -c 3-)
echo $SEED`

#### Get additional network parameters

The node needs a couple of additional parameters from the network before it can start. These are used to encrypt contract inputs & outputs.

`secretcli q register secret-network-params --node <rpc_service>`

This will create a couple of files in your current path, mainly `node-master-cert.der`

#### Configure your local node

Since the previous command was ran only using the `secretcli` we must now run a final command to load all our startup parameters to `secretd`

`secretd configure-secret node-master-cert.der "$SEED"`

