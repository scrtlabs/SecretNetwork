#!/bin/sh
set -x
set -o errexit

SECRETD=${1:-./secretd}
SECRETD_HOME=${2:-$HOME/.secretd_local}
CHAINID=${3:-secretdev-1}

ENABLE_FAUCET=${1:-"true"}

rm -rf $SECRETD_HOME

if [ ${ENABLE_FAUCET} = "true" ]; then
  _pid_=$(ps -ef | grep node.*faucet.* | grep -v grep | awk '{print $2}')
  if [ ! -z "${_pid_}" ]; then
    echo "Faucet app is running with PID:${_pid_}. Stopping..."
    kill -HUP ${_pid_} && echo "Successfully stopped PID:" {$_pid_}
  fi
fi

$SECRETD config set client chain-id "$CHAINID"
$SECRETD config set client output json
$SECRETD config set client keyring-backend test

# Build genesis file incl account for passed address
#coins="500000000000uscrt,500000000000uscrt"
coins="500000000000uscrt"
$SECRETD init --chain-id $CHAINID $CHAINID --home=$SECRETD_HOME
$SECRETD keys add validator --keyring-backend="test" --home=$SECRETD_HOME

#$SECRETD genesis add-genesis-account $($SECRETD keys show validator -a --keyring-backend="test" --home $SECRETD_HOME) $coins --home $SECRETD_HOME
$SECRETD genesis add-genesis-account validator $coins --keyring-backend="test" --home=$SECRETD_HOME

a_mnemonic="grant rice replace explain federal release fix clever romance raise often wild taxi quarter soccer fiber love must tape steak together observe swap guitar"
b_mnemonic="jelly shadow frog dirt dragon use armed praise universe win jungle close inmate rain oil canvas beauty pioneer chef soccer icon dizzy thunder meadow"
c_mnemonic="chair love bleak wonder skirt permit say assist aunt credit roast size obtain minute throw sand usual age smart exact enough room shadow charge"
d_mnemonic="word twist toast cloth movie predict advance crumble escape whale sail such angry muffin balcony keen move employ cook valve hurt glimpse breeze brick"
echo $a_mnemonic | $SECRETD keys add a --recover --keyring-backend="test" --home=$SECRETD_HOME
echo $b_mnemonic | $SECRETD keys add b --recover --keyring-backend="test" --home=$SECRETD_HOME
echo $c_mnemonic | $SECRETD keys add c --recover --keyring-backend="test" --home=$SECRETD_HOME
echo $d_mnemonic | $SECRETD keys add d --recover --keyring-backend="test" --home=$SECRETD_HOME

# $SECRETD genesis add-genesis-account "$($SECRETD keys show -a a --keyring-backend="test" --home $SECRETD_HOME)" $coins --home $SECRETD_HOME
# $SECRETD genesis add-genesis-account "$($SECRETD keys show -a b --keyring-backend="test" --home $SECRETD_HOME)" $coins --home $SECRETD_HOME
# $SECRETD genesis add-genesis-account "$($SECRETD keys show -a c --keyring-backend="test" --home $SECRETD_HOME)" $coins --home $SECRETD_HOME
# $SECRETD genesis add-genesis-account "$($SECRETD keys show -a d --keyring-backend="test" --home $SECRETD_HOME)" $coins --home $SECRETD_HOME

$SECRETD genesis add-genesis-account a $coins --home=$SECRETD_HOME --keyring-backend="test"
$SECRETD genesis add-genesis-account b $coins --home=$SECRETD_HOME --keyring-backend="test"
$SECRETD genesis add-genesis-account c $coins --home=$SECRETD_HOME --keyring-backend="test"
$SECRETD genesis add-genesis-account d $coins --home=$SECRETD_HOME --keyring-backend="test"

if [ ! -z "$2" ]; then
  $SECRETD genesis add-genesis-account $GENACCT $coins
fi

$SECRETD genesis gentx validator 5000000000uscrt --keyring-backend="test" --chain-id $CHAINID --home=$SECRETD_HOME
$SECRETD genesis collect-gentxs --home=$SECRETD_HOME

$SECRETD init-bootstrap ./node-master-key.txt ./io-master-key.txt --home=$SECRETD_HOME
#$SECRETD init-bootstrap --home $SECRETD_HOME

# Set proper defaults and change ports
sed -i 's#"tcp://127.0.0.1:26657"#"tcp://0.0.0.0:26657"#g' $SECRETD_HOME/config/config.toml
#sed -i 's/timeout_commit = "5s"/timeout_commit = "1s"/g' $SECRETD_HOME/config/config.toml
#sed -i 's/timeout_propose = "3s"/timeout_propose = "1s"/g' $SECRETD_HOME/config/config.toml
#sed -i 's/index_all_keys = false/index_all_keys = true/g' $SECRETD_HOME/config/config.toml
perl -i -pe 's/rpc-read-timeout = 600/rpc-read-timeout = 5000/' $SECRETD_HOME/config/app.toml
perl -i -pe 's/rpc-write-timeout = 600/rpc-read-timeout = 5000/' $SECRETD_HOME/config/app.toml

perl -i -pe 's/"stake"/ "uscrt"/g' $SECRETD_HOME/config/genesis.json

if [ "${ENABLE_FAUCET}" = "true" ]; then
  # Setup faucet
  setsid node ./deployment/docker/localsecret/faucet/faucet_server.js &
  # Setup secretcli
  cp $(which ${SECRETD}) $(dirname $(which ${SECRETD}))/secretcli
fi


# Start the secretd
LOG_LEVEL=trace $SECRETD start --pruning=nothing --bootstrap --home=$SECRETD_HOME --log_level=trace
