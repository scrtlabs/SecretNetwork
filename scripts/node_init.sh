#!/bin/bash
set -x
set -oe errexit

export SGX_MODE=SW
export SGX_DEBUG=0
export SGX_DOCKER_RUN=43
LOG_LEVEL=${LOG_LEVEL:-"debug"}

ENABLE_FAUCET=${1:-"true"}
KEYRING=${KEYRING:-"test"}
MONIKER=${MONIKER:-"banana"}
custom_script_path=${POST_INIT_SCRIPT:-"/root/post_init.sh"}

if [ -v ${SCRT_RPC_IP} ]; then
  echo "Set SCRT_RPC_IP to point to the network interface to bind the rpc service to"
  exit 1
fi
RPC_URL="${SCRT_RPC_IP}:26657"
FAUCET_URL="${SCRT_RPC_IP}:5000"

SCRT_HOME=${SECRETD_HOME:-$HOME/.secretd}
SCRT_SGX_STORAGE=/opt/secret/.sgx_secrets
if [ -v ${SCRT_ENCLAVE_DIR} ]; then
  echo "SCRT_ENCLAVE_DIR is not set"
  exit 1
fi
# SCRT_ENCLAVE_DIR=/usr/lib

GENESIS_file=${SCRT_HOME}/config/genesis.json
if [ ! -e $GENESIS_file ]; then
  # No genesis file found. Fresh start. Clean up
  rm -rf $SCRT_HOME
  rm -rf $SCRT_SGX_STORAGE

  mkdir -p $SCRT_SGX_STORAGE

  chain_id=${CHAINID:-"secretdev-1"}
  fast_blocks=${FAST_BLOCKS:-"false"}

  secretd config set client chain-id ${chain_id}
  secretd config set client output json
  secretd config set client keyring-backend ${KEYRING}

  secretd init ${MONIKER} --chain-id ${chain_id}

  cp ~/node_key.json ${SCRT_HOME}/config/node_key.json

  cat ${SCRT_HOME}/config/genesis.json | jq '
    .consensus_params.block.time_iota_ms = "10" |
    .app_state.staking.params.unbonding_time = "90s" |
    .app_state.gov.voting_params.voting_period = "90s" |
    .app_state.crisis.constant_fee.denom = "uscrt" |
    .app_state.gov.deposit_params.min_deposit[0].denom = "uscrt" |
    .app_state.mint.params.mint_denom = "uscrt" |
    .app_state.staking.params.bond_denom = "uscrt"
  ' > ${SCRT_HOME}/config/genesis.json.tmp
  mv ${SCRT_HOME}/config/genesis.json{.tmp,}

  if [ "${fast_blocks}" = "true" ]; then
    sed -E -i '/timeout_(propose|prevote|precommit|commit)/s/[0-9]+m?s/200ms/' ~/.secretd/config/config.toml
  fi

  if [ -e "$custom_script_path" ]; then
    echo "Running custom post init script..."
    bash $custom_script_path
    echo "Done running custom script!"
  fi

  # Setup LCD
  perl -i -pe 's;address = "tcp://0.0.0.0:1317";address = "tcp://0.0.0.0:1316";' ~/.secretd/config/app.toml
  perl -i -pe 's/enable-unsafe-cors = false/enable-unsafe-cors = true/' ~/.secretd/config/app.toml
  perl -i -pe 's/concurrency = false/concurrency = true/' ~/.secretd/config/app.toml

  # Prevent max connections error
  perl -i -pe 's/max_subscription_clients.+/max_subscription_clients = 100/' ~/.secretd/config/config.toml
  perl -i -pe 's/max_subscriptions_per_client.+/max_subscriptions_per_client = 50/' ~/.secretd/config/config.toml
fi

# CORS bypass proxy [if missing, install via npm: npm install -g local-cors-proxy]
setsid lcp --proxyUrl http://localhost:1316 --port 1317 --proxyPartial '' &

. ./node_start.sh
