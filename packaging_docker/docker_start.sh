#!/bin/ash

enigmad init $MONIKER --chain-id $CHAINID
echo "Initializing chain: $CHAINID with node moniker: $MONIKER"

wget -O /root/.enigmad/config/genesis.json $GENESISPATH > /dev/null
echo "Downloaded genesis file from: $GENESISPATH.."

enigmad validate-genesis

sed -i 's/persistent_peers = ""/persistent_peers = "'"$PERSISTENT_PEERS"'"/g' ~/.enigmad/config/config.toml
echo "Set persistent_peers: $PERSISTENT_PEERS"
enigmad start