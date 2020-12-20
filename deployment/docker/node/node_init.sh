#!/usr/bin/env bash
set -euv

# REGISTRATION_SERVICE=
# RPC_URL=http://bootstrap:26657
# CHAINID=secret-testnet-1
# PERSISTENT_PEERS=115aa0a629f5d70dd1d464bc7e42799e00f4edae@bootstrap:26656

# init the node
# rm -rf ~/.secret*
#secretcli config chain-id enigma-testnet
#secretcli config output json
#secretcli config indent true
#secretcli config trust-node true
#secretcli config keyring-backend test
# rm -rf ~/.secretd
file=/root/.secretd/config/attestation_cert.der
if [ ! -e "$file" ]
then
  rm -rf ~/.secretd/* || true

  mkdir -p /root/.secretd/.node

  # secretd init "$(hostname)" --chain-id enigma-testnet || true

  secretd init "$MONIKER" --chain-id "$CHAINID"
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

  PUBLIC_KEY=$(secretd parse attestation_cert.der 2> /dev/null | cut -c 3- )

  echo "Public key: $(secretd parse attestation_cert.der 2> /dev/null | cut -c 3- )"

  cp attestation_cert.der /root/.secretd/config/

  openssl base64 -A -in attestation_cert.der -out b64_cert
  # secretcli tx register auth attestation_cert.der --node "$RPC_URL" -y --from a
  curl -G --data-urlencode "cert=$(cat b64_cert)" http://"$REGISTRATION_SERVICE"/register

  sleep 20

  SEED=$(secretcli q register seed "$PUBLIC_KEY" --node tcp://"$RPC_URL" 2> /dev/null | cut -c 3-)
  echo "SEED: $SEED"

  secretcli q register secret-network-params --node tcp://"$RPC_URL" 2> /dev/null

  secretd configure-secret node-master-cert.der "$SEED"

  curl http://"$RPC_URL"/genesis | jq -r .result.genesis > /root/.secretd/config/genesis.json

  echo "Downloaded genesis file from $RPC_URL "

  secretd validate-genesis

fi
secretd start
