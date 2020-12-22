#!/bin/bash

# init the node
rm -rf ~/.secret*
secretcli config chain-id enigma-testnet
secretcli config output json
secretcli config indent true
secretcli config trust-node true
secretcli config keyring-backend test

secretd init banana --chain-id enigma-testnet

cp ~/node_key.json ~/.secretd/config/node_key.json

perl -i -pe 's/"stake"/"uscrt"/g' ~/.secretd/config/genesis.json
secretcli keys add a

secretd add-genesis-account "$(secretcli keys show -a a)" 1000000000000uscrt
secretd gentx --name a --keyring-backend test --amount 1000000uscrt
secretd collect-gentxs
secretd validate-genesis

secretd init-bootstrap
secretd validate-genesis

sed -i 's/persistent_peers = ""/persistent_peers = "'"$PERSISTENT_PEERS"'"/g' ~/.secretd/config/config.toml

source /opt/sgxsdk/environment && RUST_BACKTRACE=1 secretd start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap