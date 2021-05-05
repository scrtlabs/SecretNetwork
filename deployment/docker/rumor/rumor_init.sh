#!/bin/bash
set -eu

export RUMOR_TENDERMINT_ENDPOINT='node:26657'
export RUMOR_RPC_LADDR='tcp://0.0.0.0:26659'
export RUMOR_HOME='.secretd'

echo "Waiting for node to start..."
sleep 30

curl http://"$RUMOR_TENDERMINT_ENDPOINT"/genesis | jq -r .result.genesis > /root/.rumor/genesis.json

rumor
