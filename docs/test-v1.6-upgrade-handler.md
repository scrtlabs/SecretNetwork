# How to test the v1.6 upgrade with LocalSecret

## Step 1

Start a v1.5 chain.

- Port 9091 open for secret.js tests
- Port 26657 open for cw20-ics20 tests

```bash
docker run -it -p 9091:9091 -p 26657:26657 -p 1317:1317 --name localsecret ghcr.io/scrtlabs/localsecret:v1.5.1-patch.3
```

## Step 2

### Copy the suplied contract to the docker

```bash
docker cp ./contract.wasm localsecret:/root/
```

### Access docker

```bash
docker exec -it localsecret bash
```

### Instantiate a contract and interact with him

```bash
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
    const secretjs = await SecretNetworkClient.create({
      grpcWebUrl: "http://localhost:9091",
      chainId: "secretdev-1",
    });

    try {
      const { block } = await secretjs.query.tendermint.getLatestBlock({});

      if (Number(block?.header?.height) >= 1) {
        break;
      }
    } catch (e) {
      // console.error(e);
    }
    await sleep(250);
  }
}' > test/globalSetup.ts
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

Copy binaries from v1.6 chain to v1.5 chain.

```bash
# Start a v1.5 chain and wait a bit for it to setup
docker run -it -d --name localsecret-1.6 ghcr.io/scrtlabs/localsecret:v1.6.0-rc.1
sleep 5

# Copy binaries from v1.5 chain to host (a limitation of `docker cp`)

rm -rf /tmp/upgrade-bin && mkdir -p /tmp/upgrade-bin
docker cp localsecret-1.6:/usr/bin/secretcli                                /tmp/upgrade-bin
docker cp localsecret-1.6:/usr/bin/secretd                                  /tmp/upgrade-bin
docker cp localsecret-1.6:/usr/lib/librust_cosmwasm_enclave.signed.so       /tmp/upgrade-bin
docker cp localsecret-1.6:/usr/lib/libgo_cosmwasm.so                        /tmp/upgrade-bin

# Can kill localsecret-1.5 at this point
docker rm -f localsecret-1.6

# Copy binaries from host to current v1.4 chain

docker exec localsecret bash -c 'rm -rf /tmp/upgrade-bin && mkdir -p /tmp/upgrade-bin'

docker cp /tmp/upgrade-bin/secretcli                                localsecret:/tmp/upgrade-bin
docker cp /tmp/upgrade-bin/secretd                                  localsecret:/tmp/upgrade-bin
docker cp /tmp/upgrade-bin/librust_cosmwasm_enclave.signed.so       localsecret:/tmp/upgrade-bin
docker cp /tmp/upgrade-bin/libgo_cosmwasm.so                        localsecret:/tmp/upgrade-bin

# Overwrite v1.4 binaries with v1.5 binaries without affecting file permissions
# v1.4 chain is still running at this point
# we assume v1.4 binaries are loaded to RAM
# so overwriting them with v1.5 binraies won't take effect until a process restart

docker exec localsecret bash -c 'cat /tmp/upgrade-bin/secretcli                                > /usr/bin/secretcli'
docker exec localsecret bash -c 'cat /tmp/upgrade-bin/librust_cosmwasm_enclave.signed.so       > /usr/lib/librust_cosmwasm_enclave.signed.so'
docker exec localsecret bash -c 'cat /tmp/upgrade-bin/libgo_cosmwasm.so                        > /usr/lib/libgo_cosmwasm.so'

# We cannot overwrite secretd because it's being used ("Text file busy")
# so instead we're going to point the init script to the new binary

# don't setup secretcli
docker exec localsecret bash -c $'perl -i -pe \'s/^.*?secretcli.*$//\' bootstrap_init.sh'

# point script to the v1.5 secretd file
docker exec localsecret bash -c $'perl -i -pe \'s;secretd start;/tmp/upgrade-bin/secretd start;\' bootstrap_init.sh'
```

## Step 5

Propose a software upgrade on the v1.5 chain.

```bash
# 30 blocks (3 minutes) until upgrade block
UPGRADE_BLOCK="$(docker exec localsecret bash -c 'secretcli status | jq "(.SyncInfo.latest_block_height | tonumber) + 30"')"

# Propose upgrade
PROPOSAL_ID="$(docker exec localsecret bash -c "secretcli tx gov submit-proposal software-upgrade v1.6 --upgrade-height $UPGRADE_BLOCK --title blabla --description yolo --deposit 100000000uscrt --from a -y -b block | jq '.logs[0].events[] | select(.type == \"submit_proposal\") | .attributes[] | select(.key == \"proposal_id\") | .value | tonumber'")"

# Vote yes (voting period is 90 seconds)
docker exec localsecret bash -c "secretcli tx gov vote ${PROPOSAL_ID} yes --from a -y -b block"

echo "PROPOSAL_ID   = ${PROPOSAL_ID}"
echo "UPGRADE_BLOCK = ${UPGRADE_BLOCK}"
```

## Step 6

Apply the upgrade.

Wait until you see `ERR CONSENSUS FAILURE!!! err="UPGRADE \"v1.6\" NEEDED at height` in the logs, then run:

```bash
docker stop localsecret
docker start localsecret -a
```

You should see `INF applying upgrade "v1.6" at height` in the logs, following by blocks continute to stream.

## Step 7

### Query the value of the counter

```bash
secretd query compute query secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg '{"get": {}}'
```

Expected result should be:
{"get":{"count":23}}

## Step 8

Test that now cw20-ics20 is working:

TODO
