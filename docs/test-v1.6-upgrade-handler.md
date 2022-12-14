# How to test the v1.6 upgrade with LocalSecret

Always work in docs directory

## Step 1

Start a v1.5 chain.

```bash
 docker compose -f docker-compose-16.yml up -d
 docker cp node_init.sh node:/root/
```

On one terminal window:

```bash
docker exec -it bootstrap bash
./bootstrap_init.sh
```

On another terminal window

```bash
docker exec -it node bash
chmod 0777 node_init.sh
./node_init.sh
```

## Step 2 (Test seed rotation)

### Copy the suplied contract to the docker

```bash
docker cp ./contract.wasm node:/root/
```

### Access node docker

```bash
docker exec -it node bash
```

### Instantiate a contract and interact with him

```bash
secretd config node http://0.0.0.0:26657
secretd tx compute store contract.wasm --from a --gas 5000000 -y
sleep 5
INIT='{"counter":{"counter":10, "expires":100000}}'
secretd tx compute instantiate 1 "$INIT" --from a --label "c" -y
sleep 5

secretd tx compute execute secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg '{"increment":{"addition": 13}}' --from a -y
sleep 5
secretd query compute query secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg '{"get": {}}'
```

Expected result should be:
{"get":{"count":23}}

### Check that the bootstrap node is synced

```bash
docker exec -it bootstrap bash
secretd query compute query secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg '{"get": {}}'
```

Expected result should be:
{"get":{"count":23}}

## Step 3

Run the secret.js tests from the `master` branch on the `secret.js` repo.  
This will create state on the chain before the upgrade.

First delete `globalSetup` & `globalTeardown` (because we already launched the chain manually):

```bash
echo 'import { SecretNetworkClient } from "../src";
import { sleep } from "./utils";

require("ts-node").register({ transpileOnly: true });

module.exports = async () => {
  await waitForBlocks();
  console.log(`LocalSecret is running`);
};

async function waitForBlocks() {
  while (true) {
    const secretjs = new SecretNetworkClient({
      url: "http://localhost:1317",
      chainId: "secretdev-1",
    });

    try {
      const { block } = await secretjs.query.tendermint.getLatestBlock({});

      if (Number(block?.header?.height) >= 1) {
        break;
      }
    } catch (e) {
      // console.eerror(e);
    }
    await sleep(250);
  }
}
' > test/globalSetup.ts
```

```bash
echo '//@ts-ignore
require("ts-node").register({ transpileOnly: true });

module.exports = async () => {};' > test/globalTeardown.js
```

Then run the tests:

```bash
yarn test
```

## Step 4

Propose a software upgrade on the v1.5 chain.

```bash
# 30 blocks (30 minutes) until upgrade block
UPGRADE_BLOCK="$(docker exec node bash -c 'secretd status | jq "(.SyncInfo.latest_block_height | tonumber) + 30"')"

# Propose upgrade
PROPOSAL_ID="$(docker exec node bash -c "secretd tx gov submit-proposal software-upgrade v1.6 --upgrade-height $UPGRADE_BLOCK --title blabla --description yolo --deposit 100000000uscrt --from a -y -b block | jq '.logs[0].events[] | select(.type == \"submit_proposal\") | .attributes[] | select(.key == \"proposal_id\") | .value | tonumber'")"

# Vote yes (voting period is 90 seconds)
docker exec node bash -c "secretd tx gov vote ${PROPOSAL_ID} yes --from a -y -b block"

echo "PROPOSAL_ID   = ${PROPOSAL_ID}"
echo "UPGRADE_BLOCK = ${UPGRADE_BLOCK}"
```

## Step 5

Apply the upgrade.

Wait until you see `ERR CONSENSUS FAILURE!!! err="UPGRADE \"v1.6\" NEEDED at height` in BOTH of the logs, then run:

Copy binaries from v1.6 chain to v1.5 chain.

```bash
# Start a v1.6 chain and wait a bit for it to setup
SGX_MODE=SW make build_linux

# Copy binaries from host to current v1.5 chain

docker exec bootstrap bash -c 'rm -rf /tmp/upgrade-bin && mkdir -p /tmp/upgrade-bin'
docker exec node bash -c 'rm -rf /tmp/upgrade-bin && mkdir -p /tmp/upgrade-bin'

docker cp secretd                                                  bootstrap:/tmp/upgrade-bin
docker cp go-cosmwasm/librust_cosmwasm_enclave.signed.so           bootstrap:/tmp/upgrade-bin
docker cp go-cosmwasm/api/libgo_cosmwasm.so                        bootstrap:/tmp/upgrade-bin
docker cp secretd                                                  node:/tmp/upgrade-bin
docker cp go-cosmwasm/librust_cosmwasm_enclave.signed.so           node:/tmp/upgrade-bin
docker cp go-cosmwasm/api/libgo_cosmwasm.so                        node:/tmp/upgrade-bin

docker exec node bash -c 'pkill -9 secretd'

docker exec bootstrap bash -c 'cat /tmp/upgrade-bin/librust_cosmwasm_enclave.signed.so       > /usr/lib/librust_cosmwasm_enclave.signed.so'
docker exec bootstrap bash -c 'cat /tmp/upgrade-bin/libgo_cosmwasm.so                        > /usr/lib/libgo_cosmwasm.so'
docker exec node bash -c 'cat /tmp/upgrade-bin/secretd                                  > /usr/bin/secretd'
docker exec node bash -c 'cat /tmp/upgrade-bin/librust_cosmwasm_enclave.signed.so       > /usr/lib/librust_cosmwasm_enclave.signed.so'
docker exec node bash -c 'cat /tmp/upgrade-bin/libgo_cosmwasm.so                        > /usr/lib/libgo_cosmwasm.so'

docker cp bootstrap:/root/.secretd/config/priv_validator_key.json /tmp/upgrade-bin/.
docker cp /tmp/upgrade-bin/priv_validator_key.json node:/root/.secretd/config/priv_validator_key.json

docker exec node bash -c 'source /opt/sgxsdk/environment && RUST_BACKTRACE=1 LOG_LEVEL="trace" secretd start --rpc.laddr tcp://0.0.0.0:26657'

```

You should see `INF applying upgrade "v1.6" at height` in the logs, following by blocks continute to stream.

## Step 6 (Test seed rotation)

### Query the value of the counter

```bash
secretd query compute query secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg '{"get": {}}'
```

Expected result should be:
{"get":{"count":23}}

## Step 7

Test that now cw20-ics20 is working:

TODO
