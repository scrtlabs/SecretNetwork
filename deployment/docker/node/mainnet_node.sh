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
  # secretd config keyring-backend test
  secretd config node tcp://"$RPC_URL"
  secretd config chain-id "$CHAINID"

  secretd init "$MONIKER" --chain-id "$CHAINID"

  echo "Initializing chain: $CHAINID with node moniker: $(hostname)"

  # Open RPC port to all interfaces
  perl -i -pe 's/laddr = .+?26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' ~/.secretd/config/config.toml

  secretd auto-register --reset --registration-service $REGISTRATION_SERVICE

  TRUST_HEIGHT=`curl -s http://20.104.20.173:26657/commit | jq .result.signed_header.header.height | tr -d '"'`
  TRUST_HASH=`curl -s http://20.104.20.173:26657/commit | jq .result.signed_header.commit.block_id.hash`

  # enable state sync (this is the only line in the config that uses enable = false. This could change and break everything
  perl -i -pe 's/enable = false/enable = true/' ~/.secretd/config/config.toml

  # replace state sync rpc server with our custom one
  perl -i -pe 's/rpc_servers =/rpc_servers = "'$STATE_SYNC','$STATE_SYNC'"/' ~/.secretd/config/config.toml
  # replace trust height with fetched one
  perl -i -pe 's/trust_height =/trust_height = '$TRUST_HEIGHT'/' ~/.secretd/config/config.toml
  # replace trust hash with fetched one
  perl -i -pe 's/trust_hash =/trust_hash = '$TRUST_HASH'/' ~/.secretd/config/config.toml


  mv /root/genesis.json /root/.secretd/config/genesis.json

  secretd validate-genesis

  perl -i -pe 's/ / /' ~/.secretd/config/config.toml

fi
secretd start
