#!/bin/bash

set -euvo pipefail

rm -rf ~/.secretd/*
rm -rf /opt/secret/.sgx_secrets/*

secretd config set client chain-id secretdev-1
secretd config set client keyring-backend test

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

secretd genesis add-genesis-account "$(secretd keys show -a a)" 1000000000000000000uscrt
secretd genesis add-genesis-account "$(secretd keys show -a b)" 1000000000000000000uscrt
secretd genesis add-genesis-account "$(secretd keys show -a c)" 1000000000000000000uscrt
secretd genesis add-genesis-account "$(secretd keys show -a d)" 1000000000000000000uscrt

secretd genesis gentx a 1000000uscrt --chain-id secretdev-1

secretd genesis collect-gentxs
secretd genesis validate

secretd init-bootstrap
secretd genesis validate

# Setup LCD
perl -i -pe 's;address = "tcp://localhost:1317";address = "tcp://0.0.0.0:1317";' ~/.secretd/config/app.toml

source /opt/sgxsdk/environment && RUST_BACKTRACE=1 secretd start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap
