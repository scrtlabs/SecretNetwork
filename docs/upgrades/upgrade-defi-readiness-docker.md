### Local Docker image

To upgrade the image, simply replace enigmampc/secret-network-node:v1.0.0-mainnet with enigmampc/secret-network-node:v1.0.4-mainnet

No other configuration needed.

### Azure Node

The Azure image will be updated automatically, so new nodes will have the updated version already available. If you wish
to update your current node -

`stop-secret-node`

`sudo nano /usr/local/bin/secret-node/docker-compose.yaml` (or some other text editor)

replace enigmampc/secret-network-node:v1.0.0-mainnet -> enigmampc/secret-network-node:v1.0.4-mainnet

`start-secret-node`