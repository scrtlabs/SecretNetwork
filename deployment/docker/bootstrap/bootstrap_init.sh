#!/bin/bash

file=~/.secretd/config/genesis.json
if [ ! -e "$file" ]; then
  # init the node
  rm -rf ~/.secretd/*
  rm -rf /opt/secret/.sgx_secrets/*

  if [ -z "${CHAINID}" ]; then
    chain_id="$CHAINID"
  else
    chain_id="supernova-1"
  fi

  mkdir -p ./.sgx_secrets
  secretd config chain-id "$chain_id"
  secretd config keyring-backend test

  # export SECRET_NETWORK_CHAIN_ID=secretdev-1
  # export SECRET_NETWORK_KEYRING_BACKEND=test
  secretd init banana --chain-id "$chain_id"


  cp ~/node_key.json ~/.secretd/config/node_key.json
  perl -i -pe 's/"stake"/"uscrt"/g' ~/.secretd/config/genesis.json
  perl -i -pe 's/"172800000000000"/"90000000000"/g' ~/.secretd/config/genesis.json # voting period 2 days -> 90 seconds

  secretd keys add a
  secretd keys add b
  secretd keys add c
  secretd keys add d

  secretd add-genesis-account "$(secretd keys show -a a)" 1000000000000000000uscrt
#  secretd add-genesis-account "$(secretd keys show -a b)" 1000000000000000000uscrt
#  secretd add-genesis-account "$(secretd keys show -a c)" 1000000000000000000uscrt
#  secretd add-genesis-account "$(secretd keys show -a d)" 1000000000000000000uscrt


  secretd gentx a 1000000uscrt --chain-id "$chain_id"
#  secretd gentx b 1000000uscrt --keyring-backend test
#  secretd gentx c 1000000uscrt --keyring-backend test
#  secretd gentx d 1000000uscrt --keyring-backend test

  secretd collect-gentxs
  secretd validate-genesis

#  secretd init-enclave
  secretd init-bootstrap
#  cp new_node_seed_exchange_keypair.sealed .sgx_secrets
  secretd validate-genesis

  perl -i -pe 's/max_subscription_clients.+/max_subscription_clients = 100/' ~/.secretd/config/config.toml
  perl -i -pe 's/max_subscriptions_per_client.+/max_subscriptions_per_client = 50/' ~/.secretd/config/config.toml
fi

lcp --proxyUrl http://localhost:1317 --port 1337 --proxyPartial '' &

# sleep infinity
source /opt/sgxsdk/environment && RUST_BACKTRACE=1 secretd start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap