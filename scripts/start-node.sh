#!/bin/sh

set -o errexit

SECRETD=${1:-./secretd}
SECRETD_HOME=${2:-$HOME/.secretd_local}
CHAINID=${3:-secretdev-1}
GENACCT=$4

rm -rf $SECRETD_HOME

# Build genesis file incl account for passed address
coins="10000000000uscrt,100000000000stake"
$SECRETD init --chain-id $CHAINID $CHAINID --home $SECRETD_HOME
$SECRETD keys add validator --keyring-backend="test" --home $SECRETD_HOME
$SECRETD add-genesis-account $($SECRETD keys show validator -a --keyring-backend="test" --home $SECRETD_HOME) $coins --home $SECRETD_HOME

if [ ! -z "$2" ]; then
  $SECRETD add-genesis-account $GENACCT $coins
fi

$SECRETD gentx validator 5000000000uscrt --keyring-backend="test" --chain-id $CHAINID --home $SECRETD_HOME
$SECRETD collect-gentxs --home $SECRETD_HOME

# Set proper defaults and change ports
sed -i 's#"tcp://127.0.0.1:26657"#"tcp://0.0.0.0:26657"#g' $SECRETD_HOME/config/config.toml
sed -i 's/timeout_commit = "5s"/timeout_commit = "1s"/g' $SECRETD_HOME/config/config.toml
sed -i 's/timeout_propose = "3s"/timeout_propose = "1s"/g' $SECRETD_HOME/config/config.toml
sed -i 's/index_all_keys = false/index_all_keys = true/g' $SECRETD_HOME/config/config.toml
perl -i -pe 's/"stake"/ "uscrt"/g' $SECRETD_HOME/config/genesis.json

# Start the secretd
$SECRETD start --pruning=nothing --bootstrap --home $SECRETD_HOME