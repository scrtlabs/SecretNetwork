#!/bin/bash
set -ox xtrace

UPGRADE_BLOCK="$(docker exec node bash -c 'secretd status | jq "(.sync_info.latest_block_height | tonumber) + 30"')"
# Propose upgrade
TX_HASH="$(docker exec node bash -c "secretd tx upgrade software-upgrade v1.16 --upgrade-height $UPGRADE_BLOCK --title blabla --summary yolo --deposit 1000000000uscrt --no-verify --from a -y | jq '.txhash'")"
sleep 10
PROPOSAL_ID="$(docker exec node bash -c "secretd q tx $TX_HASH | jq '.events[] | select(.type == \"submit_proposal\") | .attributes[] | select(.key == \"proposal_id\") | .value | tonumber'")"
# Vote yes (voting period is 90 seconds)
docker exec node bash -c "secretd tx gov vote ${PROPOSAL_ID} yes --from validator -y"

echo "PROPOSAL_ID   = ${PROPOSAL_ID}"
echo "UPGRADE_BLOCK = ${UPGRADE_BLOCK}"
