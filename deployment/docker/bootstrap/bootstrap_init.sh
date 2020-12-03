#!/bin/bash

file=~/.secretd/config/genesis.json
if [ ! -e "$file" ]; then
  # init the node
  rm -rf ~/.secretd/*
  rm -rf ~/.secretcli/*
  rm -rf ~/.sgx_secrets/*
  secretcli config chain-id enigma-pub-testnet-3
  secretcli config output json
  secretcli config indent true
  secretcli config trust-node true
  secretcli config keyring-backend test

  secretd init banana --chain-id enigma-pub-testnet-3

  cp ~/node_key.json ~/.secretd/config/node_key.json

  perl -i -pe 's/"stake"/"uscrt"/g' ~/.secretd/config/genesis.json
  echo flame satoshi grace olive stove economy prepare hurdle end mandate sure daring more armor family produce rural quick winter monster armor hollow impose skin |
    secretcli keys add a --recover
  echo wrist mansion remain topic trick monitor olympic auction piano entry sheriff dial trash armed arrow few welcome pole clown mesh party squeeze bridge exhibit |
    secretcli keys add b --recover
  echo shadow lamp orphan upgrade improve tomorrow secret eight dinosaur hotel believe tunnel emotion problem angry goose grace expire suggest slide federal million major prevent |
    secretcli keys add c --recover
  echo quote coffee avocado reflect latin gather powder outside pudding idle era surge stock second mandate pilot another promote sword ordinary coconut hospital drift spawn |
    secretcli keys add d --recover

  secretd add-genesis-account "$(secretcli keys show -a a)" 1000000000000000000uscrt
  secretd add-genesis-account "$(secretcli keys show -a b)" 1000000000000000000uscrt
  secretd add-genesis-account "$(secretcli keys show -a c)" 1000000000000000000uscrt
  secretd add-genesis-account "$(secretcli keys show -a d)" 1000000000000000000uscrt


  secretd gentx --name a --keyring-backend test --amount 1000000uscrt
  secretd gentx --name b --keyring-backend test --amount 1000000uscrt
  secretd gentx --name c --keyring-backend test --amount 1000000uscrt
  secretd gentx --name d --keyring-backend test --amount 1000000uscrt

  secretd collect-gentxs
  secretd validate-genesis

  secretd init-bootstrap
  secretd validate-genesis
fi

secretcli rest-server --trust-node=true --chain-id enigma-pub-testnet-3 --laddr tcp://0.0.0.0:1336 &
lcp --proxyUrl http://localhost:1336 --port 1337 --proxyPartial '' &

# sleep infinity
source /opt/sgxsdk/environment && RUST_BACKTRACE=1 secretd start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap