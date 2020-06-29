## Registration

### Background

This section will explain node registration in the Secret Network. If you just care about installation 
instructions skip to the next section.

In order to verify that each node on the Secret Network is running a proper SGX node, we use a process that we call registration.
Essentially, it is the process of authenticating with the network. 

__The process is unique and bound to the node CPU__. It needs to be performed for each node, and you cannot migrate registration parameters between nodes.

For this reason, the setup will be slightly more complex than what you might be familiar with from other blockchains in the Cosmos ecosystem. 

The process is made up of two steps:

1. Enclave verification with Intel Attestation Service - this step creates an _attestation certificate_ that we will use to 
validate the node 
2. On-chain network verification - Broadcast of the _attestation certificate_ to the network. The network will verifiy that 
the certificate is signed by Intel, and that the code running in the enclave is identical to what is currently running on the network.

At the end of this process (if it is successful) the network will output an _encrypted seed_ (unique to this node), which is required for our node to start. 

For a deeper dive into the protocol see the [protocol documentation](link)

### Prerequisites

To register your node, you will need:

* RPC address of an already active node. You can use
`registration.enigma.co:26657`, or any other node that exposes RPC services.

* Account with at least 1 SCRT


### Instructions

* Initialize secret enclave. This will perform initialization, and remote attestation. Make sure SGX is enabled and running 
or this step might fail. 

`secretd init-enclave`

If `init-enclave` was succssful, you should see `attestation_cert.der` created. This is the _attestation certificate_ which we will 
need for the next step 

* Check your certificate is valid -
`PUBLIC_KEY=$(secretd parse attestation_cert.der 2> /dev/null | cut -c 3- )`
`echo $PUBLIC_KEY`
Should return your 64 character registration key if it was successful.

* Register your node on-chain

`secretcli tx register auth <path/to/attestation_cert.der> --node registration.enigma.co:26657 --from <your account>`

* Get your _encrypted seed_ from the network

`secretcli q register seed "$PUBLIC_KEY" --node registration.enigma.co:26657`

* Get additional network parameters

`secretcli q register secret-network-params --node registration.enigma.co:26657`

* Configure your local node

`secretd configure-secret node-master-cert.der "$SEED"`