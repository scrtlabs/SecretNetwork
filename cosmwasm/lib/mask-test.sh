#!/bin/bash

set -euvx

function wait_for_tx () {
    until (./enigmacli q tx "$1")
    do
        echo "$2"
        sleep 1
    done
}

# init the node
rm -rf ./.sgx_secrets
mkdir -p ./.sgx_secrets

rm -rf ~/.enigma*
./enigmacli config chain-id enigma-testnet
./enigmacli config output json
./enigmacli config indent true
./enigmacli config trust-node true
./enigmacli config keyring-backend test

./enigmad init banana --chain-id enigma-testnet
perl -i -pe 's/"stake"/"uscrt"/g' ~/.enigmad/config/genesis.json
echo "cost member exercise evoke isolate gift cattle move bundle assume spell face balance lesson resemble orange bench surge now unhappy potato dress number acid" |
    ./enigmacli keys add a --recover
./enigmad add-genesis-account "$(./enigmacli keys show -a a)" 1000000000000uscrt
./enigmad gentx --name a --keyring-backend test --amount 1000000uscrt
./enigmad collect-gentxs
./enigmad validate-genesis

./enigmad init-bootstrap ./node-master-cert.der ./io-master-cert.der

./enigmad validate-genesis

RUST_BACKTRACE=1 ./enigmad start --bootstrap &

ENIGMAD_PID=$(echo $!)

until (./enigmacli status 2>&1 | jq -e '(.sync_info.latest_block_height | tonumber) > 0' &> /dev/null)
do
    echo "Waiting for chain to start..."
    sleep 1
done

./enigmacli rest-server --chain-id enigma-testnet --laddr tcp://0.0.0.0:1337 &
LCD_PID=$(echo $!)
function cleanup()
{
    kill -KILL "$ENIGMAD_PID" "$LCD_PID"
}
trap cleanup EXIT ERR

# store wasm code on-chain so we could later instansiate it
wget -O /tmp/contract.wasm https://raw.githubusercontent.com/CosmWasm/cosmwasm-examples/f5ea00a85247abae8f8cbcba301f94ef21c66087/mask/contract.wasm

STORE_TX_HASH=$(
    yes |
    ./enigmacli tx compute store /tmp/contract.wasm --from a --gas 10000000 |
    jq -r .txhash
)

wait_for_tx "$STORE_TX_HASH" "Waiting for store to finish on-chain..."

# test storing of wasm code (this doesn't touch sgx yet)
./enigmacli q tx "$STORE_TX_HASH" |
    jq -e '.logs[].events[].attributes[] | select(.key == "code_id" and .value == "1")'


# init the contract (ocall_init + write_db + canonicalize_address)
INIT_TX_HASH=$(
    yes |
        ./enigmacli tx compute instantiate 1 "{}" --label baaaaaaa --from a |
        jq -r .txhash
)

wait_for_tx "$INIT_TX_HASH" "Waiting for instantiate to finish on-chain..."

export CONTRACT_ADDRESS=$(
    ./enigmacli q tx "$INIT_TX_HASH" |
        jq -er '.logs[].events[].attributes[] | select(.key == "contract_address") | .value'
)

./enigmacli q compute tx "$INIT_TX_HASH"

# reflect (generate callbacks)
REFLECT_TX_HASH=$(
    yes |
        ./enigmacli tx compute execute --from a $CONTRACT_ADDRESS '{"reflectmsg":{"msgs":[{"contract":{"contract_addr":"'$CONTRACT_ADDRESS'","msg":"eyJjb250cmFjdCI6e319Cg=="}}]}}' |
        jq -r .txhash
)

wait_for_tx "$REFLECT_TX_HASH" "Waiting for reflect to finish on-chain..."

./enigmacli q compute tx "$REFLECT_TX_HASH"
