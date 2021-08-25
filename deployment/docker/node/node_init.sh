#!/usr/bin/env bash
set -euv

# REGISTRATION_SERVICE=
# export RPC_URL="bootstrap:26657"
# export CHAINID="secretdev-1"
# export PERSISTENT_PEERS="115aa0a629f5d70dd1d464bc7e42799e00f4edae@bootstrap:26656"

# init the node
# rm -rf ~/.secret*

# rm -rf ~/.secretd
file=/root/.secretd/config/attestation_cert.der
if [ ! -e "$file" ]
then
  rm -rf ~/.secretd/* || true

  # secretcli config chain-id enigma-testnet
#  secretcli config output json
#  secretcli config indent true
#  secretcli config trust-node true


  mkdir -p /root/.secretd/.node
  # secretd config keyring-backend test
  secretd config node tcp://"$RPC_URL"
  secretd config chain-id "$CHAINID"
#  export SECRET_NETWORK_CHAIN_ID=$CHAINID
#  export SECRET_NETWORK_KEYRING_BACKEND=test
  # secretd init "$(hostname)" --chain-id enigma-testnet || true

  secretd init "$MONIKER" --chain-id "$CHAINID"

  # cp /tmp/.secretd/keyring-test /root/.secretd/ -r

  echo "Initializing chain: $CHAINID with node moniker: $(hostname)"

  sed -i 's/persistent_peers = ""/persistent_peers = "'"$PERSISTENT_PEERS"'"/g' ~/.secretd/config/config.toml
  echo "Set persistent_peers: $PERSISTENT_PEERS"

  # Open RPC port to all interfaces
  perl -i -pe 's/laddr = .+?26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' ~/.secretd/config/config.toml

  # Open P2P port to all interfaces
  perl -i -pe 's/laddr = .+?26656"/laddr = "tcp:\/\/0.0.0.0:26656"/' ~/.secretd/config/config.toml

  echo "Waiting for bootstrap to start..."
  sleep 10

  secretd init-enclave

  PUBLIC_KEY=$(secretd parse /opt/secret/.sgx_secrets/attestation_cert.der 2> /dev/null | cut -c 3- )

  echo "Public key: $(secretd parse /opt/secret/.sgx_secrets/attestation_cert.der 2> /dev/null | cut -c 3- )"

  cp /opt/secret/.sgx_secrets/attestation_cert.der /root/.secretd/config/

  openssl base64 -A -in attestation_cert.der -out b64_cert
  # secretd tx register auth attestation_cert.der --from a --gas-prices 0.25uscrt -y

  curl -G --data-urlencode "cert=$(cat b64_cert)" http://"$REGISTRATION_SERVICE"/register

  sleep 20

  SEED=$(secretd q register seed "$PUBLIC_KEY"  2> /dev/null | cut -c 3-)
  echo "SEED: $SEED"

  secretd q register secret-network-params 2> /dev/null

  secretd configure-secret node-master-cert.der "$SEED"

  curl http://"$RPC_URL"/genesis | jq -r .result.genesis > /root/.secretd/config/genesis.json

  echo "Downloaded genesis file from $RPC_URL "

  secretd validate-genesis

  secretd config node tcp://localhost:26657

fi
secretd start
