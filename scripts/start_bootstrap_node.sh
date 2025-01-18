#!/bin/bash
set -x
set -oe errexit

export SGX_MODE=SW
SGX_DEBUG=0
LOG_LEVEL=${LOG_LEVEL:-"info"}

ENABLE_FAUCET=${ENABLE_FAUCET:-"true"}
KEYRING=${SCRT_KEYRING:-"test"}
MONIKER=${SCRT_MONIKER:-"scrt"}

RPC_URL="0.0.0.0:26657"
FAUCET_URL="0.0.0.0:5000"

SCRT_HOME=${SECRETD_HOME:-$HOME/.secretd}
SCRT_SGX_STORAGE=${SCRT_SGX_STORAGE:-"/opt/secret/.sgx_secrets"}
if [ -v ${SCRT_ENCLAVE_DIR} ]; then
  echo "Please set SCRT_ENCLAVE_DIR to the location of SecretNetwork signed shared libraries"
  exit 1
fi

THIS=$(readlink -f "${BASH_SOURCE[0]}" 2>/dev/null || echo $0)
DIR=$(dirname "${THIS}")
. "$DIR/create_keys.sh"
 
GENESIS_file=${SCRT_HOME}/config/genesis.json
rm -fr ${SCRT_HOME}
if [ ! -e $GENESIS_file ]; then
  # No genesis file found. Fresh start. Clean up
  rm -rf $SCRT_HOME
  secretd reset-enclave
  secretd init-enclave

  chain_id=${SCRT_CHAINID:-"secretdev-1"}
  fast_blocks=${FAST_BLOCKS:-"false"}

  secretd config set client chain-id ${chain_id}
  secretd config set client output json
  secretd config set client keyring-backend ${KEYRING}

  secretd init ${MONIKER} --chain-id ${chain_id}
  # expect genesis.json node_key.json priv_validator_key.json
  ls -l -1 $SCRT_HOME/config/*.json
  cat ${SCRT_HOME}/config/genesis.json | sha256sum

  cat ${SCRT_HOME}/config/genesis.json | jq '
    .app_state.staking.params.unbonding_time = "90s" |
    .app_state.gov.params.voting_period = "90s" |
    .app_state.gov.params.expedited_voting_period = "15s" |
    .app_state.crisis.constant_fee.denom = "uscrt" |
    .app_state.gov.deposit_params.min_deposit[0].denom = "uscrt" |
    .app_state.gov.params.min_deposit[0].denom = "uscrt" |
    .app_state.gov.params.expedited_min_deposit[0].denom = "uscrt" |
    .app_state.mint.params.mint_denom = "uscrt" |
    .app_state.staking.params.bond_denom = "uscrt"
  ' > ${SCRT_HOME}/config/genesis.json.tmp
  mv ${SCRT_HOME}/config/genesis.json{.tmp,}

  if [ ! -s ${SCRT_HOME}/config/genesis.json ]; then
	  echo "Empty/non-existant genesis"
	  exit 1
  fi

  if [ "${fast_blocks}" = "true" ]; then
    sed -E -i '/timeout_(propose|prevote|precommit|commit)/s/[0-9]+m?s/200ms/' ${SCRT_HOME}/config/config.toml
  fi

  if [ -e "$custom_script_path" ]; then
    echo "Running custom post init script..."
    bash $custom_script_path
    echo "Done running custom script!"
  fi

  # Setup LCD
  perl -i -pe 's;address = "tcp://localhost:1317";address = "tcp://0.0.0.0:1316";' ${SCRT_HOME}/config/app.toml
  perl -i -pe 's;address = "localhost:9090";address = "0.0.0.0:9090";' ${SCRT_HOME}/config/app.toml
  perl -i -pe 's/enable-unsafe-cors = false/enable-unsafe-cors = true/' ${SCRT_HOME}/config/app.toml
  perl -i -pe 's/concurrency = false/concurrency = true/' ${SCRT_HOME}/config/app.toml
  perl -i -pe 's;laddr = "tcp://127.0.0.1:26657";laddr = "tcp://0.0.0.0:26657";' ${SCRT_HOME}/config/config.toml

  # Prevent max connections error
  perl -i -pe 's/max_subscription_clients.+/max_subscription_clients = 100/' ${SCRT_HOME}/config/config.toml
  perl -i -pe 's/max_subscriptions_per_client.+/max_subscriptions_per_client = 50/' ${SCRT_HOME}/config/config.toml
fi
 
# kill faucet if still running
if [ ${ENABLE_FAUCET} = "true" ]; then
      _pid_=$(ps -ef | grep node.*faucet.* | grep -v grep | awk '{print $2}')
      if [ ! -z "${_pid_}" ]; then
            echo "Faucet app is running with PID:${_pid_}. Stopping..."
            kill -HUP ${_pid_} && echo "Successfully stopped PID:" {$_pid_}
      fi
fi

_pid_=$(ps -ef | grep "lcp --proxyUrl" | grep -v grep | awk '{print $2}')
if [ ! -z "${_pid_}" ]; then
    echo "COR prody is running with PID:${_pid_}. Stopping..."
    kill -HUP ${_pid_} && echo "Successfully stopped PID:" {$_pid_}
fi

sleep 5s


# Create keys
CreateKeys

# Preload genesis accounts with funds
ico=9000000000000000000000

echo "---------- PRELOAD GENESIS ACCOUNTS ----------"
secretd genesis add-genesis-account validator ${ico}uscrt
secretd genesis add-genesis-account a ${ico}uscrt
secretd genesis add-genesis-account b ${ico}uscrt
secretd genesis add-genesis-account c ${ico}uscrt
secretd genesis add-genesis-account d ${ico}uscrt
secretd genesis add-genesis-account x ${ico}uscrt
secretd genesis add-genesis-account z ${ico}uscrt

# Genesis tx
secretd genesis gentx validator ${ico}uscrt --chain-id "$chain_id"

# Collect and validate
secretd genesis collect-gentxs
secretd genesis validate-genesis

# generate node master keys
# secretd q register secret-network-params
# ls -lh ./io-master-key.txt ./node-master-key.txt
# mkdir -p ${SCRT_HOME}/keys
# cp ./io-master-key.txt ./node-master-key.txt ${SCRT_HOME}/keys/

secretd init-bootstrap ./node-master-key.txt ./io-master-key.txt

if [ "${ENABLE_FAUCET}" = "true" ]; then
      # Setup faucet
      setsid $(which node) ${DIR}/faucet/faucet_server.js &
fi

# CORS bypass proxy [if missing, install via npm: npm install -g local-cors-proxy]
nohup lcp --proxyUrl http://0.0.0.0:1316 --port 1317 --proxyPartial '' &
# setsid lcp --proxyUrl http://0.0.0.0:1316 --port 1317 --proxyPartial '' &

nohup secretd start --rpc.laddr tcp://0.0.0.0:26657 --bootstrap --log_level ${LOG_LEVEL} &> secretd.bootstrap.log &
