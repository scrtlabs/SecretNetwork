# Run a Full Node

This document details how to join the Secret Network `mainnet` as a validator.

### Requirements

- Up to date SGX ([Read this](https://learn.scrt.network/sgx.html), [Setup](setup-sgx.md), [Verify](verify-sgx.md))
- Ubuntu/Debian host (with ZFS or LVM to be able to add more storage easily)
- A public IP address
- Open ports `TCP 26656 & 26657` _Note: If you're behind a router or firewall then you'll need to port forward on the network device._
- Reading https://docs.tendermint.com/master/tendermint-core/running-in-production.html

#### Minimum requirements

- Up to date SGX ([Read this](https://learn.scrt.network/sgx.html), [Setup](setup-sgx.md), [Verify](verify-sgx.md))
- 1GB RAM
- 100GB HDD (Premium SSD)
- 1 dedicated core of any Intel Skylake processor (Intel® 6th generation) or better

#### Recommended requirements

- Up to date SGX ([Read this](https://learn.scrt.network/sgx.html), [Setup](setup-sgx.md), [Verify](verify-sgx.md))
- 2GB RAM
- 256GB SSD (Premium SSD)
- 2 dedicated cores of any Intel Skylake processor (Intel® 6th generation) or better

### Installation

#### Install the `secretnetwork`, initialize your node and validate the genesis file:

_NOTE_: Substitute **$YOUR_MONIKER** (below) with your node's nickname or alias.

```bash
cd ~

wget https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.4/secretnetwork_1.0.4_amd64.deb

echo "97c1aa2421a203184e541928cc9c409c50afcfac5cbd55993e6a9593399587f9 secretnetwork_1.0.4_amd64.deb" | sha256sum --check

sudo apt install ./secretnetwork_1.0.4_amd64.deb

secretd init "$YOUR_MONIKER" --chain-id secret-2

wget -O ~/.secretd/config/genesis.json "https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.4/genesis.json"

echo "4ca53e34afed034d16464d025291fe16a847c9aca0a259f9237413171b19b4cf .secretd/config/genesis.json" | sha256sum --check

secretd validate-genesis
```

#### Create the enclave attestation certificate and store its public key:

```bash
secretd init-enclave

PUBLIC_KEY=$(secretd parse attestation_cert.der 2> /dev/null | cut -c 3-)
echo $PUBLIC_KEY
```

#### Configure `secretcli`:

```bash
secretcli config chain-id secret-2
secretcli config node http://rpc.enigma.co:26657
secretcli config output json
secretcli config indent true
```

#### Create your `key-alias`:

If you haven't **already created a key**, use these steps to create a secret address and send some SCRT to it. The key will be used to register your node with the Secret Network.

##### Generate a new key pair for yourself (change `<key-alias>` with any word of your choice, this is just for your internal/personal reference):

```bash
secretcli keys add <key-alias>
```

**:warning:Note:warning:: Backup the mnemonics!**
**:warning:Note:warning:: Please make sure you also [backup your validator](backup-a-validator.md)**

**Note**: If you already have a key you can import it with the bip39 mnemonic with `secretcli keys add <key-alias> --recover` or with `secretcli keys export` (exports to `stderr`!!) & `secretcli keys import`.

Then transfer funds to the address you just created.

##### Check that you have the funds:

```bash
secretcli q account $(secretcli keys show -a <key-alias>)
```

If you get the following message, it means that you have no tokens yet:

```bash
ERROR: unknown address: account secret1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx does not exist
```

#### Register and configure your node:

_NOTE_: Substitute **$YOUR_KEY_NAME** (below) with the `key-alias` you created earlier.

```bash
secretcli tx register auth ./attestation_cert.der --from "$YOUR_KEY_NAME" --gas 250000 --gas-prices 0.25uscrt

SEED=$(secretcli query register seed "$PUBLIC_KEY" | cut -c 3-)
echo $SEED

secretcli query register secret-network-params

mkdir -p ~/.secretd/.node

secretd configure-secret node-master-cert.der "$SEED"

perl -i -pe 's/^seeds = ".*?"/seeds = "332c69cb1e5a4a62916be571ff9c4cd09cc71a79\@secret-2.node.enigma.co:26656"/' ~/.secretd/config/config.toml
perl -i -pe 's;laddr = "tcp://127.0.0.1:26657";laddr = "tcp://0.0.0.0:26657";' ~/.secretd/config/config.toml
```

#### Start your node as a service:

```bash
sudo systemctl enable secret-node

sudo systemctl start secret-node # (Now your new node is live and catching up)
```

You are now a full node. :tada:

#### See your node's logs:

```bash
journalctl -u secret-node -f
```

You can stop viewing the logs by pressing `ctrl + C` which sends a signal to `journalctl` to exit.

#### Point `secretcli` to your node and query its status:

```bash
secretcli config node tcp://localhost:26657

secretcli status
```

When the value of `catching_up` is _false_, your node is fully sync'd with the network.

```bash
  "sync_info": {
    "latest_block_hash": "7BF95EED4EB50073F28CF833119FDB8C7DFE0562F611DF194CF4123A9C1F4640",
    "latest_app_hash": "7C0C89EC4E903BAC730D9B3BB369D870371C6B7EAD0CCB5080B5F9D3782E3559",
    "latest_block_height": "668538",
    "latest_block_time": "2020-10-31T17:50:56.800119764Z",
    "earliest_block_hash": "E7CAD87A4FDC47DFDE3D4E7C24D80D4C95517E8A6526E2D4BB4D6BC095404113",
    "earliest_app_hash": "",
    "earliest_block_height": "1",
    "earliest_block_time": "2020-09-15T14:02:31Z",
    "catching_up": false
  },
```

#### Get your node ID with:

```bash
secretd tendermint show-node-id
```

And publish yourself as a node with this ID:

```
<your-node-id>@<your-public-ip>:26656
```

So if someone wants to add you as a peer, have them add the above address to their `persistent_peers` in their `~/.secretd/config/config.toml`.
And if someone wants to use you from their `secretcli` then have them run:

```bash
secretcli config chain-id secret-2
secretcli config output json
secretcli config indent true
secretcli config trust-node true
secretcli config node tcp://<your-public-ip>:26657
```
