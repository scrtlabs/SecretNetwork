#!/bin/bash

file=~/.secretd/config/genesis.json
if [ ! -e "$file" ]; then
  # init the node
  rm -rf ~/.secretd/*
  rm -rf ~/.secretcli/*
  rm -rf ~/.sgx_secrets/*
  secretcli config chain-id enigma-pub-testnet-3
  secretcli config output json
  secretcli config indent true
  secretcli config trust-node true
  secretcli config keyring-backend test

  secretd init banana --chain-id enigma-pub-testnet-3

  cp ~/node_key.json ~/.secretd/config/node_key.json

  perl -i -pe 's/"stake"/"uscrt"/g' ~/.secretd/config/genesis.json
  secretcli keys add a
  secretcli keys add b
  secretcli keys add c
  secretcli keys add d

  secretd add-genesis-account "$(secretcli keys show -a a)" 1000000000000000000uscrt
  secretd add-genesis-account "$(secretcli keys show -a b)" 1000000000000000000uscrt
  secretd add-genesis-account "$(secretcli keys show -a c)" 1000000000000000000uscrt
  secretd add-genesis-account "$(secretcli keys show -a d)" 1000000000000000000uscrt

  secretd gentx --name a --keyring-backend test --amount 1000000uscrt
  secretd gentx --name b --keyring-backend test --amount 1000000uscrt
  secretd gentx --name c --keyring-backend test --amount 1000000uscrt
  secretd gentx --name d --keyring-backend test --amount 1000000uscrt

  secretd collect-gentxs
  secretd validate-genesis

  secretd init-bootstrap
  secretd validate-genesis

  perl -i -pe 's/max_subscription_clients.+/max_subscription_clients = 100/' ~/.secretd/config/config.toml
  perl -i -pe 's/max_subscriptions_per_client.+/max_subscriptions_per_client = 50/' ~/.secretd/config/config.toml
fi

secretcli rest-server --trust-node=true --chain-id enigma-pub-testnet-3 --laddr tcp://0.0.0.0:1336 &
lcp --proxyUrl http://localhost:1336 --port 1337 --proxyPartial '' &

source /opt/sgxsdk/environment && RUST_BACKTRACE=1 secretd start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap