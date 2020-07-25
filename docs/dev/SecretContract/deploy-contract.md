---
title : 'Deploy Smart Contract'
---
## Deploy Smart Contract

Before deploying or storing the contract on the testnet, need to run the cosmwasm optimizer.

## Optimize compiled wasm

```
docker run --rm -v "$(pwd)":/code \
  --mount type=volume,source="$(basename "$(pwd)")_cache",target=/code/target \
  --mount type=volume,source=registry_cache,target=/usr/local/cargo/registry \
  cosmwasm/rust-optimizer:0.8.0  
```
The contract wasm needs to be optimized to get a smaller footprint. Cosmwasm notes state the contract would be too large for the blockchain unless optimized. This example contract.wasm is 1.8M before optimizing, 90K after.

The optimization creates two files:
- contract.wasm
- hash.txt

## Store the Smart Contract on our local Testnet

```
# First lets start it up again, this time mounting our project's code inside the container.
docker run -it --rm \
 -p 26657:26657 -p 26656:26656 -p 1317:1317 \
 -v $(pwd):/root/code \
 --name secretdev enigmampc/secret-network-bootstrap-sw:latest
 ```

Upload the optimized contract.wasm to _secretdev_ :

```
docker exec -it secretdev /bin/bash

cd code

secretcli tx compute store contract.wasm --from a --gas auto -y --keyring-backend test
```

## Querying the Smart Contract and Code

List current smart contract code
```
secretcli query compute list-code
[
  {
    "id": 1,
    "creator": "enigma1klqgym9m7pcvhvgsl8mf0elshyw0qhruy4aqxx",
    "data_hash": "0C667E20BA2891536AF97802E4698BD536D9C7AB36702379C43D360AD3E40A14",
    "source": "",
    "builder": ""
  }
]
```

## Instantiate the Smart Contract

At this point the contract's been uploaded and stored on the testnet, but there's no "instance."
This is like `discovery migrate` which handles both the deploying and creation of the contract instance, except in Cosmos the deploy-execute process consists of 3 steps rather than 2 in Ethereum. You can read more about the logic behind this decision, and other comparisons to Solidity, in the [cosmwasm documentation](https://www.cosmwasm.com/docs/getting-started/smart-contracts). These steps are:
1. Upload Code - Upload some optimized wasm code, no state nor contract address (example Standard ERC20 contract)
2. Instantiate Contract - Instantiate a code reference with some initial state, creates new address (example set token name, max issuance, etc for my ERC20 token)
3. Execute Contract - This may support many different calls, but they are all unprivileged usage of a previously instantiated contract, depends on the contract design (example: Send ERC20 token, grant approval to other contract)

To create an instance of this project we must also provide some JSON input data, a starting count.

```bash
INIT="{\"count\": 100000000}"
CODE_ID=1
secretcli tx compute instantiate $CODE_ID "$INIT" --from a --label "my counter" -y --keyring-backend test
```

With the contract now initialized, we can find its address
```bash
secretcli query compute list-contract-by-code 1
```
Our instance is secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg

We can query the contract state
```bash
CONTRACT=secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg
secretcli query compute contract-state smart $CONTRACT "{\"get_count\": {}}"
```

And we can increment our counter
```bash
secretcli tx compute execute $CONTRACT "{\"increment\": {}}" --from a --keyring-backend test
```

