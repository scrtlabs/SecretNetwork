#!/bin/bash

# init the node
rm -rf ~/.enigma*
enigmacli config chain-id enigma-testnet
enigmacli config output json
enigmacli config indent true
enigmacli config trust-node true
enigmacli config keyring-backend test

enigmad init banana --chain-id enigma-testnet

cp ~/node_key.json ~/.enigmad/config/node_key.json

perl -i -pe 's/"stake"/"uscrt"/g' ~/.enigmad/config/genesis.json
enigmacli keys add a

enigmad add-genesis-account "$(enigmacli keys show -a a)" 1000000000000uscrt
enigmad gentx --name a --keyring-backend test --amount 1000000uscrt
enigmad collect-gentxs
enigmad validate-genesis

enigmad init-bootstrap
enigmad validate-genesis

sed -i 's/persistent_peers = ""/persistent_peers = "'"$PERSISTENT_PEERS"'"/g' ~/.enigmad/config/config.toml

RUST_BACKTRACE=1 enigmad start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap