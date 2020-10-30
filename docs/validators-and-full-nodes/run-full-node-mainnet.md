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
- 100GB HDD
- 1 dedicated core of any Intel Skylake processor (Intel® 6th generation) or better

#### Recommended requirements

- Up to date SGX ([Read this](https://learn.scrt.network/sgx.html), [Setup](setup-sgx.md), [Verify](verify-sgx.md))
- 2GB RAM
- 256GB SSD
- 2 dedicated cores of any Intel Skylake processor (Intel® 6th generation) or better

### Pre-Installation Steps

Use these steps to create your key and send it some SCRT. The key will be used during the installation
when registering your node on the network.

#### 1. Generate a new key pair for yourself (change `<key-alias>` with any word of your choice, this is just for your internal/personal reference):

```bash
secretcli keys add <key-alias>
```

**:warning:Note:warning:: Backup the mnemonics!**
**:warning:Note:warning:: Please make sure you also [backup your validator](backup-a-validator.md)**

**Note**: If you already have a key you can import it with the bip39 mnemonic with `secretcli keys add <key-alias> --recover` or with `secretcli keys export` (exports to `stderr`!!) & `secretcli keys import`.

Then transfer funds to address you just created.

### 2. Check that you have the funds:

```bash
secretcli q account $(secretcli keys show -a <key-alias>)
```

If you get the following message, it means that you have no tokens yet:

```bash
ERROR: unknown address: account secret1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx does not exist
```


### Installation

*NOTE*: Substitute **$YOUR_MONIKER** and **$YOUR_KEY_NAME** (below) with your node's moniker and the `<key-alias>` 
you created above.

```bash
cd ~

wget https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.0/secretnetwork_1.0.0_amd64.deb

sudo apt install ./secretnetwork_1.0.0_amd64.deb

YOUR_MONIKER="<moniker>"

secretd init "$YOUR_MONIKER" --chain-id secret-2

wget -O ~/.secretd/config/genesis.json "https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.0/genesis.json"

echo "4ca53e34afed034d16464d025291fe16a847c9aca0a259f9237413171b19b4cf .secretd/config/genesis.json" | sha256sum --check

secretd validate-genesis

secretd init-enclave

PUBLIC_KEY=$(secretd parse attestation_cert.der 2> /dev/null | cut -c 3-)
echo $PUBLIC_KEY

secretcli config chain-id secret-2
secretcli config node tcp://secret-2.node.enigma.co:26657
secretcli config trust-node true
secretcli config output json
secretcli config indent true

secretcli tx register auth ./attestation_cert.der --from "$YOUR_KEY_NAME" --gas 250000 --gas-prices 0.25uscrt

SEED=$(secretcli query register seed "$PUBLIC_KEY" | cut -c 3-)
echo $SEED

secretcli query register secret-network-params

mkdir -p ~/.secretd/.node

secretd configure-secret node-master-cert.der "$SEED"

perl -i -pe 's/^seeds = ".*?"/seeds = "bee0edb320d50c839349224b9be1575ca4e67948\@secret-2.node.enigma.co:26656"/' ~/.secretd/config/config.toml
perl -i -pe 's;laddr = "tcp://127.0.0.1:26657";laddr = "tcp://0.0.0.0:26657";' ~/.secretd/config/config.toml

sudo systemctl enable secret-node

sudo systemctl start secret-node # (Now your new node is live and catching up)

secretcli config node tcp://localhost:26657
```

You are now a full node. :tada:

#### See your node's logs:

```bash
journalctl -u secret-node -f
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
