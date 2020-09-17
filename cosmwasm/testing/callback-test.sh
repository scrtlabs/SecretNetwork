#!/bin/bash

set -euvx

function wait_for_tx() {
    until (./secretcli q tx "$1"); do
        echo "$2"
        sleep 1
    done
}

# init the node
rm -rf ./.sgx_secrets
mkdir -p ./.sgx_secrets

rm -rf ~/.secret*

./secretcli config chain-id enigma-testnet
./secretcli config output json
./secretcli config indent true
./secretcli config trust-node true
./secretcli config keyring-backend test

./secretd init banana --chain-id enigma-testnet
perl -i -pe 's/"stake"/"uscrt"/g' ~/.secretd/config/genesis.json
echo "cost member exercise evoke isolate gift cattle move bundle assume spell face balance lesson resemble orange bench surge now unhappy potato dress number acid" |
    ./secretcli keys add a --recover
./secretd add-genesis-account "$(./secretcli keys show -a a)" 1000000000000uscrt
./secretd gentx --name a --keyring-backend test --amount 1000000uscrt
./secretd collect-gentxs
./secretd validate-genesis

./secretd init-bootstrap ./node-master-cert.der ./io-master-cert.der

./secretd validate-genesis

RUST_BACKTRACE=1 ./secretd start --bootstrap &

export secretd_PID=$(echo $!)

until (./secretcli status 2>&1 | jq -e '(.sync_info.latest_block_height | tonumber) > 0' &>/dev/null); do
    echo "Waiting for chain to start..."
    sleep 1
done

./secretcli rest-server --chain-id enigma-testnet --laddr tcp://0.0.0.0:1337 &
export LCD_PID=$(echo $!)
function cleanup() {
    kill -KILL "$secretd_PID" "$LCD_PID"
}
trap cleanup EXIT ERR

export STORE_TX_HASH=$(
    yes |
        ./secretcli tx compute store ./x/compute/internal/keeper/testdata/test-contract/contract.wasm --from a --gas 10000000 |
        jq -r .txhash
)

wait_for_tx "$STORE_TX_HASH" "Waiting for store to finish on-chain..."

# test storing of wasm code (this doesn't touch sgx yet)
./secretcli q tx "$STORE_TX_HASH" |
    jq -e '.logs[].events[].attributes[] | select(.key == "code_id" and .value == "1")'

# init the contract (ocall_init + write_db + canonicalize_address)
export INIT_TX_HASH=$(
    yes |
        ./secretcli tx compute instantiate 1 '{"nop":{}}' --label baaaaaaa --from a |
        jq -r .txhash
)

wait_for_tx "$INIT_TX_HASH" "Waiting for instantiate to finish on-chain..."

./secretcli q compute tx "$INIT_TX_HASH"

export CONTRACT_ADDRESS=$(
    ./secretcli q tx "$INIT_TX_HASH" |
        jq -er '.logs[].events[].attributes[] | select(.key == "contract_address") | .value' | head -1
)

# exec (generate callbacks)
export EXEC_TX_HASH=$(
    yes |
        ./secretcli tx compute execute --from a $CONTRACT_ADDRESS "{\"a\":{\"contract_addr\":\"$CONTRACT_ADDRESS\",\"x\":2,\"y\":3}}" |
        jq -r .txhash
)

wait_for_tx "$EXEC_TX_HASH" "Waiting for exec to finish on-chain..."

./secretcli q compute tx "$EXEC_TX_HASH"

# exec (generate error inside WASM)
export EXEC_ERR_TX_HASH=$(
    yes |
        ./secretcli tx compute execute --from a $CONTRACT_ADDRESS "{\"contract_error\":{\"error_type\":\"generic_err\"}}" |
        jq -r .txhash
)

wait_for_tx "$EXEC_ERR_TX_HASH" "Waiting for exec to finish on-chain..."

./secretcli q compute tx "$EXEC_ERR_TX_HASH"

# exec (generate error inside WASM)
export EXEC_ERR_TX_HASH=$(
    yes |
        ./secretcli tx compute execute --from a $CONTRACT_ADDRESS '{"allocate_on_heap":{"bytes":1073741824}}' |
        jq -r .txhash
)

wait_for_tx "$EXEC_ERR_TX_HASH" "Waiting for exec to finish on-chain..."

./secretcli q compute tx "$EXEC_ERR_TX_HASH"
# test output data decryption
yes |
    ./secretcli tx compute execute --from a "$CONTRACT_ADDRESS" '{"unicode_data":{}}' -b block |
    jq -r .txhash |
    xargs ./secretcli q compute tx

# sleep infinity

(
    cd ./cosmwasm-js
    yarn
    cd ./packages/sdk
    yarn build
)

node ./cosmwasm/testing/callback-test.js
