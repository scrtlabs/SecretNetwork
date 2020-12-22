#!/bin/bash

file=~/.secretd/config/genesis.json
if [ ! -e "$file" ]
then
  # init the node
  rm -rf ~/.secretd/*
  rm -rf ~/.secretcli/*
  rm -rf ~/.sgx_secrets/*
  secretcli config chain-id holodeck-2
  secretcli config output json
  secretcli config indent true
  secretcli config trust-node true
  secretcli config keyring-backend test

  secretd init ChainofSecretsBootstrap --chain-id holodeck-2

  cp ~/node_key.json ~/.secretd/config/node_key.json

  perl -i -pe 's/"stake"/"uscrt"/g' ~/.secretd/config/genesis.json
  secretcli keys add cos-bootstrap
  secretcli keys add chula
  secretcli keys add tonga
  secretcli keys add fizzbin

  secretd add-genesis-account "$(secretcli keys show -a cos-bootstrap)" 1000000000000000000uscrt
  secretd add-genesis-account "$(secretcli keys show -a chula)" 1000000000000000000uscrt
  secretd add-genesis-account "$(secretcli keys show -a tonga)" 1000000000000000000uscrt
  secretd add-genesis-account "$(secretcli keys show -a fizzbin)" 1000000000000000000uscrt


  secretd gentx --name cos-bootstrap --keyring-backend test --amount 1000000uscrt

  secretd collect-gentxs
  secretd validate-genesis

  secretd init-bootstrap
  secretd validate-genesis
fi

# sleep infinity
RUST_BACKTRACE=1 secretd start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap
