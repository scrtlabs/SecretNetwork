# How to test the v1.4 upgrade with LocalSecret

## Step 1

Start a v1.3 chain. Port 9091 open for secret.js tests.

```bash
docker run -it -p 9091:9091 --name localsecret ghcr.io/scrtlabs/localsecret:v1.3.1
```

## Step 2

Create a second validator, then double sign, to simulate the CoS double sign.

Note: You can already start steps 3 & 4 while you work on step 2.

Allow multiple node on the same machine:

```bash
docker exec localsecret bash -c 'perl -i -pe "s/allow_duplicate_ip = false/allow_duplicate_ip = true/" .secretd/config/config.toml'
```

Restart LocalSecret:

```bash
docker stop localsecret
docker start localsecret -a
```

In a second terminal:

```bash
# Enter LocalSecret using:
docker exec -it localsecret bash
```

```bash
# Init node & enclave
secretd init --home val2 val2
secretd init-enclave
PUBLIC_KEY=$(secretd parse /opt/secret/.sgx_secrets/attestation_cert.der 2> /dev/null | cut -c 3- )
echo "Public key: $(secretd parse /opt/secret/.sgx_secrets/attestation_cert.der 2> /dev/null | cut -c 3- )"
secretd tx register auth /opt/secret/.sgx_secrets/attestation_cert.der -y --from a --gas-prices 0.25uscrt -b block
SEED=$(secretd q register seed "$PUBLIC_KEY" 2> /dev/null | cut -c 3-)
echo "SEED: $SEED"
secretd q register secret-network-params 2> /dev/null
secretd configure-secret node-master-cert.der "$SEED" --home val2
cp .secretd/config/genesis.json val2/config/genesis.json

# Increse all ports by 10 because the main node is using them
perl -i -pe 's/:(\d+)/":".($1+10)/e' val2/config/app.toml
perl -i -pe 's/:(\d+)/":".($1+10)/e' val2/config/config.toml

# persistent_peers to the main node
perl -i -pe "s/persistent_peers = \".+$/persistent_peers = \"$(secretcli status | jq -r .NodeInfo.id)\@127.0.0.1:26656\"/" val2/config/config.toml

# Use this priv_validator_key to always get cosConsensusAddress = secretvalcons19vjqkmrawv303rkj36wx4qc5vs0krvfu7yaqmt
echo '{
  "address": "2B240B6C7D7322F88ED28E9C6A8314641F61B13C",
  "pub_key": {
    "type": "tendermint/PubKeyEd25519",
    "value": "lXdL8mkXVSlaPiEErdED/DYoSxzeuyEAjcDKenTHBDg="
  },
  "priv_key": {
    "type": "tendermint/PrivKeyEd25519",
    "value": "V9B+ndNQ+nnw5wKHHWc4JV46kSvRfPalZHl1iCaV4BuVd0vyaRdVKVo+IQSt0QP8NihLHN67IQCNwMp6dMcEOA=="
  }
}' > val2/config/priv_validator_key.json

# Start val2 node
secretd start --home val2
```

In a third terminal:

```bash
# Enter LocalSecret using:
docker exec -it localsecret bash
```

```bash
# Increase stake of main process to 1M
# The double signing node can't be the most powerful otherwise weird stuff will happen
# (The main node will apphash and so is one of the double signing nodes)
secretcli tx staking delegate secretvaloper1ap26qrlp8mcq2pg6r47w43l0y8zkqm8aynpdzc 1000000000000uscrt --from a -y -b block

# Create a validator with 10 stake from val2's validator key
secretd tx staking create-validator \
  --amount=10000000uscrt \
  --pubkey=$(secretd --home val2 tendermint show-validator) \
  --details="Gonna double sign so fast!" \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --min-self-delegation="1" \
  --moniker=val2 \
  --from=b -y -b block

# Copy directory from val2 to val3, including priv_validator_key.json
cp -r val{2,3}
# Remove node_key.json otherwise the other peers will reject us as duplicate
rm val3/config/node_key.json
# Remove data, let it sync from 0 (we copied data from a running node, might be corrupt)
rm -rf val3/data/*
# Copy priv_validator_state.json just because the file has to be present
# Once the node syncs it'll also sync priv_validator_state.json & start to double sign
cp val{2,3}/data/priv_validator_state.json

# Increse all ports by 10 because val2 node is using them
perl -i -pe 's/:(\d+)/":".($1+10)/e' val3/config/app.toml
perl -i -pe 's/:(\d+)/":".($1+10)/e' val3/config/config.toml

# persistent_peers to the main node
# both double sign nodes should point to the main node
# for example if main<-val2<-val3 then val2 will filter out double signs coming from val3
perl -i -pe "s/^persistent_peers = \".+$/persistent_peers = \"$(secretcli status | jq -r .NodeInfo.id)\@127.0.0.1:26656\"/" val3/config/config.toml

# Start val3 node
secretd start --home val3
```

