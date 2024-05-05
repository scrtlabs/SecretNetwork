#!/bin/bash

SECRETCLI=${1:-./secretcli}
SECRETD_HOME=${2:-$HOME/.secretd_local}
CHAINID=${3:-secretdev-1}
SECRETD=${4:-"http://localhost:26657"}

if ! [ -f $SECRETD_HOME/config/genesis.json ]; then
  echo "Cannot find $SECRETD_HOME/config/genesis.json."
  exit 1
fi

set -x
set -o errexit

$SECRETCLI config set client chain-id "$CHAINID"
$SECRETCLI config set client output json
$SECRETCLI config set client keyring-backend test
$SECRETCLI config set client node $SECRETD

$SECRETCLI status --output=json | js

$SECRETCLI keys list --keyring-backend="test" --home=$SECRETD_HOME --output=json | jq

address_v=$($SECRETCLI keys show -a validator --keyring-backend="test" --home=$SECRETD_HOME)

address_a=$($SECRETCLI keys show -a a --keyring-backend="test" --home=$SECRETD_HOME)
address_b=$($SECRETCLI keys show -a b --keyring-backend="test" --home=$SECRETD_HOME)
address_c=$($SECRETCLI keys show -a c --keyring-backend="test" --home=$SECRETD_HOME)
address_d=$($SECRETCLI keys show -a d --keyring-backend="test" --home=$SECRETD_HOME)

key_a=$($SECRETCLI keys show -p a --keyring-backend="test" --home=$SECRETD_HOME)
key_b=$($SECRETCLI keys show -p b --keyring-backend="test" --home=$SECRETD_HOME)
key_c=$($SECRETCLI keys show -p c --keyring-backend="test" --home=$SECRETD_HOME)
key_d=$($SECRETCLI keys show -p d --keyring-backend="test" --home=$SECRETD_HOME)

$SECRETCLI q bank balances $address_a --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_b --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_c --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_d --home=$SECRETD_HOME --output=json | jq

# $SECRETCLI tx bank send $address_a $address_b 10uscrt --gas=auto --gas-adjustment=1.0 --chain-id=$CHAINID --home=$SECRETD_HOME --keyring-backend="test" --output=json | jq

txhash=$($SECRETCLI tx bank send $address_a $address_b 10uscrt --gas-prices=0.25uscrt -y --chain-id=$CHAINID --home=$SECRETD_HOME --keyring-backend="test" --output=json | jq ".txhash" | sed 's/"//g')
echo "FIX: $SECRETCLI q tx --type="hash" "$txhash" --home=$SECRETD_HOME"
$SECRETCLI q bank balances $address_a --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_b --home=$SECRETD_HOME --output=json | jq

txhash=$($SECRETCLI tx bank send $address_b $address_c 10uscrt --gas-prices=0.25uscrt -y --chain-id=$CHAINID --home=$SECRETD_HOME --keyring-backend="test" --output=json | jq ".txhash" | sed 's/"//g')
echo "FIX: $SECRETCLI q tx --type="hash" "$txhash" --home=$SECRETD_HOME"
$SECRETCLI q bank balances $address_b --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_c --home=$SECRETD_HOME --output=json | jq

txhash=$($SECRETCLI tx bank send $address_c $address_d 10uscrt --gas-prices=0.25uscrt -y --chain-id=$CHAINID --home=$SECRETD_HOME --keyring-backend="test" --output=json | jq ".txhash" | sed 's/"//g')
echo "FIX: $SECRETCLI q tx --type="hash" "$txhash" --home=$SECRETD_HOME"
$SECRETCLI q bank balances $address_c --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_d --home=$SECRETD_HOME --output=json | jq

txhash=$($SECRETCLI tx bank send $address_d $address_a 10uscrt --gas-prices=0.25uscrt -y --chain-id=$CHAINID --home=$SECRETD_HOME --keyring-backend="test" --output=json | jq ".txhash" | sed 's/"//g')
echo "FIX: $SECRETCLI q tx --type="hash" "$txhash" --home=$SECRETD_HOME"
$SECRETCLI q bank balances $address_d --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_a --home=$SECRETD_HOME --output=json | jq


$SECRETCLI q distribution params --output=json | jq

$SECRETCLI q distribution community-pool --output=json | jq

address_valop=$(jq '.app_state.genutil.gen_txs[0].body.messages[0].validator_address' $SECRETD_HOME/config/genesis.json)

if [[ -z $address_valop ]];then
    echo "No GENESIS tx in genesis.json"
    exit 1
fi

address_valop=$(echo $address_valop | sed 's/"//g')
$SECRETCLI q distribution validator-outstanding-rewards $address_valop --output=json | jq

$SECRETCLI q distribution commission $address_valop --output=json | jq

echo "FIXME: get realistic height"
$SECRETCLI q distribution slashes $address_valop "1" "10" --output=json | jq

DIR=$(pwd)
WORK_DIR=$(mktemp -d -p ${DIR})
if [ ! -d $WORK_DIR ]; then
  echo "Could not create $WORK_DIR"
  exit 1
fi

function cleanup {
    echo "Clean up $WORK_DIR"
    #rm -rf "$WORK_DIR"
}

trap cleanup EXIT


cd $WORK_DIR

cargo generate --git https://github.com/scrtlabs/secret-template.git --name secret-test-contract 

cd secret-test-contract
make build

if [[ ! contract.wasm.gz ]];then
    echo "failed to build a test contract"
    cd -
    return 1
fi

cd $DIR
echo "FIXME: add deploy contract"

$SECRETCLI keys add scrt_smart_contract -y --keyring-backend="test" --home=$SECRETD_HOME --output=json | jq
address_scrt=$($SECRETCLI keys show -a scrt_smart_contract --keyring-backend="test" --home=$SECRETD_HOME)
$SECRETCLI q bank balances $address_scrt --home=$SECRETD_HOME --output=json | jq
#curl http://localhost:5000/faucet?address=$address_scrt
txhash=$($SECRETCLI tx bank send $address_a $address_scrt 100000uscrt --gas-prices=0.25uscrt -y --chain-id=$CHAINID --home=$SECRETD_HOME --keyring-backend="test" --output=json | jq ".txhash" | sed 's/"//g')
$SECRETCLI q bank balances $address_scrt --home=$SECRETD_HOME --output=json | jq

$SECRETCLI tx compute store $WORK_DIR/secret-test-contract/contract.wasm -y --gas 50000 --from $address_scrt --chain-id=$CHAINID --keyring-backend="test" --home=$SECRETD_HOME --output=json | jq

$SECRETCLI q compute list-code --home=$SECRETD_HOME