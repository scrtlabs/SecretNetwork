#!/bin/sh

set -o errexit

SECRETD=${1:-./secretd}
SECRETD_HOME=${2:-$HOME/.secretd_local}
CHAINID=${3:-secretdev-1}

rm -rf $SECRETD_HOME

# Build genesis file incl account for passed address
coins="10000000000uscrt,100000000000stake"
$SECRETD init --chain-id $CHAINID $CHAINID --home $SECRETD_HOME
$SECRETD keys add validator --keyring-backend="test" --home $SECRETD_HOME
$SECRETD add-genesis-account $($SECRETD keys show validator -a --keyring-backend="test" --home $SECRETD_HOME) $coins --home $SECRETD_HOME

a_mnemonic="grant rice replace explain federal release fix clever romance raise often wild taxi quarter soccer fiber love must tape steak together observe swap guitar"
b_mnemonic="jelly shadow frog dirt dragon use armed praise universe win jungle close inmate rain oil canvas beauty pioneer chef soccer icon dizzy thunder meadow"
c_mnemonic="chair love bleak wonder skirt permit say assist aunt credit roast size obtain minute throw sand usual age smart exact enough room shadow charge"
d_mnemonic="word twist toast cloth movie predict advance crumble escape whale sail such angry muffin balcony keen move employ cook valve hurt glimpse breeze brick"
echo $a_mnemonic | $SECRETD keys add a --recover --keyring-backend="test" --home $SECRETD_HOME
echo $b_mnemonic | $SECRETD keys add b --recover --keyring-backend="test" --home $SECRETD_HOME
echo $c_mnemonic | $SECRETD keys add c --recover --keyring-backend="test" --home $SECRETD_HOME
echo $d_mnemonic | $SECRETD keys add d --recover --keyring-backend="test" --home $SECRETD_HOME

$SECRETD add-genesis-account "$($SECRETD keys show -a a --keyring-backend="test" --home $SECRETD_HOME)" $coins --home $SECRETD_HOME
$SECRETD add-genesis-account "$($SECRETD keys show -a b --keyring-backend="test" --home $SECRETD_HOME)" $coins --home $SECRETD_HOME
$SECRETD add-genesis-account "$($SECRETD keys show -a c --keyring-backend="test" --home $SECRETD_HOME)" $coins --home $SECRETD_HOME
$SECRETD add-genesis-account "$($SECRETD keys show -a d --keyring-backend="test" --home $SECRETD_HOME)" $coins --home $SECRETD_HOME

if [ ! -z "$2" ]; then
  $SECRETD add-genesis-account $GENACCT $coins
fi

$SECRETD gentx validator 5000000000uscrt --keyring-backend="test" --chain-id $CHAINID --home $SECRETD_HOME
$SECRETD collect-gentxs --home $SECRETD_HOME

$SECRETD init-bootstrap node-master-cert.der io-master-cert.der --home $SECRETD_HOME

# Set proper defaults and change ports
sed -i 's#"tcp://127.0.0.1:26657"#"tcp://0.0.0.0:26657"#g' $SECRETD_HOME/config/config.toml
#sed -i 's/timeout_commit = "5s"/timeout_commit = "1s"/g' $SECRETD_HOME/config/config.toml
#sed -i 's/timeout_propose = "3s"/timeout_propose = "1s"/g' $SECRETD_HOME/config/config.toml
#sed -i 's/index_all_keys = false/index_all_keys = true/g' $SECRETD_HOME/config/config.toml
perl -i -pe 's/rpc-read-timeout = 600/rpc-read-timeout = 5/' $SECRETD_HOME/config/app.toml
perl -i -pe 's/rpc-write-timeout = 600/rpc-read-timeout = 5/' $SECRETD_HOME/config/app.toml

perl -i -pe 's/"stake"/ "uscrt"/g' $SECRETD_HOME/config/genesis.json

# Start the secretd
LOG_LEVEL=trace $SECRETD start --pruning=nothing --bootstrap --home $SECRETD_HOME
