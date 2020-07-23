## WARNING - Work in progress and unfinished

### General

The general steps we perform to create a secret contract are:

1.  Upload Code - Upload some optimized wasm code, no state nor contract address (example Standard ERC20 contract)
2.  Instantiate Contract - Instantiate a code reference with some initial state, creates new address (example set token name, max issuance, etc for my ERC20 token)
3.  Execute Contract - This may support many different calls, but they are all unprivileged usage of a previously instantiated contract, depends on the contract design (example: Send ERC20 token, grant approval to other contract)

In this guide we will focus on steps 1 and 2.

You can read more about the process, and other comparisons to Solidity, in the  [cosmwasm documentation](https://www.cosmwasm.com/docs/getting-started/smart-contracts).

### 1. Download the prebuilt contract 

`<todo: add link>`

### 2. Upload the optimized contract.wasm:

```
secretcli tx compute store contract.wasm --from <wallet_name> --gas auto -y
```

#### Check that your code was uploaded properly

List current smart contract code

```
secretcli query compute list-code
[
  {
    "id": 1,
    "creator": "secret1klqgym9m7pcvhvgsl8mf0elshyw0qhruy4aqxx",
    "data_hash": "0C667E20BA2891536AF97802E4698BD536D9C7AB36702379C43D360AD3E40A14",
    "source": "",
    "builder": ""
  }
]
```

You should see the `data_hash` of `TBD` with your address as the `creator`

### 3. Instantiate the Smart Contract

At this point the contract's been uploaded and stored on the testnet, but there's no "instance". This allows to deploy multiple instances from the same code, for example if you wanted to create multiple different "ERC-20" coins from the same base contract. You can read more about the logic behind this decision, and other comparisons to Solidity, in the  [cosmwasm documentation](https://www.cosmwasm.com/docs/getting-started/smart-contracts).

To create an instance of this project we must also provide some JSON input data, a starting count.

```
INIT="{\"count\": 100000000}"
CODE_ID=<code_id from previous step>
secretcli tx compute instantiate $CODE_ID "$INIT" --from a --label "my counter" -y --keyring-backend test
```
With the contract now initialized, we can find its address

secretcli query compute list-contract-by-code 1

Our instance is secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg

We can query the contract state

CONTRACT=secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg
secretcli query compute contract-state smart $CONTRACT "{\"get_count\": {}}"

And we can increment our counter

secretcli tx compute execute $CONTRACT "{\"increment\": {}}" --from a
