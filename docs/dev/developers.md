# Secret Contract Devs

Developers can write secret contracts for CosmWasm running inside secure enclaves with encryption capabilities. Once the network upgrade integrating secret contract functionality has been completed, developers will be able to use private data in contracts running on the live Secret Network. Currently, we have a [contract development guide for developers](/dev/contract-dev-guide) that demonstrates how to get the Secret Network up and running on a local deployment using Docker, as well as how to write simple contracts in Rust using CosmWasm. The walkthrough also demonstrates how to interact with these contracts and how to write tests for them.

## Getting Started

### 1. Start a node locally
#### 1.1 Make sure SGX is installed
```
https://github.com/enigmampc/SecretNetwork/blob/develop/docs/dev/setup-sgx.md
```
#### 1.2 Download secret node package and follow the instructions
```
https://github.com/enigmampc/SecretNetwork/blob/develop/docs/testnet/run-full-node-testnet.md
```
### 2. Create a local scrt address & get tokens from faucet
```
https://github.com/enigmampc/SecretNetwork/blob/develop/docs/secretcli.md

https://faucet.testnet.enigma.co
```
### 3. [Start a node](/validators-and-full-nodes/run-full-node-mainnet.html)

#### 3.1 Make sure you can access the IP and DNS address you created
#### 3.2 Try to add it as a validator
#### 3.3 Use this node as the gateway node you use to deploy your contracts

### 4. Create a development environment
checkout the tag v0.5.0-alpha2
```
make cli
docker run -p 26657:26657 enigmampc/secret-network-bootstrap-sw:latest
```
#### 4.1 make sure the node is listening on port 26657
#### 4.2 make sure cli works when using --node <node_ip>:26657

### 5. Write a contract with an addition function
```
https://github.com/CosmWasm/cosmwasm
https://github.com/CosmWasm/cosmwasm-template
```
#### 5.1 Deploy it to your local dev environment
#### 5.2 Call it using the secretcli
#### 5.3 Deploy it to the testnet
#### 5.4 Check how the deployment transaction looks on the explorer
#### 5.5 Test your function on the testnet