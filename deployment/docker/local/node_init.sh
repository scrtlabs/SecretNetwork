#!/usr/bin/env bash

set -euvo pipefail

# init the node
# rm -rf ~/.secret*
#secretcli config chain-id enigma-testnet
#secretcli config output json
#secretcli config indent true
#secretcli config trust-node true
#secretcli config keyring-backend test
# rm -rf ~/.secretd

mkdir -p /root/.secretd/.node
secretd config keyring-backend test
secretd config node http://bootstrap:26657
secretd config chain-id enigma-pub-testnet-3

mkdir -p /root/.secretd/.node

secretd init "$(hostname)" --chain-id enigma-pub-testnet-3 || true

PERSISTENT_PEERS=115aa0a629f5d70dd1d464bc7e42799e00f4edae@bootstrap:26656

sed -i 's/persistent_peers = ""/persistent_peers = "'$PERSISTENT_PEERS'"/g' ~/.secretd/config/config.toml
sed -i 's/trust_period = "168h0m0s"/trust_period = "168h"/g' ~/.secretd/config/config.toml
echo "Set persistent_peers: $PERSISTENT_PEERS"

echo "Waiting for bootstrap to start..."
sleep 20

secretcli q block 1

cp /tmp/.secretd/keyring-test /root/.secretd/ -r

# MASTER_KEY="$(secretcli q register secret-network-params 2> /dev/null | cut -c 3- )"

#echo "Master key: $MASTER_KEY"

secretd init-enclave --reset

PUBLIC_KEY=$(secretd parse /opt/secret/.sgx_secrets/attestation_cert.der | cut -c 3- )

echo "Public key: $PUBLIC_KEY"

secretd parse /opt/secret/.sgx_secrets/attestation_cert.der
cat /opt/secret/.sgx_secrets/attestation_cert.der
tx_hash="$(secretcli tx register auth /opt/secret/.sgx_secrets/attestation_cert.der -y --from a --gas-prices 0.25uscrt | jq -r '.txhash')"

#secretcli q tx "$tx_hash"
sleep 15
secretcli q tx "$tx_hash"

SEED="$(secretcli q register seed "$PUBLIC_KEY" | cut -c 3-)"
echo "SEED: $SEED"
#exit

secretcli q register secret-network-params

secretd configure-secret node-master-cert.der "$SEED"

cp /tmp/.secretd/config/genesis.json /root/.secretd/config/genesis.json

secretd validate-genesis

RUST_BACKTRACE=1 secretd start --rpc.laddr tcp://0.0.0.0:26657

# ./wasmi-sgx-test.sh
