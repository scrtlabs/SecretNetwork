#!/bin/bash

file=~/.secretd/config/genesis.json
if [ ! -e "$file" ]
then
  # init the node
  rm -rf ~/.secretd/*
  rm -rf /opt/secret/.sgx_secrets/*

  chain_id=${CHAINID:-secretdev-1}
  LOG_LEVEL=${LOG_LEVEL:-INFO}

  mkdir -p ./.sgx_secrets
  secretd config chain-id "$chain_id"
  secretd config output json
  secretd config keyring-backend test

  # export SECRET_NETWORK_CHAIN_ID=secretdev-1
  # export SECRET_NETWORK_KEYRING_BACKEND=test
  secretd init banana --chain-id "$chain_id"


  cp ~/node_key.json ~/.secretd/config/node_key.json
  perl -i -pe 's/"stake"/"uscrt"/g' ~/.secretd/config/genesis.json
  perl -i -pe 's/"172800s"/"90s"/g' ~/.secretd/config/genesis.json # voting period 2 days -> 90 seconds
  perl -i -pe 's/"1814400s"/"80s"/g' ~/.secretd/config/genesis.json # unbonding period 21 days -> 80 seconds

  perl -i -pe 's/enable-unsafe-cors = false/enable-unsafe-cors = true/g' ~/.secretd/config/app.toml # enable cors

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

  secretd gentx a 1000000uscrt --chain-id "$chain_id"

  secretd collect-gentxs
  secretd validate-genesis

#  secretd init-enclave
  secretd init-bootstrap
#  cp new_node_seed_exchange_keypair.sealed .sgx_secrets
  secretd validate-genesis
fi

# Setup CORS for LCD & gRPC-web
perl -i -pe 's;address = "tcp://0.0.0.0:1317";address = "tcp://0.0.0.0:1316";' .secretd/config/app.toml
perl -i -pe 's/enable-unsafe-cors = false/enable-unsafe-cors = true/' .secretd/config/app.toml
lcp --proxyUrl http://localhost:1316 --port 1317 --proxyPartial '' &

# Setup faucet
setsid node faucet_server.js &

# Setup secretcli
cp $(which secretd) $(dirname $(which secretd))/secretcli

source /opt/sgxsdk/environment && RUST_BACKTRACE=1 LOG_LEVEL="$LOG_LEVEL" secretd start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap

