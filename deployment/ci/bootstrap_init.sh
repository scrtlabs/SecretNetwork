#!/bin/bash

set -euvo pipefail

rm -rf ~/.secretd/*
rm -rf /opt/secret/.sgx_secrets/*

secretd config chain-id secretdev-1
secretd config keyring-backend test

secretd init banana --chain-id secretdev-1

cp ~/node_key.json ~/.secretd/config/node_key.json
perl -i -pe 's/"stake"/ "uscrt"/g' ~/.secretd/config/genesis.json

perl -i -pe 's/concurrency = false/concurrency = true/' .secretd/config/app.toml

a_mnemonic="grant rice replace explain federal release fix clever romance raise often wild taxi quarter soccer fiber love must tape steak together observe swap guitar"
b_mnemonic="jelly shadow frog dirt dragon use armed praise universe win jungle close inmate rain oil canvas beauty pioneer chef soccer icon dizzy thunder meadow"
c_mnemonic="chair love bleak wonder skirt permit say assist aunt credit roast size obtain minute throw sand usual age smart exact enough room shadow charge"
d_mnemonic="word twist toast cloth movie predict advance crumble escape whale sail such angry muffin balcony keen move employ cook valve hurt glimpse breeze brick"

echo $a_mnemonic | secretd keys add a --recover
echo $b_mnemonic | secretd keys add b --recover
echo $c_mnemonic | secretd keys add c --recover
echo $d_mnemonic | secretd keys add d --recover

secretd add-genesis-account "$(secretd keys show -a a)" 1000000000000000000uscrt
secretd add-genesis-account "$(secretd keys show -a b)" 1000000000000000000uscrt
secretd add-genesis-account "$(secretd keys show -a c)" 1000000000000000000uscrt
secretd add-genesis-account "$(secretd keys show -a d)" 1000000000000000000uscrt

secretd gentx a 1000000uscrt --chain-id secretdev-1

secretd collect-gentxs
secretd validate-genesis

secretd init-bootstrap
secretd validate-genesis

source /opt/sgxsdk/environment && RUST_BACKTRACE=1 secretd start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap
