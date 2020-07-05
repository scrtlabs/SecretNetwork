### 1. Start a node locally -
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
https://faucet.testnet.enigma.co/
```
### 3. Start a node on Azure
```
https://portal.azure.com/#create/enigmampcinc1592297592354.enigma-secret-node-v0-previewp0
```
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

### 5. Write a contract with an addition function, that takes 2 inputs and adds them
```
https://github.com/CosmWasm/cosmwasm
https://github.com/CosmWasm/cosmwasm-template
```
#### 5.1 Deploy it to your local dev environment
#### 5.2 Call it using the secretcli
#### 5.3 Deploy it to the testnet
#### 5.4 Check how the deployment transaction looks on the explorer
#### 5.5 Test your function on the testnet

### 6. Go crazy:)

Suggestions for stuff to test while you're hacking away:
* Upload a contract with an error so you can see what an error looks like to a contract developer
* Start a node using a different version to see what a registration failure looks like
* Create contracts that do silly things to try and crash the network