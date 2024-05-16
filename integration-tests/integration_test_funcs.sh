
# --------- UNBONDING ---------
# Staking unbound - unbound certian funds back from the validator to the delegator
# Args:
#   validator operator address
#   delegator address
#   amount to unbound
function staking_unbond() {
    local val_addr=${1:?}
    local del_addr=${2:?}
    local -i amount=${3:?}

    json_unbond=$(mktemp -p $TMP_DIR)

    $SECRETCLI tx staking unbond $val_addr ${amount}uscrt -y --from $del_addr --chain-id $CHAINID --keyring-backend test --home $SECRETD_HOME --fees 3000uscrt --output json | jq > $json_unbond
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "Error =>  $SECRETCLI tx staking unbond $val_addr ${amount}uscrt --from $del_addr --chain-id $CHAINID"
        return 1
    fi
    code_id=$(cat $json_unbond | jq ".code")
    if [[ ${code_id} -ne 0 ]]; then 
        cat $json_unbond | jq ".raw_log"
        return 1
    fi
    txhash=$(cat $json_unbond | jq ".txhash" | sed 's/"//g')
    sleep 5s
    json_unbond_tx=$(mktemp -p $TMP_DIR)
    $SECRETCLI q tx --type hash $txhash --output json | jq > $json_unbond_tx
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "Error => $SECRETCLI q tx --type hash $txhash"
        return 1
    fi
    code_id=$(cat $json_unbond_tx | jq ".code")
    if [[ ${code_id} -ne 0 ]]; then 
        $(cat $json_unbond_tx | jq ".raw_log")
        return 1
    fi
    return 0
}

# Check if the remaining balance matches the expected value
# Args:
#   validator operator address
#   delegator address
#   expected amount
function check_unbound() {
    local val_addr=${1:?}
    local del_addr=${2:?}
    local -i expected_amount=${3:?}

    json_query=$(mktemp -p $TMP_DIR)
    $SECRETCLI q staking delegation ${del_addr} ${val_addr} --chain-id $CHAINID --output json | jq > $json_query
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "Error =>  $SECRETCLI q staking delegation ${del_addr} ${val_addr} --chain-id $CHAINID"
        return 1
    fi
    amount=$(cat $json_query | jq '.delegation_response.balance.amount' | sed 's/"//g')
    if [ $amount -ne $expected_amount ]; then
        echo "Error => ${del_addr} expected amount is ${expected_amount}, factual is ${amount}"
        return 1
    fi
    return 0
}
# ------ UNBONDING - END ------

# ---------- STAKING ----------
# Staking queries - delegations from delegator with specific validator
# Args:
#   validator operator address
#   delegator address
function staking_query_delegation() {
    local val_addr=${1:?}
    local del_addr=${2:?}
    json_query=$(mktemp -p $TMP_DIR)
    $SECRETCLI q staking delegation ${del_addr} ${val_addr} --chain-id $CHAINID --output json | jq > $json_query
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "Error =>  $SECRETCLI q staking delegation ${del_addr} ${val_addr} --chain-id $CHAINID"
        return 1
    fi
    amount=$(cat $json_query | jq '.delegation_response.balance.amount' | sed 's/"//g')
    if [ $amount -eq 0 ]; then
        echo "Error => ${del_addr} amount is ${amount}"
        return 1
    fi
    return 0
}

# Staking queries - delegations from delegator
# Args:
#   delegator address
function staking_query_delegations() {
    local del_addr=${1:?}
    json_query=$(mktemp -p $TMP_DIR)
    $SECRETCLI q staking delegations ${del_addr} --chain-id $CHAINID --output json | jq > $json_query
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "Error =>  $SECRETCLI q staking delegations ${del_addr} --chain-id $CHAINID"
        return 1
    fi
    cat $json_query | jq -c '.delegation_responses[] | select ( .balance.amount )' | jq
    return 0
}

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
    cat $json_delegate_tx | jq ".tx" | jq
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
        return 1
    fi
    return 0
}

# Withdraw rewards for delegator from validator
# Args:
#   validatator operator address
#   Delegator address
function staking_withdraw_rewards() {
    local val_addr=${1:?}
    local del_addr=${2:?}
    json_withdraw=$(mktemp -p $TMP_DIR)
    $SECRETCLI tx distribution withdraw-rewards $val_addr -y --from $del_addr --keyring-backend test --home $SECRETD_HOME --chain-id $CHAINID --output json --fees 3000uscrt | jq > $json_withdraw
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "Error => $SECRETCLI tx distribution withdraw-rewards $val_addr -y --from $del_addr --keyring-backend test --home $SECRETD_HOME --chain-id $CHAINID --output json --fees 3000uscrt"
        return 1
    fi
    code_id=$(cat $json_withdraw | jq ".code")
    if [[ ${code_id} -ne 0 ]]; then 
        cat $json_withdraw | jq ".raw_log"
        return 1
    fi
    txhash=$(cat $json_withdraw | jq ".txhash" | sed 's/"//g')
    sleep 5s
    json_withdraw_tx=$(mktemp -p $TMP_DIR)
    $SECRETCLI q tx --type hash $txhash --output json | jq > $json_withdraw_tx
    retVal=$?
    if [ $retVal -ne 0 ]; then
        echo "Error => $SECRETCLI q tx --type hash $txhash"
        return 1
    fi
    code_id=$(cat $json_withdraw_tx | jq ".code")
    if [[ ${code_id} -ne 0 ]]; then 
        $(cat $json_withdraw_tx | jq ".raw_log")
        return 1
    fi
    cat $json_withdraw_tx | jq ".tx" | jq
    return 0
}
# ------------------------------