#!/bin/bash

export SGX_MODE=SW
SECRETCLI=${1:-./secretcli}
SECRETD_HOME=${2:-$HOME/.secretd_local}
CHAINID=${3:-"secretdev-1"}
KEYRING=${4:-"test"}
SECRETD=${5:-"http://localhost:26657"}
SCRT_SGX_STORAGE=/opt/secret/.sgx_secrets
if [ ! $SECRETD_HOME/config/genesis.json ]; then
  echo "Cannot find $SECRETD_HOME/config/genesis.json."
  exit 1
fi

if [ ! -z $BASIC_TEST_DEBUG ]; then
  set -x
else
  set +x
fi
set -o errexit

# -------------------------------------------
THIS=$(readlink -f "${BASH_SOURCE[0]}" 2>/dev/null || echo $0)
DIR=$(dirname "${THIS}")
. "$DIR/integration_test_funcs.sh"

TMP_DIR=$(mktemp -d -p ${DIR})
if [ ! -d $WORK_DIR ]; then
  echo "Could not create $WORK_DIR"
  exit 1
fi

function cleanup {
  echo "Clean up $TMP_DIR"
  rm -rf "$TMP_DIR"
}

trap cleanup EXIT
# ----------------------------------------------


# ----- CLIENT CONFIGURATION - START -----
$SECRETCLI config set client chain-id "$CHAINID"
$SECRETCLI config set client output json
$SECRETCLI config set client keyring-backend test
$SECRETCLI config set client node $SECRETD
# ----- CLIENT CONFIGURATION - END -----

# ----- NODE STATUS CHECK - START -----
$SECRETCLI status --output=json | jq
# ----- NODE STATUS CHECK - END -----

# ------ NODE REGISTRATION - START ------
if [ -f $SCRT_SGX_STORAGE/attectation_cert.der ]; then
  rm ${SCRT_SGX_STORAGE}/attectation_cert.der
fi
./secretd init-enclave
if [ $? -ne 0 ]; then
  echo "Failed to initialize SGX enclave"
  exit 1
fi

if [ ! -f ${SCRT_SGX_STORAGE}/attestation_cert.der ]; then
  echo "Failed to generate attestation_cert.der certificate"
  exit 1
fi

PUBLIC_KEY=$(./secretd parse ${SCRT_SGX_STORAGE}/attestation_cert.der  2> /dev/null | cut -c 3-)
if [ -z $PUBLIC_KEY ]; then
  echo "Failed to parse attestation_cert.der certificate"
  exit 1
fi
echo "Certificate public key: $PUBLIC_KEY"

# On-chain registration and attestation
json_register=$(mktemp -p $TMP_DIR)
./secretd tx register auth ${SCRT_SGX_STORAGE}/attestation_cert.der -y --from a --fees 3000uscrt --keyring-backend ${KEYRING} --home ${SECRETD_HOME} --output json | jq > $json_register
if [ $? -ne 0 ]; then
  echo "Failed to register/auth node"
  exit 1
fi
code_id=$(cat $json_register | jq ".code")
if [[ ${code_id} -ne 0 ]]; then
  echo "Failed to register/auth node. Code: ${code_id}. Error: $(cat $json_register | jq '.raw_log')"
  exit 1
fi
sleep 5s
txhash=$(cat $json_register | jq ".txhash" | tr -d '"')
$SECRETCLI q tx --type=hash "$txhash" --output json | jq > $json_register
code_id=$(cat $json_register | jq ".code")
if [[ ${code_id} -ne 0 ]]; then
  echo "Failed to register/auth node. Error: $(cat $json_register | jq '.raw_log')"
  exit 1
fi

SEED=$(./secretd query register seed $PUBLIC_KEY | cut -c 3-)
if [ -z $SEED ]; then 
  echo "Failed to obtain encrypted seed"
  exit 1
fi
echo "Encrypted seed: $SEED"
sleep 5s
./secretd query register secret-network-params
if [ ! -f ./io-master-key.txt ] || [ ! -f ./node-master-key.txt ]; then
  echo "Failed to generate IO and Node Exch master key"
  exit 1
fi
ls -lh ./io-master-key.txt ./node-master-key.txt

