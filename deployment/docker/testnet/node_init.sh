#!/usr/bin/env bash
set -euv

# REGISTRATION_SERVICE=
# export RPC_URL="bootstrap:26657"
# export CHAINID="secretdev-1"
# export PERSISTENT_PEERS="115aa0a629f5d70dd1d464bc7e42799e00f4edae@bootstrap:26656"

# init the node
# rm -rf ~/.secret*
FORCE_SYNC=${FORCE_SYNC:-}

CHAINID="${CHAINID:-confidential-1}"
MONIKER="${MONIKER:-default}"
STATE_SYNC1="${STATE_SYNC1:-http://bootstrap:26657}"
STATE_SYNC2="${STATE_SYNC2:-${STATE_SYNC1:-http://bootstrap:26657}}"

# rm -rf ~/.secretd
file=/root/.secretd/config/attestation_cert.der
if [ ! -e "$file" ]
then
  rm -rf ~/.secretd/* || true

  mkdir -p /root/.secretd/.node
  secretd config node tcp://"$RPC_URL"
  secretd config chain-id "$CHAINID"
  secretd config keyring-backend test
#  export SECRET_NETWORK_CHAIN_ID=$CHAINID
#  export SECRET_NETWORK_KEYRING_BACKEND=test
  # secretd init "$(hostname)" --chain-id enigma-testnet || true

  secretd init "$MONIKER" --chain-id "$CHAINID"

  b_mnemonic="jelly shadow frog dirt dragon use armed praise universe win jungle close inmate rain oil canvas beauty pioneer chef soccer icon dizzy thunder meadow"

  secretd keys add a
  echo $b_mnemonic | secretd keys add b --recover

  echo "Initializing chain: $CHAINID with node moniker: $(hostname)"

  sed -i 's/persistent_peers = ""/persistent_peers = "'"$PERSISTENT_PEERS"'"/g' ~/.secretd/config/config.toml
  echo "Set persistent_peers: $PERSISTENT_PEERS"

  # Open RPC port to all interfaces
  perl -i -pe 's/laddr = .+?26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' ~/.secretd/config/config.toml

  # Open P2P port to all interfaces
  perl -i -pe 's/laddr = .+?26656"/laddr = "tcp:\/\/0.0.0.0:26656"/' ~/.secretd/config/config.toml
  perl -i -pe 's/concurrency = false/concurrency = true/' ~/.secretd/config/app.toml

  echo "Waiting for bootstrap to start..."
  sleep 10

  secretd init-enclave

  PUBLIC_KEY=$(secretd parse /opt/secret/.sgx_secrets/attestation_cert.der 2> /dev/null | cut -c 3- )

  echo "Public key: $(secretd parse /opt/secret/.sgx_secrets/attestation_cert.der 2> /dev/null | cut -c 3- )"

  curl http://"$FAUCET_URL"/faucet?address=$(secretd keys show -a a)
  # cp /opt/secret/.sgx_secrets/attestation_cert.der ./
  sleep 10
  # openssl base64 -A -in attestation_cert.der -out b64_cert
  # secretd tx register auth attestation_cert.der --from a --gas-prices 0.25uscrt -y

  secretd tx register auth /opt/secret/.sgx_secrets/attestation_cert.der -y --from a --gas-prices 0.25uscrt

  sleep 10

  SEED=$(secretd q register seed "$PUBLIC_KEY"  2> /dev/null | cut -c 3-)
  echo "SEED: $SEED"

  secretd q register secret-network-params 2> /dev/null

  secretd configure-secret node-master-cert.der "$SEED"

  curl http://"$RPC_URL"/genesis | jq -r .result.genesis > /root/.secretd/config/genesis.json

  echo "Downloaded genesis file from $RPC_URL "

  secretd validate-genesis

  secretd config node tcp://localhost:26657

  # this is here to make sure that the node doesn't resync
  cp /opt/secret/.sgx_secrets/attestation_cert.der /root/.secretd/config/

  if [ ! -z "$STATE_SYNC1" ] && [ ! -z "$STATE_SYNC2" ];
  then

    echo "Syncing with state sync"

    LATEST_HEIGHT=$(curl -s $STATE_SYNC1/block | jq -r .result.block.header.height); \
    BLOCK_HEIGHT=$((LATEST_HEIGHT - 2000)); \
    TRUST_HASH=$(curl -s "$STATE_SYNC1/block?height=$BLOCK_HEIGHT" | jq -r .result.block_id.hash)

    echo "State sync node: $STATE_SYNC1,$STATE_SYNC2"
    echo "Trust hash: $TRUST_HASH; Block height: $BLOCK_HEIGHT"

    # enable state sync (this is the only line in the config that uses enable = false. This could change and break everything
    perl -i -pe 's/enable = false/enable = true/' /root/.secretd/config/config.toml

    sed -i.bak -E "s|^(enable[[:space:]]+=[[:space:]]+).*$|\1true| ; \
    s|^(rpc_servers[[:space:]]+=[[:space:]]+).*$|\1\"$STATE_SYNC1,$STATE_SYNC2\"| ; \
    s|^(trust_height[[:space:]]+=[[:space:]]+).*$|\1$BLOCK_HEIGHT| ; \
    s|^(trust_hash[[:space:]]+=[[:space:]]+).*$|\1\"$TRUST_HASH\"|" $HOME/.secretd/config/config.toml
  else
    echo "Syncing with block sync"
  fi

fi
secretd start
