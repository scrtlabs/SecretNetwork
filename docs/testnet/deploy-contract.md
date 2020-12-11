# :warning: This is an old testnet guide and will not work on current mainnet and testnets :warning:

### General

The general steps we perform to create a Secret Contract are:

1.  Upload Code - Upload some optimized wasm code, no state nor contract address (example Standard ERC20 contract)
2.  Instantiate Contract - Instantiate a code reference with some initial state, creates new address (example set token name, max issuance, etc for my ERC20 token)
3.  Execute Contract - This may support many different calls, but they are all unprivileged usage of a previously instantiated contract, depends on the contract design (example: Send ERC20 token, grant approval to other contract)

In this guide we will focus on steps 1 and 2.

You can read more about the process, and other comparisons to Solidity, in the [cosmwasm documentation](https://www.cosmwasm.com/docs/getting-started/smart-contracts).

### 1. Download the prebuilt contract

`https://github.com/enigmampc/SecretNetwork/releases/download/v0.8.1/contract.wasm.gz`

This contract is a copy of the contract used to create the SSCRT privacy coin, so now you'll be creating your own privacy coin!

Download this contract somewhere it can be accessed using `secretcli`

### 2. Upload the optimized contract.wasm:

```
secretcli tx compute store contract.wasm.gz --from <wallet_name> --gas auto -y
```

This command will store your contract code on-chain, which can then be used to initialize a new contract instance

#### Check that your code was uploaded properly

List current smart contract code

```
secretcli query compute list-code
[
  ...,
  {
    "id": 12,
    "creator": "secret1klqgym9m7pcvhvgsl8mf0elshyw0qhruy4aqxx",
    "data_hash": "655CD32C174731A5A06A75F5FCC9B4E76D1556C208454FCB9E0062BA10C98409",
    "source": "",
    "builder": ""
  }
  ....
]
```

You should see the `data_hash` of `655CD32C174731A5A06A75F5FCC9B4E76D1556C208454FCB9E0062BA10C98409` with your address as the `creator`

Remember the "id" of the code. We will be using it in the next step.

### 3. Instantiate the Smart Contract

At this point the code been uploaded and stored on the testnet, but it has not been initialized yet. This is how we can allow the deployment of multiple contract instances from the same code. For example, if you wanted to create multiple different "ERC-20" coins from the same base contract. You can read more about the logic behind this decision, and other comparisons to Solidity, in the [cosmwasm documentation](https://www.cosmwasm.com/docs/getting-started/smart-contracts).

To create a new contract instance we must provide some initialization data, encoded as JSON. Remember we're creating a new privacy coin, so we will have a number of different parameters to tune.

These parameters are:

- `name` - string - name of the coin
- `symbol` - string - short form of the coin (e.g. SCRT/BTC/ETH/etc.)
- `decimals` - integer - number of decimals this coin supports (e.g. SCRT supports 6, ETH supports 18)
- `initial_balances` - the initial distribution of coins. A list of values in the form `[{"address": "<address>", "amount": "<num_of_coins>"}, {"address": "<address2>"...]`. This can also be an empty array (`[]`) if you do not wish to allocate any tokens

Example parameters:

`{"name": "Example Coin", "symbol": "EXC", "decimals": 6, "initial_balances": [{"address": "secret13flczxqyzvqrvv0npvap6qfg66zan4fy83la63", "amount": "1000"}]}`

Now, to initialize the contract we will use the following command (replace the parameters with values of your choosing):

```
secretcli tx compute instantiate <code_id> --label <choose-an-alias> '{"name": "<coin_name>", "symbol": "<coin_symbol>", "decimals": <num_of_decimals>, "initial_balances": []}' --from <key-alias>
```

Be careful not to forget the quotes at the begining and end of the initialization parameters!

#### Find the contract address

The contract is now initialized, congratulations! Our final step is to find the newly deployed contract address. We can do this in a number of ways:

- Check the explorer. If successful, your contract address should be displayed in the transactions tab

- Query the deployed instances by code id - `secretcli query compute list-contract-by-code <code_id>`

- Query the transaction you just sent using `secretcli q tx <TX_HASH>` and look for the attribute `contract_address`

Congratulations, you're done deploying your first contract!

The contract code is based on the code that can be found [here](https://github.com/enigmampc/secret-secret) if you want to learn more, or interact with your new privacy coin.