mkdir -p ${SECRETD_HOME}/.node
./secretd configure-secret node-master-key.txt $SEED --home ${SECRETD_HOME}
if [ $? -ne 0 ]; then
  echo "Failed to configure secret node"
  exit 1
fi

# Skip adding persistent peers seeds to config

# Optimize SGX memory for heavy contract calculations (e.g. NFT minting)
sed -i.bak -e "s/^contract-memory-enclave-cache-size *=.*/contract-memory-enclave-cache-size = \"15\"/" ${SECRETD_HOME}/config/app.toml

# Set min gas price
perl -i -pe 's/^minimum-gas-prices = .+?$/minimum-gas-prices = "0.0125uscrt"/' ${SECRETD_HOME}/config/app.toml

NODE_ID=$(./secretd tendermint show-node-id --home ${SECRETD_HOME})
if [ -z $NODE_ID ]; then
  echo "Failed to obtain node id"
  exit 1
fi
echo "Node ID: ${NODE_ID}"
echo "<======= Secret Node registration successful ======>"

# ------ NODE REGISTRATION - END --------

# ----- KEY OPERATIONS - START -----
$SECRETCLI keys list --keyring-backend ${KEYRING} --home=$SECRETD_HOME --output=json | jq

address_v=$($SECRETCLI keys show -a validator --keyring-backend ${KEYRING} --home=$SECRETD_HOME)

address_a=$($SECRETCLI keys show -a a --keyring-backend ${KEYRING} --home=$SECRETD_HOME)
address_b=$($SECRETCLI keys show -a b --keyring-backend ${KEYRING} --home=$SECRETD_HOME)
address_c=$($SECRETCLI keys show -a c --keyring-backend ${KEYRING} --home=$SECRETD_HOME)
address_d=$($SECRETCLI keys show -a d --keyring-backend ${KEYRING} --home=$SECRETD_HOME)

key_a=$($SECRETCLI keys show -p a --keyring-backend ${KEYRING} --home=$SECRETD_HOME)
key_b=$($SECRETCLI keys show -p b --keyring-backend ${KEYRING} --home=$SECRETD_HOME)
key_c=$($SECRETCLI keys show -p c --keyring-backend ${KEYRING} --home=$SECRETD_HOME)
key_d=$($SECRETCLI keys show -p d --keyring-backend ${KEYRING} --home=$SECRETD_HOME)
# ----- KEY OPERATIONS - END -----

# ----- BANK BALANCE TRANSERS - START -----
$SECRETCLI q bank balances $address_a --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_b --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_c --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_d --home=$SECRETD_HOME --output=json | jq

txhash=$($SECRETCLI tx bank send $address_a $address_b 10uscrt --gas-prices=0.25uscrt -y --chain-id=$CHAINID --home=$SECRETD_HOME --keyring-backend ${KEYRING} --output=json | jq ".txhash" | sed 's/"//g')
sleep 5s
$SECRETCLI q tx --type="hash" "$txhash" --output=json | jq
$SECRETCLI q bank balances $address_a --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_b --home=$SECRETD_HOME --output=json | jq

txhash=$($SECRETCLI tx bank send $address_b $address_c 10uscrt --gas-prices=0.25uscrt -y --chain-id=$CHAINID --home=$SECRETD_HOME --keyring-backend ${KEYRING} --output=json | jq ".txhash" | sed 's/"//g')
sleep 5s
$SECRETCLI q tx --type="hash" "$txhash" --output=json | jq
$SECRETCLI q bank balances $address_b --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_c --home=$SECRETD_HOME --output=json | jq

txhash=$($SECRETCLI tx bank send $address_c $address_d 10uscrt --gas-prices=0.25uscrt -y --chain-id=$CHAINID --home=$SECRETD_HOME --keyring-backend ${KEYRING} --output=json | jq ".txhash" | sed 's/"//g')
sleep 5s
$SECRETCLI q tx --type="hash" "$txhash" --output=json | jq
$SECRETCLI q bank balances $address_c --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_d --home=$SECRETD_HOME --output=json | jq

