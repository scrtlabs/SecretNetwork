#!/usr/bin/env bash

set -uvo pipefail

# init the node
# rm -rf ~/.secret*
#secretcli config chain-id enigma-testnet
#secretcli config output json
#secretcli config indent true
#secretcli config trust-node true
#secretcli config keyring-backend test
rm -rf ~/.secretd

NO_TESTS="${NO_TESTS:''}"

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

if [ -z "$NO_TESTS" ]
then
    RUST_BACKTRACE=1 secretd start
else
    RUST_BACKTRACE=1 secretd start &
fi


########## RUN INTEGRATION TESTS

function wait_for_tx () {
    until (secretd q tx "$1" &> /dev/null)
    do
        echo "$2"
        sleep 1
    done
}

until (secretd status 2>&1 | jq -e '(.SyncInfo.latest_block_height | tonumber) > 0' &>/dev/null); do
    echo "Waiting for chain to start..."
    sleep 1
done

sleep 5

# store wasm code on-chain so we could later instantiate it
export STORE_TX_HASH=$(
    yes |
    secretd tx compute store erc20.wasm --from a --gas 1200000 --gas-prices 0.25uscrt --output json |
    jq -r .txhash
)

wait_for_tx "$STORE_TX_HASH" "Waiting for store to finish on-chain..."

# test storing of wasm code (this doesn't touch sgx yet)
secretd q tx "$STORE_TX_HASH" --output json |
    jq -e '.logs[].events[].attributes[] | select(.key == "code_id" and .value == "1")'

# init the contract (ocall_init + write_db + canonicalize_address)
# a is a tendermint address (will be used in transfer: https://github.com/CosmWasm/cosmwasm-examples/blob/f2f0568ebc90d812bcfaa0ef5eb1da149a951552/erc20/src/contract.rs#L110)
# secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t is just a random address
# balances are set to 108 & 53 at init
INIT_TX_HASH=$(
    yes |
        secretd tx compute instantiate 1 "{\"decimals\":10,\"initial_balances\":[{\"address\":\"$(secretd keys show a -a)\",\"amount\":\"108\"},{\"address\":\"secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t\",\"amount\":\"53\"}],\"name\":\"ReuvenPersonalRustCoin\",\"symbol\":\"RPRC\"}" --label RPRCCoin --output json --gas-prices 0.25uscrt --from a |
        jq -r .txhash
)

wait_for_tx "$INIT_TX_HASH" "Waiting for instantiate to finish on-chain..."

export CONTRACT_ADDRESS=$(
    secretd q tx "$INIT_TX_HASH" --output json |
        jq -er '.logs[].events[].attributes[] | select(.key == "contract_address") | .value' |
        head -1
)

# test balances after init (ocall_query + read_db + canonicalize_address)
secretd q compute query "$CONTRACT_ADDRESS" "{\"balance\":{\"address\":\"$(secretd keys show a -a)\"}}" --output json |
    jq -e '.balance == "108"'
secretd q compute query "$CONTRACT_ADDRESS" "{\"balance\":{\"address\":\"secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t\"}}" --output json |
    jq -e '.balance == "53"'

# transfer 10 balance (ocall_handle + read_db + write_db + humanize_address + canonicalize_address)
TRANSFER_TX_HASH=$(
    yes |
        secretd tx compute execute --from a "$CONTRACT_ADDRESS" '{"transfer":{"amount":"10","recipient":"secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t"}}' --gas-prices 0.25uscrt --output json 2> /dev/null |
        jq -r .txhash
)

wait_for_tx "$TRANSFER_TX_HASH" "Waiting for transfer to finish on-chain..."

# test balances after transfer (ocall_query + read_db)
secretd q compute query "$CONTRACT_ADDRESS" "{\"balance\":{\"address\":\"$(secretd keys show a -a)\"}}" --output json |
    jq -e '.balance == "98"'
secretd q compute query "$CONTRACT_ADDRESS" "{\"balance\":{\"address\":\"secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t\"}}" --output json |
    jq -e '.balance == "63"'

(secretd q compute query "$CONTRACT_ADDRESS" "{\"balance\":{\"address\":\"secret1zzzzzzzzzzzzzzzzzz\"}}" --output json || true) 2>&1 | grep -c 'canonicalize_address errored: invalid checksum'

echo "All is done. Yay!"

