#!/bin/bash

set -euvx

function wait_for_tx () {
    until (secretd q tx "$1" --output json)
    do
        echo "$2"
        sleep 1
    done
}

# init the node
rm -rf /opt/secret/.sgx_secrets *.der ~/*.der
mkdir -p /opt/secret/.sgx_secrets

rm -rf ~/.secretd

#export SECRET_NETWORK_CHAIN_ID=secretdev-1
#export SECRET_NETWORK_KEYRING_BACKEND=test
secretd config keyring-backend test
secretd config chain-id secretdev-1
secretd config output json

secretd init banana --chain-id secretdev-1
perl -i -pe 's/"stake"/"uscrt"/g' ~/.secretd/config/genesis.json
echo "cost member exercise evoke isolate gift cattle move bundle assume spell face balance lesson resemble orange bench surge now unhappy potato dress number acid" |
    secretd keys add a --recover --keyring-backend test
secretd add-genesis-account "$(secretd keys show -a --keyring-backend test a)" 1000000000000uscrt
secretd gentx a 1000000uscrt --chain-id secretdev-1 --keyring-backend test
secretd collect-gentxs
secretd validate-genesis

secretd init-bootstrap node-master-cert.der io-master-cert.der
secretd validate-genesis

RUST_BACKTRACE=1 secretd start --bootstrap --log_level error &


export SECRETD_PID=$(echo $!)


until (secretd status 2>&1 | jq -e '(.SyncInfo.latest_block_height | tonumber) > 0' &>/dev/null); do
    echo "Waiting for chain to start..."
    sleep 1
done

function cleanup() {
    kill -KILL "$SECRETD_PID"
}
trap cleanup EXIT ERR

# store wasm code on-chain so we could later instansiate it
export STORE_TX_HASH=$(
    secretd tx compute store ./x/compute/internal/keeper/testdata/test-contract/contract.wasm --from a --gas 10000000 --gas-prices 0.25uscrt --output json -y |
        jq -r .txhash
)

wait_for_tx "$STORE_TX_HASH" "Waiting for store to finish on-chain..."

# test storing of wasm code (this doesn't touch sgx yet)
secretd q tx "$STORE_TX_HASH" --output json |
    jq -e '.logs[].events[].attributes[] | select(.key == "code_id" and .value == "1")'

# init the contract (ocall_init + write_db + canonicalize_address)
export INIT_TX_HASH=$(
    secretd tx compute instantiate 1 '{"nop":{}}' --label baaaaaaa --from a --gas-prices 0.25uscrt -y --output json |
        jq -r .txhash
)

wait_for_tx "$INIT_TX_HASH" "Waiting for instantiate to finish on-chain..."

secretd q compute tx "$INIT_TX_HASH" --output json

export CONTRACT_ADDRESS=$(
    secretd q tx "$INIT_TX_HASH" --output json |
        jq -er '.logs[].events[].attributes[] | select(.key == "contract_address") | .value' |
        head -1
)

# exec (generate callbacks)
export EXEC_TX_HASH=$(
    secretd tx compute execute --from a $CONTRACT_ADDRESS "{\"a\":{\"contract_addr\":\"$CONTRACT_ADDRESS\",\"x\":2,\"y\":3}}" -y --gas-prices 0.25uscrt --output json |
        jq -r .txhash
)

wait_for_tx "$EXEC_TX_HASH" "Waiting for exec to finish on-chain..."

secretd q compute tx "$EXEC_TX_HASH"

# exec (generate error inside WASM)
export EXEC_ERR_TX_HASH=$(
    secretd tx compute execute --from a $CONTRACT_ADDRESS "{\"contract_error\":{\"error_type\":\"generic_err\"}}" -y --gas-prices 0.25uscrt --output json |
        jq -r .txhash
)

wait_for_tx "$EXEC_ERR_TX_HASH" "Waiting for exec to finish on-chain..."

secretd q compute tx "$EXEC_ERR_TX_HASH"

# exec (generate error inside WASM)
export EXEC_ERR_TX_HASH=$(
    secretd tx compute execute --from a $CONTRACT_ADDRESS '{"allocate_on_heap":{"bytes":1073741824}}' -y --gas-prices 0.25uscrt --output json |
        jq -r .txhash
)

wait_for_tx "$EXEC_ERR_TX_HASH" "Waiting for exec to finish on-chain..."

secretd q compute tx "$EXEC_ERR_TX_HASH"

# test output data decryption
secretd tx compute execute --from a "$CONTRACT_ADDRESS" '{"unicode_data":{}}' -b block -y --gas-prices 0.25uscrt --output json |
    jq -r .txhash |
    xargs secretd q compute tx

# sleep infinity

(
    cd ./cosmwasm-js
    yarn
    cd ./packages/sdk
    yarn build
)

node ./cosmwasm/testing/callback-test.js
