#!/bin/bash
set -o xtrace

UPGRADE_BLOCK="$(docker exec node bash -c 'secretd status | jq "(.SyncInfo.latest_block_height | tonumber) + 30"')"
# Propose upgrade
PROPOSAL_ID="$(docker exec node bash -c "secretd tx gov submit-proposal software-upgrade v1.15 --upgrade-height $UPGRADE_BLOCK --title blabla --description yolo --deposit 100000000uscrt --from a -y -b block | jq '.logs[0].events[] | select(.type == \"submit_proposal\") | .attributes[] | select(.key == \"proposal_id\") | .value | tonumber'")"
# Vote yes (voting period is 90 seconds)
docker exec node bash -c "secretd tx gov vote ${PROPOSAL_ID} yes --from a -y -b block"

echo "PROPOSAL_ID   = ${PROPOSAL_ID}"
echo "UPGRADE_BLOCK = ${UPGRADE_BLOCK}"
