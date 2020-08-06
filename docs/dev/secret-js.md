# SecretJS and CosmWasm

Throughout the [Secret Network Contract Dev Guide](/dev/contract-dev-guide.md) we interacted with the blockchain using secretcli, we can also run a rest server and expose the api to any rest client.

In this guide we'll use SecretJS, which is based on [CosmJS](hhttps://github.com/CosmWasm/cosmjs), the Swiss Army knife to power JavaScript based client solutions ranging from Web apps/explorers over browser extensions to server-side clients like faucets/scrapers in the Cosmos ecosystem.

# Resources
- [cosmwasmclient-part-1](https://medium.com/confio/cosmwasmclient-part-1-reading-e0313472a158)
- [cosmwasmclient-part-2](https://medium.com/confio/cosmwasmclient-part-2-writing-dfb608f1a7f9)
- [Introduction to CosmWasm JS](https://medium.com/confio/introduction-to-cosmwasm-js-548f58d9f6af)

## Start the node

```bash
# Start enigmachain from your project directory so it's mounted at /code in the container
docker run -it --rm \
 -p 26657:26657 -p 26656:26656 -p 1317:1317 \
 -v $(pwd):/code \
 --name enigmadev enigmadev
```

## Start the rest server
This allows API access to the Secret Network

**NOTE**: In a new terminal
```bash
docker exec enigmadev \
  enigmacli rest-server \
  --node tcp://localhost:26657 \
  --trust-node \
  --laddr tcp://0.0.0.0:1317
```

## Install CosmWasm CLI 
[Also see installation guide](https://github.com/CosmWasm/cosmwasm-js/tree/master/packages/cli#installation-and-first-run)

Installing the CLI / REPL (read–eval–print loop) is optional, but does provide a handy playground for development. The script below can be executed from any Node.js script, web app or browser extension.

```bash
# Install the cli local to your new Counter project
yarn add @cosmwasm/cli --dev

# start cosmwasm-cli
npx @cosmwasm/cli
```

## CosmWasmClient Part 1: Reading

```ts
// connect to rest server
// For reading, CosmWasmClient will suffice, we don't need to sign any transactions
const client = new CosmWasmClient("http://localhost:1317")

// query chain ID
await client.getChainId()

// query chain height
await client.getHeight()

// Get deployed code
await client.getCodes()

// Get the contracts for our simple counter
const contracts = await client.getContracts(1)

const contractAddress = contracts[0].address

// Query the current count
let count = await client.queryContractSmart(contractAddress, { "get_count": {}})

// Note the result is JSON, so we have to parse it

JSON.parse(fromUtf8(count))
```

## CosmWasmClient Part 2: Writing

To increment our counter and change state, we have to connect our wallet

Start cosmwasm-cli

This time we initialize with [helpers from cosmwasm-cli examples](https://github.com/levackt/cosmwasm-js/blob/master/packages/cli/examples/helpers.ts), and easily configure fees, create random accounts etc

```bash
npx @cosmwasm/cli --init helpers.ts
```

```ts
.editor
// These options are needed to configure the SigningCosmWasmClient to use enigma-testnet

const enigmaOptions = {
  httpUrl: "http://localhost:1317",
  networkId: "enigma-testnet",
  feeToken: "uscrt",
  gasPrice: 0.025,
  bech32prefix: "enigma",
}
^D

// Either load or create a mnemonic key from file foo.key
const mnemonic = loadOrCreateMnemonic("foo.key");

// connect the wallet, this time client is a SigningCosmWasmClient, in order to sign and broadcast transactions.
const {address, client} = await connect(mnemonic, enigmaOptions);

// Check account
await client.getAccount();

// If the result is `undefined` it means the account hasn't been funded.

// Upload the contract
const wasm = fs.readFileSync("contract.wasm");
const uploadReceipt = await client.upload(wasm, {});

// Get the code ID from the receipt
const codeId = uploadReceipt.codeId;

// Create an instance
const initMsg = {"count": 0}

const contract = await client.instantiate(codeId, initMsg, "My Counter")

const contractAddress = contract.contractAddress

// and because we imported the helpers, we can use smartQuery instead of client.queryContractSmart
smartQuery(client, contractAddress, { get_count: {} })

// The message to increment the counter requires no params
const handleMsg = { increment: {} }

// execute the message
client.execute(contractAddress, handleMsg);

// Query again to confirm it worked
smartQuery(client, contractAddress, { get_count: {} })

```
![](cosmwasm-cli.png)