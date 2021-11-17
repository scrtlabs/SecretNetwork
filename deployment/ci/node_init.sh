#!/usr/bin/env bash

set -euvo pipefail

# init the node
# rm -rf ~/.secret*
#secretcli config chain-id enigma-testnet
#secretcli config output json
#secretcli config indent true
#secretcli config trust-node true
#secretcli config keyring-backend test
rm -rf ~/.secretd

mkdir -p /root/.secretd/.node
secretd config keyring-backend test
secretd config node tcp://bootstrap:26657
secretd config chain-id secretdev-1

secretd init "$(hostname)" --chain-id secretdev-1 || true

PERSISTENT_PEERS="115aa0a629f5d70dd1d464bc7e42799e00f4edae@bootstrap:26656"

sed -i 's/persistent_peers = ""/persistent_peers = "'$PERSISTENT_PEERS'"/g' ~/.secretd/config/config.toml
echo "Set persistent_peers: $PERSISTENT_PEERS"

echo "Waiting for bootstrap to start..."
sleep 20

cp /tmp/.secretd/keyring-test /root/.secretd/ -r

secretd init-enclave

PUBLIC_KEY=$(secretd parse /opt/secret/.sgx_secrets/attestation_cert.der 2> /dev/null | cut -c 3- )

echo "Public key: $(secretd parse /opt/secret/.sgx_secrets/attestation_cert.der 2> /dev/null | cut -c 3- )"

secretd tx register auth /opt/secret/.sgx_secrets/attestation_cert.der -y --from a --gas-prices 0.25uscrt

sleep 10

SEED=$(secretd q register seed "$PUBLIC_KEY" 2> /dev/null | cut -c 3-)
echo "SEED: $SEED"

secretd q register secret-network-params 2> /dev/null

secretd configure-secret node-master-cert.der "$SEED"

cp /tmp/.secretd/config/genesis.json /root/.secretd/config/genesis.json

secretd validate-genesis

secretd config node tcp://localhost:26657

RUST_BACKTRACE=1 secretd start &

./wasmi-sgx-test.sh
