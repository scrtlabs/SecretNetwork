#!/bin/bash

set -euvo pipefail

rm -rf ~/.secretd/*
rm -rf /opt/secret/.sgx_secrets/*

secretd config chain-id secretdev-1
secretd config keyring-backend test

secretd init banana --chain-id secretdev-1

cp ~/node_key.json ~/.secretd/config/node_key.json
perl -i -pe 's/"stake"/ "uscrt"/g' ~/.secretd/config/genesis.json

secretd keys add a
secretd keys add b
secretd keys add c
secretd keys add d

secretd add-genesis-account "$(secretd keys show -a a)" 1000000000000000000uscrt

secretd gentx a 1000000uscrt --chain-id secretdev-1

secretd collect-gentxs
secretd validate-genesis

secretd init-bootstrap
secretd validate-genesis

source /opt/sgxsdk/environment && RUST_BACKTRACE=1 secretd start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap
