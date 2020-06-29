#!/usr/bin/env bash

set -euv

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

mkdir -p /root/.secretd/.node

# secretd init "$(hostname)" --chain-id enigma-testnet || true

secretd init "$(hostname)" --chain-id "$CHAINID"
echo "Initializing chain: $CHAINID with node moniker: $(hostname)"


sed -i 's/persistent_peers = ""/persistent_peers = "'"$PERSISTENT_PEERS"'"/g' ~/.secretd/config/config.toml
echo "Set persistent_peers: $PERSISTENT_PEERS"

echo "Waiting for bootstrap to start..."
sleep 10

secretd init-enclave

PUBLIC_KEY=$(secretd parse attestation_cert.der 2> /dev/null | cut -c 3- )

echo "Public key: $(secretd parse attestation_cert.der 2> /dev/null | cut -c 3- )"

secretcli tx register auth attestation_cert.der --node "$RPC_URL" -y --from a

sleep 5

SEED=$(secretcli q register seed "$PUBLIC_KEY" --node "$RPC_URL" 2> /dev/null | cut -c 3-)
echo "SEED: $SEED"

secretcli q register secret-network-params --node "$RPC_URL" 2> /dev/null

secretd configure-secret node-master-cert.der "$SEED"

curl http://"$RPC_URL"/genesis | jq -r .result.genesis > /root/.secretd/config/genesis.json

echo "Downloaded genesis file from: $GENESISPATH.."

secretd validate-genesis

RUST_BACKTRACE=1 secretd start