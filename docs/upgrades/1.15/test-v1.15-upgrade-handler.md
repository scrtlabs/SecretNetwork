# How to test the v1.15 upgrade with LocalSecret

__NOTE__: Always work in docs/upgrades/1.15 directory

## Step 1

Start a v1.14.0 chain.

```bash
docker compose -f docker-compose-115.yml up -d
```
We use modified node_init.sh script to start a regular node.
Let's copy this script over into the node container
```bash
 docker cp node_init.sh node:/root/
```

(For convenience) Open a new terminal window:

```bash
docker exec -it bootstrap bash
```

, and start a bootstrap instance:
```bash
./bootstrap_init.sh
```

__Note__: If for whatever reason, bootstrap_init.sh fails to start reporting an illigal instraction,
most likely cause is /tmp/secretd left from the previous run. Try to remove and recreate this directory
if the script fails to run, restart the compose.

Open yet another terminal window where you will start a regular node:

```bash
docker exec -it node bash
```

To start a node instance
```bash
chmod 0777 node_init.sh
./node_init.sh
```

## Step 2 (Test basic contract)

Copy a contract to node container:
```bash
docker cp ./contract.wasm node:/root/
```

Shell into the node container:
```bash
docker exec -it node bash
```

Store, instantiate, and execute the contract:
```bash
secretd config node http://0.0.0.0:26657
secretd tx compute store contract.wasm --from a --gas 5000000 -y
sleep 5
INIT='{"counter":{"counter":10, "expires":100000}}'
secretd tx compute instantiate 1 "$INIT" --from a --label "c" -y
sleep 5
ADDR=`secretd q compute list-contract-by-code 1 | jq -r '.[0].contract_address'`

secretd tx compute execute $ADDR '{"increment":{"addition": 13}}' --from a -y
sleep 5
secretd query compute query $ADDR '{"get": {}}'
```

Expected result should be:
```json
{"get":{"count":23}}
```

## Step 3

Propose a software upgrade on the v1.14 chain:
We request an upgrade to take place at +30blocks (@10block/min)

Let's capture the block height:
```bash
UPGRADE_BLOCK="$(docker exec node bash -c 'secretd status | jq "(.SyncInfo.latest_block_height | tonumber) + 30"')"
```

Let's create a proposal and capture its ID:
```bash
# Propose upgrade
PROPOSAL_ID="$(docker exec node bash -c "secretd tx gov submit-proposal software-upgrade v1.15 --upgrade-height $UPGRADE_BLOCK --title blabla --description yolo --deposit 100000000uscrt --from a -y -b block | jq '.logs[0].events[] | select(.type == \"submit_proposal\") | .attributes[] | select(.key == \"proposal_id\") | .value | tonumber'")"
```

Let's vote for the proposal
```bash
# Vote yes (voting period is 90 seconds)
docker exec node bash -c "secretd tx gov vote ${PROPOSAL_ID} yes --from a -y -b block"
```

Optionally, you can check the proposal ID and the upgrade block height:
```bash
echo "PROPOSAL_ID   = ${PROPOSAL_ID}"
echo "UPGRADE_BLOCK = ${UPGRADE_BLOCK}"
```

## Step 4

Perform the upgrade:
At the upgrade block height you will see `ERR CONSENSUS FAILURE!!! err="UPGRADE \"v1.15\" NEEDED at height` for both containers: bootstrap and node.

At this point, the chain is waiting for the upgrade to be performed

__Special note__: If you run Ubuntu 22.04 or higher, you should consider building a localsecret target, which will produce a docker image for your active branch. We assume that you are on branch with tag v1.15.0 or cosmos-sdk-0.50.x-merged as of this writing.

Option A: Either build locally, if you are on Ubuntu 20.04
```bash
FEATURES="light-client-validation,random" SGX_MODE=SW make build-linux
```
Option B: build a docker image if you are on Ubuntu 22.04 or higher
```bash
make localsecret
```

After the build is done, assemble the binary artefacts.
You will need the following binaries:
* secretd
* librust_cosmwasm_enclave.signed.so
* libgo_cosmwasm.so
* librandom_api.so
* tendermint_enclave.signed.so

If you chose option B, you can access the binaries in the container and copy them to a host dir,
e.g. SecretNetwork/docs/upgrades/1.15/bin by running commands:
```bash
docker exec secret-0.50.x:/usr/bin/secretd ./bin
docker exec secret-0.50.x:/usr/lib/librust_cosmwasm_enclave.signed.so ./bin
docker exec secret-0.50.x:/usr/lib/libgo_cosmwasm.so ./bin
docker exec secret-0.50.x:/usr/lib/librandom_api.so ./bin
docker exec secret-0.50.x:/usr/lib/tendermint_enclave.signed.so ./bin

```

```bash
# Copy binaries from host to current v1.14 chain

docker exec bootstrap bash -c 'rm -rf /tmp/upgrade-bin && mkdir -p /tmp/upgrade-bin'
docker exec node bash -c 'rm -rf /tmp/upgrade-bin && mkdir -p /tmp/upgrade-bin'
# update bootstrap
docker cp ./bin/secretd                               bootstrap:/tmp/upgrade-bin
docker cp ./bin/librust_cosmwasm_enclave.signed.so    bootstrap:/tmp/upgrade-bin
docker cp ./bin/libgo_cosmwasm.so                     bootstrap:/tmp/upgrade-bin
# update node
docker cp ./bin/secretd                               node:/tmp/upgrade-bin
docker cp ./bin/librust_cosmwasm_enclave.signed.so    node:/tmp/upgrade-bin
docker cp ./bin/libgo_cosmwasm.so                     node:/tmp/upgrade-bin
docker cp ./bin/librandom_api.so                      node:/tmp/upgrade-bin
docker cp ./bin/tendermint_enclave.signed.so          node:/tmp/upgrade-bin
# stop node's secretd
docker exec node bash -c 'pkill -9 secretd'
# copy over updated binaries
docker exec bootstrap bash -c 'cp /tmp/upgrade-bin/librust_cosmwasm_enclave.signed.so /usr/lib/'
docker exec bootstrap bash -c 'cp /tmp/upgrade-bin/libgo_cosmwasm.so /usr/lib/'
docker exec node bash -c 'cp /tmp/upgrade-bin/secretd /usr/bin/'
docker exec node bash -c 'cp /tmp/upgrade-bin/librust_cosmwasm_enclave.signed.so /usr/lib/'
docker exec node bash -c 'cp /tmp/upgrade-bin/libgo_cosmwasm.so /usr/lib/'

# prepare a tmp dir to store validator's private key
rm -rf /tmp/upgrade-bin && mkdir -p /tmp/upgrade-bin
docker cp bootstrap:/root/.secretd/config/priv_validator_key.json /tmp/upgrade-bin/.
docker cp /tmp/upgrade-bin/priv_validator_key.json node:/root/.secretd/config/priv_validator_key.json
```

Restart node:
```bash
source /opt/sgxsdk/environment && RUST_BACKTRACE=1 LOG_LEVEL="trace" secretd start --rpc.laddr tcp://0.0.0.0:26657
```

The log file will print out `INF applying upgrade "v1.15" at height` 

If the upgrade process was successful, the blockchain will resume generating new blocks after the height at which the upgrade was requested.

Chck that the previously deployed contract is still present
Query the value of the counter:
```bash
secretd query compute query $ADDR '{"get": {}}'
```
Expected result should be:
```json
{"get":{"count":23}}
```
