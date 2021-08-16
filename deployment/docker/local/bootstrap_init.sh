#!/bin/bash
set -euo pipefail

file=~/.secretd/config/genesis.json
if [ ! -e "$file" ]
then
  # init the node
  rm -rf ~/.secretd/*
  rm -rf ~/.secretcli/*
  rm -rf ~/.sgx_secrets/*
  secretcli config chain-id enigma-pub-testnet-3
  secretcli config output json
#  secretcli config indent true
#  secretcli config trust-node true
  secretcli config keyring-backend test

  secretd init banana --chain-id enigma-pub-testnet-3

  cp ~/node_key.json ~/.secretd/config/node_key.json

  perl -i -pe 's/"stake"/"uscrt"/g' ~/.secretd/config/genesis.json
  secretcli keys add a
  secretcli keys add b
  secretcli keys add c
  secretcli keys add d

  secretd add-genesis-account "$(secretcli keys show -a a)" 1000000000000000000uscrt
#  secretd add-genesis-account "$(secretcli keys show -a b)" 1000000000000000000uscrt
#  secretd add-genesis-account "$(secretcli keys show -a c)" 1000000000000000000uscrt
#  secretd add-genesis-account "$(secretcli keys show -a d)" 1000000000000000000uscrt


  secretd gentx a 1000000uscrt --keyring-backend test --chain-id enigma-pub-testnet-3
  # These fail for some reason:
  # secretd gentx --name b --keyring-backend test --amount 1000000uscrt
  # secretd gentx --name c --keyring-backend test --amount 1000000uscrt
  # secretd gentx --name d --keyring-backend test --amount 1000000uscrt

  secretd collect-gentxs
  secretd validate-genesis

  secretd init-bootstrap
  secretd validate-genesis
fi

# sleep infinity
source /opt/sgxsdk/environment && RUST_BACKTRACE=1 secretd start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap
