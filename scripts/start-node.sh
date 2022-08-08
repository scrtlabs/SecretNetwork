#!/bin/sh

set -o errexit

CHAINID=$1
GENACCT=$2

if [ -z "$1" ]; then
  echo "Need to input chain id..."
  exit 1
fi

rm -rf ~/.secretd

# Build genesis file incl account for passed address
coins="10000000000uscrt,100000000000stake"
secretd init --chain-id $CHAINID $CHAINID
secretd keys add validator --keyring-backend="test"
secretd add-genesis-account $(secretd keys show validator -a --keyring-backend="test") $coins

if [ ! -z "$2" ]; then
  secretd add-genesis-account $GENACCT $coins
fi

secretd gentx validator 5000000000uscrt --keyring-backend="test" --chain-id $CHAINID
secretd collect-gentxs

# Set proper defaults and change ports
sed -i 's#"tcp://127.0.0.1:26657"#"tcp://0.0.0.0:26657"#g' ~/.secretd/config/config.toml
sed -i 's/timeout_commit = "5s"/timeout_commit = "1s"/g' ~/.secretd/config/config.toml
sed -i 's/timeout_propose = "3s"/timeout_propose = "1s"/g' ~/.secretd/config/config.toml
sed -i 's/index_all_keys = false/index_all_keys = true/g' ~/.secretd/config/config.toml
perl -i -pe 's/"stake"/ "uscrt"/g' ~/.secretd/config/genesis.json

# Start the secretd
secretd start --pruning=nothing --bootstrap