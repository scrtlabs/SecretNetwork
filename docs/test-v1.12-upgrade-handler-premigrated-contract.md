# How to test the v1.12 upgrade with LocalSecret

## Step 1

### Start a v1.11 chain

```bash
docker run -p 1316:1317 -p 26657 -it --name localsecret ghcr.io/scrtlabs/localsecret:v1.11.0-beta.19
```

## Step 2

### Copy the suplied contract to the docker

```bash
docker cp ./contract.wasm localsecret:/root/
```

### Instantiate a contract with admin and get its address

```bash
docker exec localsecret bash -c 'secretcli tx wasm store contract.wasm --from a --gas 5000000 -y -b block'
docker exec localsecret bash -c 'secretcli tx wasm init 1 "{\"nop\":{}}" --from a --label "xyz" -y -b block --admin secret1ap26qrlp8mcq2pg6r47w43l0y8zkqm8a450s03'
docker exec localsecret bash -c 'secretcli q wasm list-contract-by-code 1 | jq -r ".[0].contract_address"'
```
Expected result should be: `secret1mfk7n6mc2cg6lznujmeckdh4x0a5ezf6hx6y8q`

### Instantiate a second contract and get its address

```bash
docker exec localsecret bash -c 'secretcli tx wasm init 1 "{\"nop\":{}}" --from a --label "premigrated" -y -b block --admin secret1ap26qrlp8mcq2pg6r47w43l0y8zkqm8a450s03'
docker exec localsecret bash -c 'secretcli q wasm list-contract-by-code 1 | jq -r ".[1].contract_address"'
```
Expected result should be: `secret18wy2w4rzg9xxsm2ru8jq8tdq053h39epxvd4rl`


### Check that you're the admin

On the first contract:
```bash
docker exec localsecret bash -c 'secretcli q wasm contract secret1mfk7n6mc2cg6lznujmeckdh4x0a5ezf6hx6y8q | jq -r .admin'
```
Expected result should be: `secret1ap26qrlp8mcq2pg6r47w43l0y8zkqm8a450s03`

On the second contract:
```bash
docker exec localsecret bash -c 'secretcli q wasm contract secret18wy2w4rzg9xxsm2ru8jq8tdq053h39epxvd4rl | jq -r .admin'
```
Expected result should be: `secret1ap26qrlp8mcq2pg6r47w43l0y8zkqm8a450s03`

## Step 3
### Migrate the first contract on the v11 chain
```bash
docker cp ./migrate_contract_v2.wasm localsecret:/root/
docker exec localsecret bash -c 'secretcli tx wasm store migrate_contract_v2.wasm --from a --gas 5000000 -y -b block'
docker exec localsecret bash -c 'secretcli tx wasm migrate secret1mfk7n6mc2cg6lznujmeckdh4x0a5ezf6hx6y8q 2 "{\"migrate\":{}}" --from a -y -b block' | jq -r .code
```

Expected result should be: `0`

### Check that you CAN'T query and execute the contract
Query:
```bash
docker exec localsecret bash -c 'secretcli q wasm query secret1mfk7n6mc2cg6lznujmeckdh4x0a5ezf6hx6y8q "{\"get_counter\":{}}"'
```

You should see an error:
```
Error: got an error finding base64 of the error: regexMatch '[]' should have a length of 2. error: rpc error: code = Unauthenticated desc = Execution error: Enclave: calling a function in the contract failed for an unexpected reason: query contract failed
```

Execute:
```bash
docker exec localsecret bash -c 'secretcli tx wasm execute secret1mfk7n6mc2cg6lznujmeckdh4x0a5ezf6hx6y8q {\"increment\":{}} --from a -y -b block'
```

You should see an error:
```
... "raw_log": "failed to execute message; message index: 0: Execution error: Enclave: the contract panicked: execute contract failed" ...
```

## Step 4
### Compile a docker with version 12 of the network
Compile a v1.12 LocalSecret 
```bash
DOCKER_TAG=v1.12-local make localsecret
```
Alternatively, to save compilation, you can use this one: `http://ghcr.io/scrtlabs/localsecret:v1.12.0-eshel.1`

## Step 5
Copy binaries from the v1.12 LocalSecret to the running v1.11 LocalSecret.

```bash
# Start a v1.12 chain and wait a bit for it to setup
docker run -it -d --name localsecret-1.12 ghcr.io/scrtlabs/localsecret:v1.12.0-eshel.1 
# or: docker run -it -d --name localsecret-1.12 ghcr.io/scrtlabs/localsecret:v1.12.0-eshel.1 
sleep 5

# Copy binaries from v1.12 chain to host (a limitation of `docker cp`)
rm -rf /tmp/upgrade-bin && mkdir -p /tmp/upgrade-bin
docker cp localsecret-1.12:/usr/bin/secretcli                              /tmp/upgrade-bin
docker cp localsecret-1.12:/usr/bin/secretd                                /tmp/upgrade-bin
docker cp localsecret-1.12:/usr/lib/librust_cosmwasm_enclave.signed.so     /tmp/upgrade-bin
docker cp localsecret-1.12:/usr/lib/libgo_cosmwasm.so                      /tmp/upgrade-bin

# Can kill localsecret-1.12 at this point
docker rm -f localsecret-1.12

# Copy binaries from host to current v1.11 chain
docker exec localsecret bash -c 'rm -rf /tmp/upgrade-bin && mkdir -p /tmp/upgrade-bin'

docker cp /tmp/upgrade-bin/secretcli                                localsecret:/tmp/upgrade-bin
docker cp /tmp/upgrade-bin/secretd                                  localsecret:/tmp/upgrade-bin
docker cp /tmp/upgrade-bin/librust_cosmwasm_enclave.signed.so       localsecret:/tmp/upgrade-bin
docker cp /tmp/upgrade-bin/libgo_cosmwasm.so                        localsecret:/tmp/upgrade-bin

# Overwrite v1.11 binaries with v1.12 binaries without affecting file permissions
# v1.11 chain is still running at this point
# we assume v1.11 binaries are loaded to RAM
# so overwriting them with v1.12 binaries won't take effect until a process restart

docker exec localsecret bash -c 'cat /tmp/upgrade-bin/secretcli                             > /usr/bin/secretcli'
docker exec localsecret bash -c 'cat /tmp/upgrade-bin/librust_cosmwasm_enclave.signed.so    > /usr/lib/librust_cosmwasm_enclave.signed.so'
docker exec localsecret bash -c 'cat /tmp/upgrade-bin/libgo_cosmwasm.so                     > /usr/lib/libgo_cosmwasm.so'

# We cannot overwrite secretd because it's being used ("Text file busy")
# so instead we're going to point the init script to the new binary

# don't setup secretcli
docker exec localsecret bash -c $'perl -i -pe \'s/^.*?secretcli.*$//\' bootstrap_init.sh'

# point script to the v1.12 secretd file
docker exec localsecret bash -c $'perl -i -pe \'s;secretd start;/tmp/upgrade-bin/secretd start;\' bootstrap_init.sh'
```

## Step 6

Propose a software upgrade on the v1.11 chain.

```bash
# 20 blocks (2 minutes) until upgrade block
UPGRADE_BLOCK="$(docker exec localsecret bash -c 'secretcli status | jq "(.SyncInfo.latest_block_height | tonumber) + 20"')"

# Propose upgrade
PROPOSAL_ID="$(docker exec localsecret bash -c "secretcli tx gov submit-proposal software-upgrade v1.12 --upgrade-height $UPGRADE_BLOCK --title blabla --description yolo --deposit 100000000uscrt --from a -y -b block | jq '.logs[0].events[] | select(.type == \"submit_proposal\") | .attributes[] | select(.key == \"proposal_id\") | .value | tonumber'")"

# Vote yes (voting period is 90 seconds)
docker exec localsecret bash -c "secretcli tx gov vote ${PROPOSAL_ID} yes --from a -y -b block"

echo "PROPOSAL_ID   = ${PROPOSAL_ID}"
echo "UPGRADE_BLOCK = ${UPGRADE_BLOCK}"
```

## Step 7

Apply the upgrade.

Wait until you see `ERR CONSENSUS FAILURE!!! err="UPGRADE \"v1.12\" NEEDED at height` in the logs, then run:

```bash
docker stop localsecret
docker start localsecret -a
```

You should see `INF applying upgrade "v1.12" at height` in the logs, followed by blocks continuing to stream.

## Step 8

### Check that the admin is now set to the hardcoded value

```bash
docker exec localsecret bash -c 'secretcli q wasm contract secret1mfk7n6mc2cg6lznujmeckdh4x0a5ezf6hx6y8q | jq -r .admin'
```

Expected result should be: `secret1ap26qrlp8mcq2pg6r47w43l0y8zkqm8a450s03`

### Upgrade the contract

```bash
docker cp ./contract-with-migrate.wasm.gz localsecret:/root/
docker exec localsecret bash -c 'secretcli tx wasm store contract-with-migrate.wasm.gz --from a --gas 5000000 -y -b block'
docker exec localsecret bash -c 'secretcli tx wasm migrate secret1mfk7n6mc2cg6lznujmeckdh4x0a5ezf6hx6y8q 2 "{\"migrate\":{}}" --from a -y -b block' | jq -r .code
```

Expected result should be: `0`

### Check the contract history

```bash
docker exec localsecret bash -c 'secretcli q wasm contract-history secret1mfk7n6mc2cg6lznujmeckdh4x0a5ezf6hx6y8q' | jq
```

Expected result should look like this:

```json
{
  "entries": [
    {
      "operation": "CONTRACT_CODE_HISTORY_OPERATION_TYPE_INIT",
      "code_id": "1",
      "updated": {
        "block_height": "6",
        "tx_index": "0"
      },
      "msg": "/307FYU9h9g96KS4Mz9jEarU+2a71zcm3WMgx+0Gmm6gCbZNrWqp6+IIdiaiZzzhNkC9C7jFAMewHrtCcYfCY5XlqRJku7TPYYlr5K2rHctP7QLXMk1VMeh5zXR9S2rrX5DJxIb1uTElFHhqBnfPQl004eHxmvFblWmtGJVIpoRzjqU7yokrCYEJK6d1i876QHhilFRPAIW/3A=="
    },
    {
      "operation": "CONTRACT_CODE_HISTORY_OPERATION_TYPE_MIGRATE",
      "code_id": "2",
      "updated": {
        "block_height": "243",
        "tx_index": "0"
      },
      "msg": "0eoOpwuUcXE6u05CVy7o5CPln5L/c/uyh1qEOKkYyoigCbZNrWqp6+IIdiaiZzzhNkC9C7jFAMewHrtCcYfCY+PsNVSw+7DTG9zXdU2ZINEW+EN4IjDXPqnZF5shanRnFJ6oRLt7K6Jel8nB36/fyAdkZfeQK+6PT6eOT40Gp6HRYi7jh85Yh0CJVUL2kO6fVBP1dpg6QAJAtw=="
    }
  ]
}
```
