#!/usr/bin/bash
set -x
set -oe errexit
export SGX_MODE=SW
export SCRT_CHAINID=${SCRT_CHAINID:-secretdev-1}
SCRT_MONIKER=${SCRT_MONIKER:-scrt}
SECRETD=${SECRETD:-/usr/local/bin/secretd}
SCRT_ENVLAVE_DIR=${SCRT_ENCLAVE_DIR:-"/usr/local/lib/scrt"}
SCRT_KEYRING=${SCRT_KEYRING:-"test"}
SCRT_HOME=${SCRT_HOME:-${HOME}/.secretd}
SCRT_SGX_STORAGE=${SCRT_SGX_STORAGE:-"/opt/secret/.sgx_secrets"}

if [ -v ${SCRT_BOOTSTRAP} ]; then
	echo "Please set SCRT_BOOTSTRAP to point to bootstrap node ip"
	exit 1
fi
if [ -v ${SCRT_BOOTSTRAP_NODE_ID} ]; then
	echo "Please set SCRT_BOOTSTRAP_NODE_ID to point to bootstrap node id"
	exit 1
fi
mkdir -p ${SCRT_SGX_STORAGE}

_pid_=$(ps -ef | grep "lcp --proxyUrl" | grep -v grep | awk '{print $2}')
if [ ! -z "${_pid_}" ]; then
    echo "Faucet app is running with PID:${_pid_}. Stopping..."
    kill -HUP ${_pid_} && echo "Successfully stopped PID:" {$_pid_}
fi

# Full clean up of the node before start
rm -fr ${SCRT_HOME}
echo "Init SGX enclave"
secretd reset-enclave
secretd init-enclave