This should tombstone the second validator. On the first validator you should see `INF verified new evidence of byzantine behavior`. This might take a few seconds. If it take more than 30 seconds, try to stop then start val2 or val3.

To verify that val2 is jailed, this should return `true`:

```bash
docker exec -it localsecret bash -c 'secretcli q staking validator secretvaloper1fc3fzy78ttp0lwuujw7e52rhspxn8uj5m98e7s | jq .jailed'
```

## Step 3

Run the secret.js tests from the `master` branch on the `secret.js` repo.  
This will create state on the chain before the upgrade.

First delete globalSetup & globalTeardown (because we already launched the chain manually):

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

Compile a v1.4 chain just to extract the binaries from it.

in `app/upgrades/v1.4/cos_patch.go` use these (from val2):

```diff
- var (
-	cosValidatorAddress = "secretvaloper1hscf4cjrhzsea5an5smt4z9aezhh4sf5jjrqka"
-	cosConsensusAddress = "secretvalcons1rd5gs24he44ufnwawshu3u73lh33cx5z7npzre"
- )
+ // TESTNET DONT COMMIT!!!!
+ var (
+	/* TESTNET DONT COMMIT!!!! */ cosValidatorAddress = /* TESTNET DONT COMMIT!!!! */ "secretvaloper1fc3fzy78ttp0lwuujw7e52rhspxn8uj5m98e7s" /* TESTNET DONT COMMIT!!!! */
+	/* TESTNET DONT COMMIT!!!! */ cosConsensusAddress = /* TESTNET DONT COMMIT!!!! */ "secretvalcons19vjqkmrawv303rkj36wx4qc5vs0krvfu7yaqmt" /* TESTNET DONT COMMIT!!!! */
+ )
+ // TESTNET DONT COMMIT!!!!
```

Compile:

```bash
DOCKER_TAG=v0.0.0 make build-localsecret
```

Run:

```bash
docker run -it --rm --name localsecret-1.4 ghcr.io/scrtlabs/localsecret:v0.0.0
```

## Step 5

Copy binaries from v1.4 chain to v1.3 chain.

```bash
# Copy binaries from v1.4 chain to host (a limitation of `docker cp`)

rm -rf /tmp/upgrade-bin && mkdir -p /tmp/upgrade-bin

docker cp localsecret-1.4:/usr/bin/secretcli                                /tmp/upgrade-bin
docker cp localsecret-1.4:/usr/bin/secretd                                  /tmp/upgrade-bin
docker cp localsecret-1.4:/usr/lib/librust_cosmwasm_enclave.signed.so       /tmp/upgrade-bin
docker cp localsecret-1.4:/usr/lib/libgo_cosmwasm.so                        /tmp/upgrade-bin

# Can kill localsecret-1.4 at this point
docker rm -f localsecret-1.4

# Copy binaries from host to current v1.3 chain

docker exec localsecret bash -c 'rm -rf /tmp/upgrade-bin && mkdir -p /tmp/upgrade-bin'

docker cp /tmp/upgrade-bin/secretcli                                localsecret:/tmp/upgrade-bin
docker cp /tmp/upgrade-bin/secretd                                  localsecret:/tmp/upgrade-bin
docker cp /tmp/upgrade-bin/librust_cosmwasm_enclave.signed.so       localsecret:/tmp/upgrade-bin
docker cp /tmp/upgrade-bin/libgo_cosmwasm.so                        localsecret:/tmp/upgrade-bin

# Overwrite v1.3 binaries with v1.4 binaries without affecting file permissions
# v1.3 chain is still running at this point
# we assume v1.3 binaries are loaded to RAM
# so overwriting them with v1.4 binraies won't take effect until a process restart

docker exec localsecret bash -c 'cat /tmp/upgrade-bin/secretcli                                > /usr/bin/secretcli'
docker exec localsecret bash -c 'cat /tmp/upgrade-bin/librust_cosmwasm_enclave.signed.so       > /usr/lib/librust_cosmwasm_enclave.signed.so'
docker exec localsecret bash -c 'cat /tmp/upgrade-bin/libgo_cosmwasm.so                        > /usr/lib/libgo_cosmwasm.so'

# We cannot overwrite secretd because it's being used ("Text file busy")
# so instead we're going to point the init script to the new binary

# don't setup secretcli
docker exec localsecret bash -c $'perl -i -pe \'s/^.*?secretcli.*$//\' bootstrap_init.sh'

# point script to the v1.4 secretd file
docker exec localsecret bash -c $'perl -i -pe \'s;RUST_BACKTRACE=1 secretd start;RUST_BACKTRACE=1 /tmp/upgrade-bin/secretd start;\' bootstrap_init.sh'
```

