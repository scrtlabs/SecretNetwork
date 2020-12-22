#!/usr/bin/env bash

set -euv

# init the node
# rm -rf ~/.secret*
#secretcli config chain-id enigma-testnet
#secretcli config output json
#secretcli config indent true
#secretcli config trust-node true
#secretcli config keyring-backend test
rm -rf ~/.secretd

mkdir -p /root/.secretd/.node

secretd init "$(hostname)" --chain-id enigma-testnet || true

PERSISTENT_PEERS=115aa0a629f5d70dd1d464bc7e42799e00f4edae@bootstrap:26656

sed -i 's/persistent_peers = ""/persistent_peers = "'$PERSISTENT_PEERS'"/g' ~/.secretd/config/config.toml
echo "Set persistent_peers: $PERSISTENT_PEERS"

echo "Waiting for bootstrap to start..."
sleep 20

# MASTER_KEY="$(secretcli q register secret-network-params --node http://bootstrap:26657 2> /dev/null | cut -c 3- )"

#echo "Master key: $MASTER_KEY"

secretd init-enclave

PUBLIC_KEY=$(secretd parse attestation_cert.der 2> /dev/null | cut -c 3- )

echo "Public key: $(secretd parse attestation_cert.der 2> /dev/null | cut -c 3- )"

secretcli tx register auth attestation_cert.der --node http://bootstrap:26657 -y --from a

sleep 10

SEED=$(secretcli q register seed "$PUBLIC_KEY" --node http://bootstrap:26657 2> /dev/null | cut -c 3-)
echo "SEED: $SEED"

secretcli q register secret-network-params --node http://bootstrap:26657 2> /dev/null

secretd configure-secret node-master-cert.der "$SEED"

cp /tmp/.secretd/config/genesis.json /root/.secretd/config/genesis.json

secretd validate-genesis

RUST_BACKTRACE=1 secretd start &

./wasmi-sgx-test.sh