# configure to use bootstrap node
secretd config set client chain-id ${SCRT_CHAINID}
secretd config set client output json
secretd config set client keyring-backend ${SCRT_KEYRING}
${SECRETD} init ${SCRT_MONIKER} --chain-id ${SCRT_CHAINID}
secretd config set client node tcp://${SCRT_BOOTSTRAP}:26657
secretd config set client output json
# expect genesis.json node_key.json priv_validator_key.json
ls -l -1 $SCRT_HOME/config/*.json

if [ -v $SCRT_BOOTSTRAP ]; then
	echo "Please set up SCRT_BOOTSTRAP env to point to a bootstrap node <ip:26657>"
	exit 1
fi
# get genesis from bootstrap node
curl http://${SCRT_BOOTSTRAP}:26657/genesis | jq '.result.genesis' > ${SCRT_HOME}/config/genesis.json
if [ ! -s ${SCRT_HOME}/config/genesis.json ]; then
	echo "Empty/non-existant genesis"
	exit 1
fi

cat ${SCRT_HOME}/config/genesis.json | sha256sum

ls -lh ${SCRT_SGX_STORAGE}/attestation_cert.der

echo "Verify the certificate is valid"
# Extract public key from certificate
PUBLIC_KEY=$(secretd parse $SCRT_SGX_STORAGE/attestation_cert.der 2>/dev/null | cut -c 3-)
echo "Public key: ${PUBLIC_KEY}"

# fund wallet
THIS=$(readlink -f "${BASH_SOURCE[0]}" 2>/dev/null || echo $0)
DIR=$(dirname "${THIS}")
. "$DIR/create_keys.sh"

# Setup LCD
perl -i -pe 's;address = "tcp://localhost:1317";address = "tcp://0.0.0.0:1316";' ${SCRT_HOME}/config/app.toml
perl -i -pe 's;address = "tcp://localhost:9090";address = "tcp://0.0.0.0:9090";' ${SCRT_HOME}/config/app.toml
perl -i -pe 's/enable-unsafe-cors = false/enable-unsafe-cors = true/' ${SCRT_HOME}/config/app.toml
perl -i -pe 's/concurrency = false/concurrency = true/' ${SCRT_HOME}/config/app.toml
perl -i -pe 's;laddr = "tcp://127.0.0.1:26657";laddr = "tcp://0.0.0.0:26657";' ${SCRT_HOME}/config/config.toml

# Prevent max connections error
perl -i -pe 's/max_subscription_clients.+/max_subscription_clients = 100/' ${SCRT_HOME}/config/config.toml
perl -i -pe 's/max_subscriptions_per_client.+/max_subscriptions_per_client = 50/' ${SCRT_HOME}/config/config.toml

CreateKeys
SCRT_WALLET=a
secretd tx register auth ${SCRT_SGX_STORAGE}/attestation_combined.bin -y --fees 3000uscrt --from ${SCRT_WALLET} --chain-id ${SCRT_CHAINID}

# txhash=$(secretd tx register auth ${SCRT_SGX_STORAGE}/attestation_combined.bin -y --fees 3000uscrt --from ${SCRT_WALLET} --chain-id ${SCRT_CHAINID} | jq '.txhash' | tr -d '"')
sleep 5s
# secretd q tx --type hash ${txhash}
# pull and check node encryption seed from the network
SEED=$(secretd q register seed ${PUBLIC_KEY} | cut -c 3-)
echo ${SEED}

cat ${SCRT_HOME}/config/genesis.json | jq
echo "^^^^^^^^^^^ GENESIS ^^^^^^^^^^"
sleep 5s

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

 mv ${SCRT_HOME}/config/genesis.json.tmp ${SCRT_HOME}/config/genesis.json

 if [ ! -s ${SCRT_HOME}/config/genesis.json ]; then
	ls -l ${SCRT_HOME}/config/genesis.json
	echo "Empty/non-existant genesis"
	exit 1
 fi

secretd q register secret-network-params
ls -lh ./io-master-key.txt ./node-master-key.txt
mkdir -p ${SCRT_HOME}/keys
cp ./io-master-key.txt ./node-master-key.txt ${SCRT_HOME}/keys/

echo "--- Configure Secret Node ---"
mkdir -p ${SCRT_HOME}/.node
secretd configure-secret ${SCRT_HOME}/keys/node-master-key.txt ${SEED}

# add Seeds And Persistent Peers To Configuration File.
# Need: SCRT_BOOTSTRAP_NODE_ID and SCRT_BOOTSTRAP
# obtain bootstrap node id by running on bootstrap : secretd tendermint 
perl -i -pe 's/seeds = ""/seeds = "$ENV{SCRT_BOOTSTRAP_NODE_ID}\@$ENV{SCRT_BOOTSTRAP}:26656"/' ${SCRT_HOME}/config/config.toml

perl -i -pe 's/persistent_peers = ""/persistent_peers = "$ENV{SCRT_BOOTSTRAP_NODE_ID}\@$ENV{SCRT_BOOTSTRAP}:26656"/' ${SCRT_HOME}/config/config.toml

sed -i.bak -e "s/^contract-memory-enclave-cache-size *=.*/contract-memory-enclave-cache-size = \"15\"/" ${SCRT_HOME}/config/app.toml

# set minimum-gas-price
# node will not accept transactions that specify --fees lower than the minimun-gas-price
perl -i -pe 's/^minimum-gas-prices = .+?$/minimum-gas-prices = "0.0125uscrt"/' ${SCRT_HOME}/config/app.toml

# Get Node ID
secretd tendermint show-node-id

# no need to use bootstrap node at this point - point to local secretd
secretd config set client node tcp://0.0.0.0:26657

# CORS bypass proxy [if missing, install via npm: npm install -g local-cors-proxy]
nohup lcp --proxyUrl http://0.0.0.0:1316 --port 1317 --proxyPartial '' &
# setsid lcp --proxyUrl http://0.0.0.0:1316 --port 1317 --proxyPartial '' &
# Start CORs proxy right before secretd
nohup secretd start --rpc.laddr tcp://0.0.0.0:26657 &> secretd.full.log &



