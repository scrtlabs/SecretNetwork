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

  mkdir -p /root/.secretd/.node
  secretd config node tcp://"$RPC_URL"
  secretd config chain-id "$CHAINID"
  secretd config keyring-backend test
  # export SECRET_NETWORK_CHAIN_ID=$CHAINID
  # export SECRET_NETWORK_KEYRING_BACKEND=test
  # secretd init "$(hostname)" --chain-id enigma-testnet || true

  secretd init "$MONIKER" --chain-id "$CHAINID"

  b_mnemonic="jelly shadow frog dirt dragon use armed praise universe win jungle close inmate rain oil canvas beauty pioneer chef soccer icon dizzy thunder meadow"

  secretd keys add a
  echo $b_mnemonic | secretd keys add b --recover

  secretd keys add a
  echo $b_mnemonic | secretd keys add b --recover

  echo "Initializing chain: $CHAINID with node moniker: $MONIKER"

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

  secretd configure-secret node-master-key.txt "$SEED"

  curl http://"$RPC_URL"/genesis | jq -r .result.genesis > /root/.secretd/config/genesis.json

  echo "Downloaded genesis file from $RPC_URL"

  secretd validate-genesis

  # this is here to make sure that the node doesn't resync
  cp /opt/secret/.sgx_secrets/attestation_cert.der /root/.secretd/config/

  if [ "$VALIDATOR" == "true" ]
  then
    echo "Setting this node up as a validator"
    balance=$(secretd q bank balances $(secretd keys show -a a) --output json | jq ".balances[0].amount" -r)
    fee=5000
    staking_amount="$((balance-fee))"uscrt

    echo "Staking amount: $staking_amount"

    secretd tx staking create-validator \
      --amount=$staking_amount \
      --pubkey=$(secretd tendermint show-validator) \
      --from=a \
      --moniker=$(hostname)
      --commission-rate="0.10" \
      --commission-max-rate="0.20" \
      --commission-max-change-rate="0.01" \
      --min-self-delegation="1"
  fi
fi

secretd config node tcp://localhost:26657
secretd start
