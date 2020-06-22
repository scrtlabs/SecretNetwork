#!/usr/bin/env bash

set -euv

# RPC_URL=http://bootstrap:26657
# CHAINID=secret-testnet-1
# PERSISTENT_PEERS=115aa0a629f5d70dd1d464bc7e42799e00f4edae@bootstrap:26656

# init the node
# rm -rf ~/.enigma*
#enigmacli config chain-id enigma-testnet
#enigmacli config output json
#enigmacli config indent true
#enigmacli config trust-node true
#enigmacli config keyring-backend test
rm -rf ~/.enigmad

mkdir -p /root/.enigmad/.node

# enigmad init "$(hostname)" --chain-id enigma-testnet || true

enigmad init "$(hostname)" --chain-id "$CHAINID"
echo "Initializing chain: $CHAINID with node moniker: $(hostname)"


sed -i 's/persistent_peers = ""/persistent_peers = "'"$PERSISTENT_PEERS"'"/g' ~/.enigmad/config/config.toml
echo "Set persistent_peers: $PERSISTENT_PEERS"

echo "Waiting for bootstrap to start..."
sleep 10

enigmad init-enclave

PUBLIC_KEY=$(enigmad parse attestation_cert.der 2> /dev/null | cut -c 3- )

echo "Public key: $(enigmad parse attestation_cert.der 2> /dev/null | cut -c 3- )"

enigmacli tx register auth attestation_cert.der --node "$RPC_URL" -y --from a

sleep 5

SEED=$(enigmacli q register seed "$PUBLIC_KEY" --node "$RPC_URL" 2> /dev/null | cut -c 3-)
echo "SEED: $SEED"

enigmacli q register secret-network-params --node "$RPC_URL" 2> /dev/null

enigmad configure-secret node-master-cert.der "$SEED"

curl http://"$RPC_URL"/genesis | jq -r .result > /root/.enigmad/config/genesis.json

echo "Downloaded genesis file from: $GENESISPATH.."

enigmad validate-genesis

RUST_BACKTRACE=1 enigmad start