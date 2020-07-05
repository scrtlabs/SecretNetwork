#!/bin/ash

secretd init $MONIKER --chain-id $CHAINID
echo "Initializing chain: $CHAINID with node moniker: $MONIKER"

wget -O /root/.secretd/config/genesis.json $GENESISPATH > /dev/null
echo "Downloaded genesis file from: $GENESISPATH.."

secretd validate-genesis

sed -i 's/persistent_peers = ""/persistent_peers = "'$PERSISTENT_PEERS'"/g' ~/.secretd/config/config.toml
echo "Set persistent_peers: $PERSISTENT_PEERS"
secretd start
