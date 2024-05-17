#!/usr/bin/bash
set +x
set -o errexit

SECRETD=${1:-./secretd}
SECRETD_HOME=${2:-$HOME/.secretd_local}
CHAINID=${3:-secretdev-1}
ENABLE_FAUCET=${4:-"false"}

LOG_LEVEL=${LOG_LEVEL:-"info"}

rm -rf $SECRETD_HOME

if [ ${ENABLE_FAUCET} = "true" ]; then
  _pid_=$(ps -ef | grep node.*faucet.* | grep -v grep | awk '{print $2}')
  if [ ! -z "${_pid_}" ]; then
    echo "Faucet app is running with PID:${_pid_}. Stopping..."
    kill -HUP ${_pid_} && echo "Successfully stopped PID:" {$_pid_}
  fi
fi

THIS=$(readlink -f "${BASH_SOURCE[0]}" 2>/dev/null||echo $0)
DIR=`dirname "${THIS}"`
TMP_DIR=$(mktemp -d -p ${DIR})
if [ ! -d $WORK_DIR ]; then
  echo "Could not create $WORK_DIR"
  exit 1
fi

function cleanup {
    echo "Clean up $TMP_DIR"
    rm -rf "$TMP_DIR"
}

trap cleanup EXIT

$SECRETD config set client chain-id "$CHAINID"
$SECRETD config set client output json
$SECRETD config set client keyring-backend test

# Build genesis file incl account for passed address
#coins="500000000000uscrt,500000000000uscrt"
coins="500000000000uscrt"
$SECRETD init --chain-id $CHAINID $CHAINID --home=$SECRETD_HOME
retVal=$?
if [ $retVal -ne 0 ]; then
  echo "Error => $SECRETD init --chain-id $CHAINID $CHAINID --home=$SECRETD_HOME"
  exit 1
fi

jq '
.consensus_params.block.time_iota_ms = "10" |
.app_state.staking.params.unbonding_time = "90s" |
.app_state.gov.voting_params.voting_period = "90s" |
.app_state.crisis.constant_fee.denom = "uscrt" |
.app_state.gov.deposit_params.min_deposit[0].denom = "uscrt" |
.app_state.mint.params.mint_denom = "uscrt" |
.app_state.staking.params.bond_denom = "uscrt"
' $SECRETD_HOME/config/genesis.json > $SECRETD_HOME/config/genesis.json.tmp
mv ${SECRETD_HOME}/config/genesis.json.tmp ${SECRETD_HOME}/config/genesis.json

v_mnemonic="push certain add next grape invite tobacco bubble text romance again lava crater pill genius vital fresh guard great patch knee series era tonight"
a_mnemonic="grant rice replace explain federal release fix clever romance raise often wild taxi quarter soccer fiber love must tape steak together observe swap guitar"
b_mnemonic="jelly shadow frog dirt dragon use armed praise universe win jungle close inmate rain oil canvas beauty pioneer chef soccer icon dizzy thunder meadow"
c_mnemonic="chair love bleak wonder skirt permit say assist aunt credit roast size obtain minute throw sand usual age smart exact enough room shadow charge"
d_mnemonic="word twist toast cloth movie predict advance crumble escape whale sail such angry muffin balcony keen move employ cook valve hurt glimpse breeze brick"

echo $v_mnemonic | $SECRETD keys add validator --recover --keyring-backend="test" --home=$SECRETD_HOME
echo $a_mnemonic | $SECRETD keys add a --recover --keyring-backend="test" --home=$SECRETD_HOME
echo $b_mnemonic | $SECRETD keys add b --recover --keyring-backend="test" --home=$SECRETD_HOME
echo $c_mnemonic | $SECRETD keys add c --recover --keyring-backend="test" --home=$SECRETD_HOME
echo $d_mnemonic | $SECRETD keys add d --recover --keyring-backend="test" --home=$SECRETD_HOME

address_v=$($SECRETD keys show -a validator --keyring-backend="test" --home $SECRETD_HOME)
address_a=$($SECRETD keys show -a a --keyring-backend="test" --home $SECRETD_HOME)
address_b=$($SECRETD keys show -a b --keyring-backend="test" --home $SECRETD_HOME)
address_c=$($SECRETD keys show -a c --keyring-backend="test" --home $SECRETD_HOME)
address_d=$($SECRETD keys show -a d --keyring-backend="test" --home $SECRETD_HOME)

echo "[*] Account validator: $address_v"
echo "[+] Account         a: $address_a"
echo "[+] Account         b: $address_b"
echo "[+] Account         c: $address_c"
echo "[+] Account         d: $address_d"

$SECRETD genesis add-genesis-account validator $coins --keyring-backend="test" --home=$SECRETD_HOME
retVal=$?
if [ $retVal -ne 0 ]; then
  echo "Error => $SECRETD genesis add-genesis-account validator $coins"
  exit 1
fi

$SECRETD genesis add-genesis-account a $coins --home=$SECRETD_HOME --keyring-backend="test"
retVal=$?
if [ $retVal -ne 0 ]; then
  echo "Error => $SECRETD genesis add-genesis-account a $coins"
  exit 1
fi

$SECRETD genesis add-genesis-account b $coins --home=$SECRETD_HOME --keyring-backend="test"
retVal=$?
if [ $retVal -ne 0 ]; then
  echo "Error => $SECRETD genesis add-genesis-account b $coins"
  exit 1
fi

$SECRETD genesis add-genesis-account c $coins --home=$SECRETD_HOME --keyring-backend="test"
retVal=$?
if [ $retVal -ne 0 ]; then
  echo "Error => $SECRETD genesis add-genesis-account c $coins"
  exit 1
fi

$SECRETD genesis add-genesis-account d $coins --home=$SECRETD_HOME --keyring-backend="test"
retVal=$?
if [ $retVal -ne 0 ]; then
  echo "Error => $SECRETD genesis add-genesis-account d $coins"
  exit 1
fi

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
SGX_MODE=SW RUST_BACKTRACE=1 SKIP_LIGHT_CLIENT_VALIDATION=true $SECRETD start --pruning=nothing --bootstrap --home=$SECRETD_HOME --log_level ${LOG_LEVEL}
