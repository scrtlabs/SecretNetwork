# How To Join Secret Network as a Full Node on Testnet

This document details how to join the Secret Network `testnet` as a validator.

## Requirements

- Ubuntu/Debian host (with ZFS or LVM to be able to add more storage easily)
- A public IP address
- Open ports `TCP 26656 & 26657` _Note: If you're behind a router or firewall then you'll need to port forward on the network device._
- Reading https://docs.tendermint.com/master/tendermint-core/running-in-production.html
- RPC address of an already active node. You can use `bootstrap.pub.testnet.enigma.co:26657`, or any other node that exposes RPC services.
- Account with at least 1 SCRT

### Minimum requirements

- 1GB RAM
- 100GB HDD
- 1 dedicated core of any Intel Skylake processor (Intel® 6th generation) or better

### Recommended requirements

- 2GB RAM
- 256GB SSD
- 2 dedicated cores of any Intel Skylake processor (Intel® 6th generation) or better
- Motherboard with support for SGX in the BIOS

Refer to https://ark.intel.com/content/www/us/en/ark.html#@Processors if unsure if your processor supports SGX

## Installation

### 0. Step up SGX on your local machine

See instructions [here](/docs/validators-and-full-nodes/setup-sgx.md)

### 1. Download the Secret Network package installer for Debian/Ubuntu:

```bash
wget https://github.com/enigmampc/SecretNetwork/releases/download/v0.5.0-rc1/secretnetwork_0.5.0-rc1_amd64.deb
```

([How to verify releases](/testnet/verify-sgx.md))

### 2. Install the package:

```bash
sudo dpkg -i secretnetwork_0.5.0-rc1_amd64.deb
```

### 3. Initialize your installation of the Secret Network. Choose a **moniker** for yourself that will be public, and replace `<MONIKER>` with your moniker below

```bash
secretd init <MONIKER> --chain-id enigma-pub-testnet-1
```

### 4. Download a copy of the Genesis Block file: `genesis.json`

```bash
wget -O ~/.secretd/config/genesis.json "https://github.com/enigmampc/SecretNetwork/releases/download/v0.5.0-rc1/genesis.json"
```

### 5. Validate the checksum for the `genesis.json` file you have just downloaded in the previous step:

```bash
echo "d12a38c37d7096b0c0d59a56af12de2e4e5eca598d53699119344b26a6794026 $HOME/.secretd/config/genesis.json" | sha256sum --check
```

### 6. Validate that the `genesis.json` is a valid genesis file:

```bash
secretd validate-genesis
```

### 7. Create the `.sgx_secrets` folder

```bash
mkdir .sgx_secrets
```

### 8. Initialize secret enclave

Make sure SGX is enabled and running or this step might fail.

```bash
export SCRT_ENCLAVE_DIR=/usr/lib
```

```bash
secretd init-enclave
```

### 9. Check that initialization was successful

Attestation certificate should have been created by the previous step

```bash
ls attestation_cert.der
```

### 10. Check your certificate is valid

Should print your 64 character registration key if it was successful.

```bash
PUBLIC_KEY=$(secretd parse attestation_cert.der 2> /dev/null | cut -c 3-)
echo $PUBLIC_KEY
```

### 11. Config `secretcli`

```bash
secretcli config chain-id enigma-pub-testnet-1
secretcli config node tcp://bootstrap.pub.testnet.enigma.co:26657
secretcli config broadcast-mode block
secretcli config output json
secretcli config indent true
```

### 12. Register your node on-chain

This step can be run from any location (doesn't have to be from the same node)

```bash
secretcli tx register auth <path/to/attestation_cert.der> --from <your account>
```

### 13. Pull & check your node's encrypted seed from the network

```bash
SEED=$(secretcli query register seed "$PUBLIC_KEY" | cut -c 3-)
echo $SEED
```

### 14. Get additional network parameters

These are necessary to configure the node before it starts

```bash
secretcli query register secret-network-params
```

### 15. Configure your secret node

```bash
secretd configure-secret node-master-cert.der "$SEED"
```

### 16. Add persistent peers to your configuration file.

You can also use Enigma's node:

```
perl -i -pe 's/persistent_peers = ""/persistent_peers = "115aa0a629f5d70dd1d464bc7e42799e00f4edae\@bootstrap.pub.testnet.enigma.co:26656"/' ~/.secretd/config/config.toml
```

### 17. Listen for incoming RPC requests so that light nodes can connect to you:

```bash
perl -i -pe 's/laddr = .+?26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' ~/.secretd/config/config.toml
```

### 18. Enable `secret-node` as a system service:

```
sudo systemctl enable secret-node
```

### 19. Start `secret-node` as a system service:

```
sudo systemctl start secret-node
```

### 20. If everything above worked correctly, the following command will show your node streaming blocks (this is for debugging purposes only, kill this command anytime with Ctrl-C):

```bash
journalctl -f -u secret-node
```

```
-- Logs begin at Mon 2020-02-10 16:41:59 UTC. --
Feb 10 21:18:34 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:34.307] Executed block                               module=state height=2629 validTxs=0 invalidTxs=0
Feb 10 21:18:34 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:34.317] Committed state                              module=state height=2629 txs=0 appHash=34BC6CF2A11504A43607D8EBB2785ED5B20EAB4221B256CA1D32837EBC4B53C5
Feb 10 21:18:39 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:39.382] Executed block                               module=state height=2630 validTxs=0 invalidTxs=0
Feb 10 21:18:39 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:39.392] Committed state                              module=state height=2630 txs=0 appHash=17114C79DFAAB82BB2A2B67B63850864A81A048DBADC94291EB626F584A798EA
Feb 10 21:18:44 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:44.458] Executed block                               module=state height=2631 validTxs=0 invalidTxs=0
Feb 10 21:18:44 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:44.468] Committed state                              module=state height=2631 txs=0 appHash=D2472874A63CE166615E5E2FDFB4006ADBAD5B49C57C6B0309F7933CACC24B10
^C
```

You are now a full node. :tada:

### 21. Get your node ID with:

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
secretcli config chain-id enigma-pub-testnet-1
secretcli config output json
secretcli config indent true
secretcli config node tcp://<your-public-ip>:26657
```
