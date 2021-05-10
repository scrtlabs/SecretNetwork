#!/bin/bash
set -eu

export RUMOR_TENDERMINT_ENDPOINT='bootstrap:26657'
export RUMOR_HOME='.secretd'

echo "Waiting for node to start..."
sleep 30

rumor
