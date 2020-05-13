#!/bin/bash

set -euv

# init the node
# rm -rf ~/.enigma*
#enigmacli config chain-id enigma-testnet
#enigmacli config output json
#enigmacli config indent true
#enigmacli config trust-node true
#enigmacli config keyring-backend test
rm -rf ~/.enigmad

mkdir -p /root/.enigmad/.node

enigmad init $(hostname) --chain-id enigma-testnet || true

PERSISTENT_PEERS=115aa0a629f5d70dd1d464bc7e42799e00f4edae@bootstrap:26656

sed -i 's/persistent_peers = ""/persistent_peers = "'$PERSISTENT_PEERS'"/g' ~/.enigmad/config/config.toml
echo "Set persistent_peers: $PERSISTENT_PEERS"

echo "Waiting for bootstrap to start..."
sleep 10

MASTER_KEY="$(enigmacli q register master-key --node http://bootstrap:26657 2> /dev/null | cut -c 3- )"

echo "Master key: $MASTER_KEY"

enigmad init-enclave

PUBLIC_KEY="$(enigmad parse attestation_cert.der 2> /dev/null | cut -c 3- )"

echo "Public key: $PUBLIC_KEY"

enigmacli tx register auth attestation_cert.der --node http://bootstrap:26657 -y --from a

sleep 5

SEED=$(enigmacli q register seed $PUBLIC_KEY --node http://bootstrap:26657 2> /dev/null | cut -c 3-)
echo "SEED: $SEED"

$(enigmad configure-secret $MASTER_KEY $SEED)

RUST_BACKTRACE=1 enigmad start