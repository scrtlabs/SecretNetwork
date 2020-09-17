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
./secretcli config chain-id secret-sanity
./secretcli config output json
./secretcli config indent true
./secretcli config trust-node true
./secretcli config keyring-backend test

./secretd init banana --chain-id secret-sanity
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

export SECRETD_PID=$(echo $!)

until (./secretcli status 2>&1 | jq -e '(.sync_info.latest_block_height | tonumber) > 0' &>/dev/null); do
    echo "Waiting for chain to start..."
    sleep 1
done

./secretcli rest-server --chain-id secret-sanity --laddr tcp://0.0.0.0:1337 &
export LCD_PID=$(echo $!)
function cleanup() {
    kill -KILL "$SECRETD_PID" "$LCD_PID"
}
trap cleanup EXIT ERR

# store wasm code on-chain so we could later instansiate it
export STORE_TX_HASH=$(
    yes |
        ./secretcli tx compute store ./x/compute/internal/keeper/testdata/erc20.wasm --from a --gas 10000000 |
        jq -r .txhash
)

wait_for_tx "$STORE_TX_HASH" "Waiting for store to finish on-chain..."

# test storing of wasm code (this doesn't touch sgx yet)
./secretcli q tx "$STORE_TX_HASH" |
    jq -e '.logs[].events[].attributes[] | select(.key == "code_id" and .value == "1")'

# init the contract (ocall_init + write_db + canonicalize_address)
# a is a tendermint address (will be used in transfer: https://github.com/CosmWasm/cosmwasm-examples/blob/f5ea00a85247abae8f8cbcba301f94ef21c66087/erc20/src/contract.rs#L110)
# secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t is just a random address
# balances are set to 108 & 53 at init
export INIT_TX_HASH=$(
    yes |
        ./secretcli tx compute instantiate 1 "{\"decimals\":10,\"initial_balances\":[{\"address\":\"$(./secretcli keys show a -a)\",\"amount\":\"108\"},{\"address\":\"secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t\",\"amount\":\"53\"}],\"name\":\"ReuvenPersonalRustCoin\",\"symbol\":\"RPRC\"}" --label RPRCCoin --from a |
        jq -r .txhash
)

wait_for_tx "$INIT_TX_HASH" "Waiting for instantiate to finish on-chain..."

./secretcli q compute tx "$INIT_TX_HASH"

export CONTRACT_ADDRESS=$(
    ./secretcli q tx "$INIT_TX_HASH" |
        jq -er '.logs[].events[].attributes[] | select(.key == "contract_address") | .value'
)

# test balances after init (ocall_query + read_db + canonicalize_address)
./secretcli q compute query "$CONTRACT_ADDRESS" "{\"balance\":{\"address\":\"$(./secretcli keys show a -a)\"}}" |
    jq -e '.balance == "108"'
./secretcli q compute query "$CONTRACT_ADDRESS" "{\"balance\":{\"address\":\"secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t\"}}" |
    jq -e '.balance == "53"'

# transfer 10 balance (ocall_handle + read_db + write_db + humanize_address + canonicalize_address)
yes |
    ./secretcli tx compute execute --from a "$CONTRACT_ADDRESS" '{"transfer":{"amount":"10","recipient":"secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t"}}' -b block |
    jq -r .txhash |
    xargs ./secretcli q compute tx

# test balances after transfer (ocall_query + read_db)
./secretcli q compute query "$CONTRACT_ADDRESS" "{\"balance\":{\"address\":\"$(./secretcli keys show a -a)\"}}" |
    jq -e '.balance == "98"'
./secretcli q compute query "$CONTRACT_ADDRESS" "{\"balance\":{\"address\":\"secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t\"}}" |
    jq -e '.balance == "63"'

(./secretcli q compute query "$CONTRACT_ADDRESS" "{\"balance\":{\"address\":\"secret1zzzzzzzzzzzzzzzzzz\"}}" || true) 2>&1 | grep -c 'canonicalize_address errored: invalid checksum'

# sleep infinity

(
    cd ./cosmwasm-js
    yarn
    cd ./packages/sdk
    yarn build
)

node ./cosmwasm/testing/cosmwasm-js-test.js

echo "All is done. Yay!"