txhash=$($SECRETCLI tx bank send $address_d $address_a 10uscrt --gas-prices=0.25uscrt -y --chain-id=$CHAINID --home=$SECRETD_HOME --keyring-backend ${KEYRING} --output=json | jq ".txhash" | sed 's/"//g')
sleep 5s
$SECRETCLI q tx --type="hash" "$txhash" --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_d --home=$SECRETD_HOME --output=json | jq
$SECRETCLI q bank balances $address_a --home=$SECRETD_HOME --output=json | jq
# ----- BANK BALANCE TRANSERS - END -----

# ----- DISTRIBUTIONS - START -----
$SECRETCLI q distribution params --output=json | jq

$SECRETCLI q distribution community-pool --output=json | jq

address_valop=$(jq '.app_state.genutil.gen_txs[0].body.messages[0].validator_address' $SECRETD_HOME/config/genesis.json)

if [[ -z $address_valop ]]; then
  echo "No GENESIS tx in genesis.json"
  exit 1
fi

address_valop=$(echo $address_valop | sed 's/"//g')
$SECRETCLI q distribution validator-outstanding-rewards $address_valop --output=json | jq

$SECRETCLI q distribution commission $address_valop --output=json | jq

echo "FIXME: get realistic height"
$SECRETCLI q distribution slashes $address_valop "1" "10" --output=json | jq
# ----- DISTRIBUTIONS - END -----

# ----- SMART CONTRACTS - START -----
$SECRETCLI keys add scrtsc --keyring-backend ${KEYRING} --home=$SECRETD_HOME --output=json | jq
address_scrt=$($SECRETCLI keys show -a scrtsc --keyring-backend ${KEYRING} --home=$SECRETD_HOME)
$SECRETCLI q bank balances $address_scrt --home=$SECRETD_HOME --output=json | jq
txhash=$($SECRETCLI tx bank send $address_a $address_scrt 1000000uscrt --gas-prices=0.25uscrt -y --chain-id=$CHAINID --home=$SECRETD_HOME --keyring-backend ${KEYRING} --output=json | jq ".txhash" | sed 's/"//g')
sleep 5s
$SECRETCLI q tx --type=hash "$txhash" --output json | jq
$SECRETCLI q bank balances $address_scrt --home=$SECRETD_HOME --output=json | jq

txhash=$($SECRETCLI tx compute store ./integration-tests/test-contracts/contract.wasm.gz -y --gas 950000 --fees 12500uscrt --from $address_scrt --chain-id=$CHAINID --keyring-backend ${KEYRING} --home=$SECRETD_HOME --output=json | jq ".txhash" | sed 's/"//g')
sleep 5s
$SECRETCLI q tx --type=hash "$txhash" --output json | jq
$SECRETCLI q compute list-code --home=$SECRETD_HOME --output json | jq

CONTRACT_LABEL="counterContract"

TMPFILE=$(mktemp -p $TMP_DIR)

res_comp_1=$(mktemp -p $TMP_DIR)
$SECRETCLI tx compute instantiate 1 '{"count": 1}' --from $address_scrt --fees 5000uscrt --label $CONTRACT_LABEL -y --keyring-backend ${KEYRING} --home=$SECRETD_HOME --chain-id $CHAINID --output json | jq >$res_comp_1
txhash=$(cat $res_comp_1 | jq ".txhash" | sed 's/"//g')
sleep 5s
res_q_tx=$(mktemp -p $TMP_DIR)
$SECRETCLI q tx --type=hash "$txhash" --output json | jq >$res_q_tx
code_id=$(cat $res_q_tx | jq ".code")
if [[ ${code_id} -ne 0 ]]; then
  cat $res_q_tx | jq ".raw_log"
  exit 1
fi
sleep 5s
code_id=$($SECRETCLI q compute list-code --home=$SECRETD_HOME --output json | jq ".code_infos[0].code_id" | sed 's/"//g')
$SECRETCLI q compute list-contract-by-code $code_id --home=$SECRETD_HOME --output json | jq
contr_addr=$($SECRETCLI q compute list-contract-by-code $code_id --home=$SECRETD_HOME --output json | jq ".contract_infos[0].contract_address" | sed 's/"//g')
$SECRETCLI q compute contract $contr_addr --output json | jq
expected_count=$($SECRETCLI q compute query $contr_addr '{"get_count": {}}' --home=$SECRETD_HOME --output json | jq ".count")
if [[ ${expected_count} -ne 1 ]]; then
  echo "Expected count is 1, got ${expected_count}"
  exit 1
fi
# Scenario 1 - execute by query by contract label
json_compute_s1=$(mktemp -p $TMP_DIR)
$SECRETCLI tx compute execute --label $CONTRACT_LABEL --from scrtsc '{"increment":{}}' -y --home $SECRETD_HOME --keyring-backend ${KEYRING} --chain-id $CHAINID --fees 3000uscrt --output json | jq >$json_compute_s1
code_id=$(cat $json_compute_s1 | jq ".code")
if [[ ${code_id} -ne 0 ]]; then
  cat $json_compute_s1 | jq ".raw_log"
  exit 1
fi
txhash=$(cat $json_compute_s1 | jq ".txhash" | sed 's/"//g')
sleep 5s
$SECRETCLI q tx --type=hash "$txhash" --output json | jq

expected_count=$($SECRETCLI q compute query $contr_addr '{"get_count": {}}' --home=$SECRETD_HOME --output json | jq '.count')
if [[ ${expected_count} -ne 2 ]]; then
  echo "Expected count is 2, got ${expected_count}"
  exit 1
fi
# Scenario 2 - execute by contract address
json_compute_s2=$(mktemp -p $TMP_DIR)
$SECRETCLI tx compute execute $contr_addr --from scrtsc '{"increment":{}}' -y --home $SECRETD_HOME --keyring-backend ${KEYRING} --chain-id $CHAINID --fees 3000uscrt --output json | jq >$json_compute_s2
code_id=$(cat $json_compute_s2 | jq ".code")
if [[ ${code_id} -ne 0 ]]; then
  cat $json_compute_s2 | jq ".raw_log"
  exit 1
fi
sleep 5s
txhash=$(cat $json_compute_s2 | jq ".txhash" | sed 's/"//g')
$SECRETCLI q tx --type=hash "$txhash" --output json | jq

expected_count=$($SECRETCLI q compute query $contr_addr '{"get_count": {}}' --home=$SECRETD_HOME --output json | jq '.count')
if [[ ${expected_count} -ne 3 ]]; then
  echo "Expected count is 3, got ${expected_count}"
  exit 1
fi
# ----- SMART CONTRACTS - END -----

# ------ STAKING - START ----------
$SECRETCLI q staking params --output json | jq
retVal=$?
if [ $retVal -ne 0 ]; then
  echo "Error => $SECRETCLI q staking params"
  exit 1
fi

$SECRETCLI q staking validators --chain-id $CHAINID --output json | jq
retVal=$?
if [ $retVal -ne 0 ]; then
  echo "Error => $SECRETCLI q staking validators --chain-id $CHAINID"
  exit 1
fi

val_addr=$($SECRETCLI keys show validator --bech val -a --keyring-backend ${KEYRING} --home $SECRETD_HOME)
$SECRETCLI query staking delegations-to $val_addr --output json | jq
retVal=$?
if [ $retVal -ne 0 ]; then
  echo "Error => $SECRETCLI query staking delegations-to $val_addr"
  exit 1
fi

$SECRETCLI q staking validator $val_addr --chain-id $CHAINID --output json | jq
retVal=$?
if [ $retVal -ne 0 ]; then
  echo "Error => $SECRETCLI q staking validator $val_addr --chain-id $CHAINID"
  exit 1
fi

# Account A stakes 500uscrt to validator
if ! staking_delegate $val_addr $address_a 500; then
  echo "Staking validation from $address_a to $val_addr with 500uscrt failed"
  exit 1
fi
# Check account A stake with validator - should be 500
if ! staking_check $val_addr $address_a 500; then
  echo "Staking delegations from $address_a to $val_addr is not 500"
  exit 1
fi

# Account B stakes 1000uscrt to validator
if ! staking_delegate $val_addr $address_b 1000; then
  echo "Staking validation from $address_b to $val_addr with 1000uscrt failed"
  exit 1
fi
# Check account B stake with validator - should be 1000
if ! staking_check $val_addr $address_b 1000; then
  echo "Staking delegations from $address_b to $val_addr is not 1000"
  exit 1
fi

# Account C stakes 1500uscrt to validator
if ! staking_delegate $val_addr $address_c 1500; then
  echo "Staking validation from $address_c to $val_addr with 1500uscrt failed"
  exit 1
fi
# Check account C stake with validator - should be 1500
if ! staking_check $val_addr $address_c 1500; then
  echo "Staking delegations from $address_c to $val_addr is not 1500"
  exit 1
fi

# Account D stakes 5000uscrt to validator
if ! staking_delegate $val_addr $address_d 5000; then
  echo "Staking validation from $address_d to $val_addr with 5000uscrt failed"
  exit 1
fi
# Check account D stake with validator - should be 5000
if ! staking_check $val_addr $address_d 5000; then
  echo "Staking delegations from $address_d to $val_addr is not 5000"
  exit 1
fi

# Withdraw rewards from A
if ! staking_withdraw_rewards $val_addr $address_a; then
  echo "Withdrawing rewards for $address_a from $val_addr failed"
  exit 1
fi

# Check stakes for A
if ! staking_query_delegation $val_addr $address_a; then
  echo "Query delegations for $address_a with $val_addr is 0uscrt"
  exit 1
fi

# Check stakes for B
if ! staking_query_delegation $val_addr $address_b; then
  echo "Query delegations for $address_b with $val_addr is 0uscrt"
  exit 1
fi

# Check stakes for C
if ! staking_query_delegation $val_addr $address_c; then
  echo "Query delegations for $address_c with $val_addr is 0uscrt"
  exit 1
fi

# Check stakes for D
if ! staking_query_delegation $val_addr $address_d; then
  echo "Query delegations for $address_d with $val_addr is 0uscrt"
  exit 1
fi

# Check all stakes for A
if ! staking_query_delegations $address_a; then
  echo "Query delegations for $address_a"
  exit 1
fi

# Check all stakes for B
if ! staking_query_delegations $address_b; then
  echo "Query delegations for $address_b"
  exit 1
fi

# Check all stakes for C
if ! staking_query_delegations $address_c; then
  echo "Query delegations for $address_c"
  exit 1
fi

# Check all stakes for D
if ! staking_query_delegations $address_d; then
  echo "Query delegations for $address_d"
  exit 1
fi

# -------- STAKING - END ----------

# NOTE: this section should run after staking!
# ------ UNBONDING - START --------
if ! staking_unbond $val_addr $address_a 250; then
  echo "Tx staking unbond for $address_a from $val_addr with the amount 250 failed"
  exit 1
fi

if ! check_unbound $val_addr $address_a 250; then
  echo "Delegator ${address_a} new delegated amount with $val_addrs is not 250"
  exit 1
fi

if ! staking_unbond $val_addr $address_b 500; then
  echo "Tx staking unbond for $address_b from $val_addr with the amount 500 failed"
  exit 1
fi

if ! check_unbound $val_addr $address_b 500; then
  echo "Delegator ${address_b} new delegated amount with $val_addrs is not 500"
  exit 1
fi

if ! staking_unbond $val_addr $address_c 500; then
  echo "Tx staking unbond for $address_c from $val_addr with the amount 500 failed"
  exit 1
fi

if ! check_unbound $val_addr $address_c 1000; then
  echo "Delegator ${address_c} new delegated amount with $val_addrs is not 1000"
  exit 1
fi

if ! staking_unbond $val_addr $address_d 2500; then
  echo "Tx staking unbond for $address_d from $val_addr with the amount 2500 failed"
  exit 1
fi

if ! check_unbound $val_addr $address_d 2500; then
  echo "Delegator ${address_d} new delegated amount with $val_addrs is not 2500"
  exit 1
fi

if ! staking_check_pools; then
  echo "Staking pools are zeroes"
  exit 1
fi

# Check unbonding for address a - expect 250 (as per pervious transactions)
if ! staking_check_unbonding $val_addr $address_a 250; then
  echo "Staking check unbonding for ${address_a} failed"
  exit 1
fi

# Check unbonding for address b - expect 500 (as per pervious transactions)
if ! staking_check_unbonding $val_addr $address_b 500; then
  echo "Staking check unbonding for ${address_b} failed"
  exit 1
fi

# Check unbonding for address c - expect 500 (as per pervious transactions)
if ! staking_check_unbonding $val_addr $address_c 500; then
  echo "Staking check unbonding for ${address_c} failed"
  exit 1
fi

# Check unbonding for address d - expect 2500 (as per pervious transactions)
if ! staking_check_unbonding $val_addr $address_d 2500; then
  echo "Staking check unbonding for ${address_d} failed"
  exit 1
fi

# ------ UNBONDING - END ----------

# ------ SIGNING - START --------
# Note: TMP_DIR is already available and the cleanup with trap on exit set
# function cleanup_tmp_files {
#     echo "Clean up temp dir"
#     rm -fvr $TMP_DIR
# }
#TMP_DIR=$(mktemp -d -p $(pwd))
unsigned_tx_file=$TMP_DIR/unsigned_tx.json
amount_to_send="10000"
$SECRETCLI tx bank send $address_a $address_b ${amount_to_send}uscrt --fees=5000uscrt --generate-only --output=json >$unsigned_tx_file

unsigned_tx_file_aux=$TMP_DIR/unsigned_tx_aux.json
amount_to_send="10000"
$SECRETCLI tx bank send $address_a $address_b ${amount_to_send}uscrt --fee-payer=${address_b} --fees=5000uscrt --generate-only --output=json >$unsigned_tx_file_aux

# direct sign mode
signed_tx_file_direct=$TMP_DIR/signed_tx_direct.json
$SECRETCLI tx sign $unsigned_tx_file --from $address_a --keyring-backend ${KEYRING} --home ${SECRETD_HOME} >$signed_tx_file_direct
txhash=$($SECRETCLI tx broadcast $signed_tx_file_direct --from $address_a --keyring-backend ${KEYRING} --home ${SECRETD_HOME} --output=json | jq '.txhash' | tr -d '"')
sleep 5s
if [[ ! $($SECRETCLI q tx --type="hash" $txhash --output=json | jq) ]]; then
  echo "Failed to query tx $txhash"
  exit 1
fi

# amino-json sign mode
signed_tx_file_amino=$TMP_DIR/signed_tx_amino.json
$SECRETCLI tx sign $unsigned_tx_file --from $address_a --sign-mode=amino-json --keyring-backend ${KEYRING} --home ${SECRETD_HOME} >$signed_tx_file_amino
txhash=$($SECRETCLI tx broadcast $signed_tx_file_amino --from $address_a --keyring-backend ${KEYRING} --home ${SECRETD_HOME} --output=json | jq '.txhash' | tr -d '"')
sleep 5s
if [[ ! $($SECRETCLI q tx --type="hash" $txhash --output=json | jq) ]]; then
  echo "Failed to query tx $txhash"
  exit 1
fi

# direct aux sign mode
signed_tx_file_direct_aux=$TMP_DIR/signed_tx_direct_aux.json
signed_tx_file_direct_aux_final=$TMP_DIR/signed_tx_direct_aux_final.json
$SECRETCLI tx sign $unsigned_tx_file_aux --from $address_a --sign-mode=direct-aux --keyring-backend ${KEYRING} --home ${SECRETD_HOME} >$signed_tx_file_direct_aux
$SECRETCLI tx sign $signed_tx_file_direct_aux --from $address_b --keyring-backend ${KEYRING} --home ${SECRETD_HOME} >$signed_tx_file_direct_aux_final
txhash=$($SECRETCLI tx broadcast $signed_tx_file_direct_aux_final --from $address_b --keyring-backend ${KEYRING} --home ${SECRETD_HOME} --output=json | jq '.txhash' | tr -d '"')
sleep 5s
if [[ ! $($SECRETCLI q tx --type="hash" $txhash --output=json | jq) ]]; then
  echo "Failed to query tx $txhash"
  exit 1
fi

# encode/decode tx
encoded_tx=$TMP_DIR/encoded_tx
decoded_tx=$TMP_DIR/decoded_tx
if [[ $($SECRETCLI tx encode $signed_tx_file_direct_aux_final >$encoded_tx) ]]; then
  echo "Failed to encode tx $signed_tx_file_direct_aux_final"
  exit 1
fi
if [[ $($SECRETCLI tx decode $(cat $encoded_tx) >$decoded_tx) ]]; then
  echo "Failed to decode tx"
  exit 1
fi

# remove newline
signed_tx_file_direct_aux_final_truncated=$TMP_DIR/tx.json
cat $signed_tx_file_direct_aux_final | tr -d '\n' >$signed_tx_file_direct_aux_final_truncated

diff $decoded_tx $signed_tx_file_direct_aux_final_truncated >/dev/null
if [[ ! $? ]]; then
  echo "Failed to match decoded and signed txs"
  exit 1
fi

# multisig
$SECRETCLI keys add --multisig=a,b,c --multisig-threshold 2 abc --home=$SECRETD_HOME
address_abc=$($SECRETCLI keys show -a abc --keyring-backend ${KEYRING} --home=$SECRETD_HOME)

$SECRETCLI q bank balance abc uscrt --output=json | jq '.balance.amount'
$SECRETCLI tx bank send $address_a $address_abc 100000uscrt --fees=2500uscrt -y --keyring-backend ${KEYRING} --home ${SECRETD_HOME}
sleep 5s
$SECRETCLI q bank balance abc uscrt --output=json | jq '.balance.amount'

unsigned_tx_file_multisig=$TMP_DIR/unsigned_tx_multisig.json
signed_a=$TMP_DIR/aSig.json
signed_b=$TMP_DIR/bSig.json
signed_multisig=$TMP_DIR/signed_multisig.json
amount_to_send_multisig="1000"
$SECRETCLI tx bank send $address_abc $address_a ${amount_to_send_multisig}uscrt --fees=5000uscrt --generate-only --output=json >$unsigned_tx_file_multisig

$SECRETCLI tx sign --multisig=abc --from a --output=json $unsigned_tx_file_multisig --keyring-backend ${KEYRING} --home ${SECRETD_HOME} >$signed_a
if [ $? -ne 0]; then
  echo "Failed to $SECRETCLI tx sign --multisig=abc --from a --output=json $unsigned_tx_file_multisig"
  exit 1
fi
$SECRETCLI tx sign --multisig=abc --from b --output=json $unsigned_tx_file_multisig --keyring-backend ${KEYRING} --home ${SECRETD_HOME} >$signed_b
if [ $? -ne 0]; then
  echo "Failed to $SECRETCLI tx sign --multisig=abc --from b --output=json $unsigned_tx_file_multisig"
  exit 1
fi
$SECRETCLI tx multisign $unsigned_tx_file_multisig abc $signed_a $signed_b --keyring-backend ${KEYRING} --home ${SECRETD_HOME} --output json >$signed_multisig
if [ $? -ne 0]; then
  echo "Failed to $SECRETCLI tx multisign $unsigned_tx_file_multisig abc $signed_a $signed_b"
  exit 1
fi

tx_bc_ms=$(mktemp -p $TMP_DIR)
$SECRETCLI tx broadcast $signed_multisig --from a --keyring-backend ${KEYRING} --home ${SECRETD_HOME} --output json | jq > $tx_bc_ms
if [ $? -ne 0 ]; then
  echo "Failed to $SECRETCLI tx broadcast $signed_multisig --from a"
  exit 1
fi
txhash=$(cat $tx_bc_ms | jq '.txhash' | tr -d '"')
sleep 10s
qtx_json=$(mktemp -p $TMP_DIR)
$SECRETCLI q tx --type="hash" $txhash --output=json > $qtx_json
if [ $? -ne 0 ]; then
  echo "Error: Failed to query tx by hash $txhash"
  exit 1
fi
echo "Tx: $txhash Code:$(cat $qtx_json | jq '.code') RawLog:$(cat $qtx_json | jq '.raw_log')"
# ------ SIGNING - END --------

set +x
echo " *** INTEGRATION TESTS PASSED! ***"
exit 0
