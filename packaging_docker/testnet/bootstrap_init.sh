#!/bin/bash

file=~/.enigmad/config/genesis.json
if [ ! -e "$file" ]

  # init the node
  # rm -rf ~/.enigma* || true
  enigmacli config chain-id enigma-testnet-1
  enigmacli config output json
  enigmacli config indent true
  enigmacli config trust-node true
  enigmacli config keyring-backend test

  enigmad init banana --chain-id enigma-testnet-1

  cp ~/node_key.json ~/.enigmad/config/node_key.json

  perl -i -pe 's/"stake"/"uscrt"/g' ~/.enigmad/config/genesis.json
  enigmacli keys add a
  enigmacli keys add b
  enigmacli keys add c
  enigmacli keys add d

  enigmad add-genesis-account "$(enigmacli keys show -a a)" 1000000000000000000uscrt
  enigmad add-genesis-account "$(enigmacli keys show -a b)" 1000000000000000000uscrt
  enigmad add-genesis-account "$(enigmacli keys show -a c)" 1000000000000000000uscrt
  enigmad add-genesis-account "$(enigmacli keys show -a d)" 1000000000000000000uscrt


  enigmad gentx --name a --keyring-backend test --amount 1000000uscrt
  enigmad gentx --name b --keyring-backend test --amount 1000000uscrt
  enigmad gentx --name c --keyring-backend test --amount 1000000uscrt
  enigmad gentx --name d --keyring-backend test --amount 1000000uscrt

  enigmad collect-gentxs
  enigmad validate-genesis

  enigmad init-bootstrap
  enigmad validate-genesis
fi
source /opt/sgxsdk/environment && RUST_BACKTRACE=1 enigmad start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap