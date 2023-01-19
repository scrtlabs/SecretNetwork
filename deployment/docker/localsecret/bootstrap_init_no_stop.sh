#!/bin/bash

ENABLE_FAUCET=${1:-"true"}

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
  jq '
    .consensus_params.block.time_iota_ms = "10" | 
    .app_state.staking.params.unbonding_time = "90s" | 
    .app_state.gov.voting_params.voting_period = "90s" |
    .app_state.crisis.constant_fee.denom = "uscrt" |
    .app_state.gov.deposit_params.min_deposit[0].denom = "uscrt" |
    .app_state.mint.params.mint_denom = "uscrt" |
    .app_state.staking.params.bond_denom = "uscrt"
  ' ~/.secretd/config/genesis.json > ~/.secretd/config/genesis.json.tmp && mv ~/.secretd/config/genesis.json{.tmp,}

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
  secretd gentx b 1000000uscrt --chain-id "$chain_id"
  secretd gentx c 1000000uscrt --chain-id "$chain_id"
  secretd gentx d 1000000uscrt --chain-id "$chain_id"

  secretd collect-gentxs
  secretd validate-genesis

  secretd init-bootstrap
  secretd validate-genesis

  # Setup LCD
  perl -i -pe 's;address = "tcp://0.0.0.0:1317";address = "tcp://0.0.0.0:1316";' ~/.secretd/config/app.toml
  perl -i -pe 's/enable-unsafe-cors = false/enable-unsafe-cors = true/' ~/.secretd/config/app.toml
  perl -i -pe 's/concurrency = false/concurrency = true/' ~/.secretd/config/app.toml
fi

setsid lcp --proxyUrl http://localhost:1316 --port 1317 --proxyPartial '' &

if [ "${ENABLE_FAUCET}" = "true" ]; then
  # Setup faucet
  setsid node faucet_server.js &
  # Setup secretcli
  cp $(which secretd) $(dirname $(which secretd))/secretcli
fi

source /opt/sgxsdk/environment && RUST_BACKTRACE=1 LOG_LEVEL="$LOG_LEVEL" secretd start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap

