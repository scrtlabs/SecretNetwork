
# ---------- STAKING ----------
# Staking delegation
# Args:
#   validator address
#   delegator address
#   amount
function staking_delegate() {
    local val_addr=${1:?}
    local del_addr=${2:?}
    local -i amount=${3:?}
    json_delegate=$(mktemp -p $TMP_DIR)
    $SECRETCLI tx staking delegate $val_addr ${amount}uscrt -y --from $del_addr --chain-id $CHAINID --keyring-backend test --home $SECRETD_HOME --fees 3000uscrt --output json| jq > $json_delegate
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "Error => $SECRETCLI tx staking delegate $val_addr 500uscrt -y --from a --chain-id $CHAINID --keyring-backend test --home $SECRETD_HOME --fees 3000uscrt"
        return 1
    fi
    code_id=$(cat $json_delegate | jq ".code")
    if [[ ${code_id} -ne 0 ]]; then 
        cat $json_delegate | jq ".raw_log"
        return 1
    fi
    txhash=$(cat $json_delegate | jq ".txhash" | sed 's/"//g')
    sleep 5s
    json_delegate_tx=$(mktemp -p $TMP_DIR)
    $SECRETCLI q tx --type hash $txhash --output json | jq > $json_delegate_tx
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "Error => $SECRETCLI q tx --type hash $txhash"
        return 1
    fi
    code_id=$(cat $json_delegate_tx | jq ".code")
    if [[ ${code_id} -ne 0 ]]; then 
        $(cat $json_delegate_tx | jq ".raw_log")
        return 1
    fi
    echo "Blcok height:" $(cat $json_delegate_tx | jq ".height")
    $(cat $json_delegate_tx | jq ".tx" | jq)
    return 0
}

# Staking check - checks the delegation amount for the specified delegator
# Args:
#   validator address
#   delegator address
#   expected amount
function staking_check() {
    local val_addr=${1:?}
    local del_addr=${2:?}
    local -i amount=${3:?}
    json_q_stakes=$(mktemp -p $TMP_DIR)
    $SECRETCLI query staking delegations-to $val_addr --output json | jq > $json_q_stakes
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "Error => $SECRETCLI query staking delegations-to $val_addr"
        return 1
    fi
    jq_staking_query_1=".delegation_responses[] | select ( .delegation.delegator_address | contains(\"$del_addr\") )"
    staking_amount_a=$(cat $json_q_stakes | jq -c "$jq_staking_query_1" | jq '.balance.amount' | sed 's/"//g')
    if [ ${staking_amount_a} -ne ${amount} ]; then
    echo "Error => Staking amount for account a with $val_addr is incorrect. Expected $amount, got $staking_amount_a"
    exit 1
    fi

}
# ------------------------------