## Step 6

Propose a software upgrade on v1.3 chain.

```bash
# 30 blocks (3 minutes) until upgrade block
UPGRADE_BLOCK="$(docker exec localsecret bash -c 'secretcli status | jq "(.SyncInfo.latest_block_height | tonumber) + 30"')"

# Propose upgrade
PROPOSAL_ID="$(docker exec localsecret bash -c "secretcli tx gov submit-proposal software-upgrade v1.4 --upgrade-height $UPGRADE_BLOCK --title 'Shockwave Delta Upgrade' --description YOLO --deposit 100000000uscrt --from a -y -b block | jq '.logs[0].events[] | select(.type == \"submit_proposal\") | .attributes[] | select(.key == \"proposal_id\") | .value | tonumber'")"

# Vote yes (voting period is 90 seconds)
docker exec localsecret bash -c "secretcli tx gov vote ${PROPOSAL_ID} yes --from a -y -b block"

echo "PROPOSAL_ID = ${PROPOSAL_ID}"
echo "UPGRADE_BLOCK = ${UPGRADE_BLOCK}"
```

## Step 7

Apply the upgrade.

Wait until you see `ERR CONSENSUS FAILURE!!! err="UPGRADE \"v1.4\" NEEDED at height` in the logs, then run:

```bash
docker stop localsecret
docker start localsecret -a
```

You should see `INF applying upgrade "v1.4" at height` in the logs, following by blocks continute to stream.

## Step 8

Restart val2 and unjail.

Restart and apply the upgrade:

```bash
docker exec -it localsecret bash -c '/tmp/upgrade-bin/secretd start --home val2'
```

Then in another terminal, unjail:

```bash
# Enter LocalSecret using:
docker exec -it localsecret bash
```

```bash
# Unjail
secretcli tx slashing unjail --from b -y -b block

# This should output "false":
secretcli q staking validators | jq '.validators[] | select(.operator_address == "secretvaloper1fc3fzy78ttp0lwuujw7e52rhspxn8uj5m98e7s") | .jailed'

# This should output "2":
secretcli q block | jq '.block.last_commit.signatures | length'

# This random account should now have 502uscrt delegated to val2:
secretcli q staking delegations secret1hsjtghm83lt8p0ksmpf3ef9rlcfswksm0c87z5 | jq
```

## Step 9

Test that old v0.10 contracts are still working (query + exec + init from stored code).

```bash
# This should output "SSCRT":
secretcli q compute query secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg '{"token_info":{}}' | jq .token_info.symbol

# This should output "0":
secretcli tx compute exec secret18vd8fpwxzck93qlwghaj6arh4p7c5n8978vsyg '{"deposit":{}}' --amount 1uscrt -b block --from a -y | jq .code

# This should output "0":
secretcli tx compute init 1 '{"name":"Secret SCRT","admin":"secret1ap26qrlp8mcq2pg6r47w43l0y8zkqm8a450s03","symbol":"SSCRT","decimals":6,"initial_balances":[],"prng_seed":"eW8=","config":{"public_total_supply":true,"enable_deposit":true,"enable_redeem":true,"enable_mint":false,"enable_burn":false},"supported_denoms":["uscrt"]}' --label "$RANDOM" -b block --from a -y | jq .code
```

## Step 10

Run the integration tests from the `SecretNetwork` repo, without IBC:

```bash
# Skip IBC tests
perl -i -pe 's/describe\("IBC"/describe.skip\("IBC"/' integration-tests/test.ts

# Run integration tests
(cd integration-tests && yarn test)
```
