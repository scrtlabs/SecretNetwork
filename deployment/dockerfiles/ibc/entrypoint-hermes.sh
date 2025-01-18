#! /bin/bash

echo "veryfing balances"
hermes keys balance --chain secretdev-1
hermes keys balance --chain secretdev-2
hermes --config /home/hermes-user/.hermes/alternative-config.toml keys balance --chain secretdev-1
hermes --config /home/hermes-user/.hermes/alternative-config.toml keys balance --chain secretdev-2

echo "creating chain"
hermes create channel --a-chain secretdev-1 --b-chain secretdev-2 --a-port transfer --b-port transfer --new-client-connection --yes

echo "relaying forever"
hermes start
