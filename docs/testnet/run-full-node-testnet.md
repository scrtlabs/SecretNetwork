# How To Join Secret Network as a Full Node on Testnet

This document details how to join the Secret Network `testnet` as a full node. Once your full node is running, you can turn it into a validator in the optional last step.

## Requirements

- Ubuntu/Debian host (with ZFS or LVM to be able to add more storage easily)
- A public IP address
- Open ports `TCP 26656 & 26657` _Note: If you're behind a router or firewall then you'll need to port forward on the network device._
- Reading https://docs.tendermint.com/master/tendermint-core/running-in-production.html
- RPC address of an already active node. You can use `bootstrap.secrettestnet.io:26657`, or any other node that exposes RPC services.

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

See instructions for [setup](setup-sgx-testnet.md) and [verification](verify-sgx.md).

### 1. Download the Secret Network package installer for Debian/Ubuntu:

```bash
wget https://github.com/chainofsecrets/SecretNetwork/releases/download/v1.0.0/secretnetwork_1.0.0_amd64.deb
```

([How to verify releases](../verify-releases.md))

### 2. Install the package:

```bash
sudo dpkg -i secretnetwork_1.0.0_amd64.deb
```

### 3. Initialize your installation of the Secret Network.

Choose a **moniker** for yourself, and replace `<MONIKER>` with your moniker below.
This moniker will serve as your public nickname in the network.

```bash
secretd init <MONIKER> --chain-id holodeck-2
```

### 4. Download a copy of the Genesis Block file: `genesis.json`

```bash
wget -O ~/.secretd/config/genesis.json "https://github.com/chainofsecrets/SecretNetwork/releases/download/v1.0.0/holodeck-2-genesis.json"
```

### 5. Validate the checksum for the `genesis.json` file you have just downloaded in the previous step:

```bash
echo "e45d6aa9825bae70c277509c8346122e265d64cb4211c23def4ae8f6bf3da2f1 $HOME/.secretd/config/genesis.json" | sha256sum --check
```

### 6. Validate that the `genesis.json` is a valid genesis file:

```bash
secretd validate-genesis
```

### 7. The rest of the commands should be ran from the home folder (`/home/<your_username>`)

```bash
cd ~
```

### 8. Initialize secret enclave

Make sure the directory `~/.sgx_secrets` exists:

```bash
mkdir -p ~/.sgx_secrets
```

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
ls -lh ./attestation_cert.der
```

### 10. Check your certificate is valid

Should print your 64 character registration key if it was successful.

```bash
PUBLIC_KEY=$(secretd parse attestation_cert.der 2> /dev/null | cut -c 3-)
echo $PUBLIC_KEY
```

### 11. Config `secretcli`, generate a key and get some test-SCRT from the faucet

The steps using `secretcli` can be run on any machine, they don't need to be on the full node itself. We'll refer to the machine where you are using `secretcli` as the "CLI machine" below.

To run the steps with `secretcli` on another machine, [set up the CLI](install_cli.md) there.

Configure `secretcli`. Initially you'll be using the bootstrap node, as you'll need to connect to a running node and your own node is not running yet.

```bash
secretcli config chain-id holodeck-2
secretcli config node tcp://bootstrap.secrettestnet.io:26657
secretcli config output json
secretcli config indent true
secretcli config trust-node true
```

Set up a key. Make sure you backup the mnemonic and the keyring password.

```bash
secretcli keys add $INSERT_YOUR_KEY_NAME
```

This will output your address, a 45 character-string starting with `secret1...`. Copy/paste it to get some test-SCRT from [the faucet](https://faucet.secrettestnet.io/). Continue when you have confirmed your account has some test-SCRT in it.

### 12. Register your node on-chain

Run this step on the CLI machine. If you're using different CLI machine than the full node, copy `attestation_cert.der` from the full node to the CLI machine.

```bash
secretcli tx register auth <path/to/attestation_cert.der> --from $INSERT_YOUR_KEY_NAME --gas 250000
```

### 13. Pull & check your node's encrypted seed from the network

Run this step on the CLI machine.

```bash
SEED=$(secretcli query register seed "$PUBLIC_KEY" | cut -c 3-)
echo $SEED
```

### 14. Get additional network parameters

Run this step on the CLI machine.

These are necessary to configure the node before it starts.

```bash
secretcli query register secret-network-params
ls -lh ./io-master-cert.der ./node-master-cert.der
```

If you're using different CLI machine than the validator node, copy `node-master-cert.der` from the CLI machine to the validator node.

### 15. Configure your secret node

From here on, run commands on the full node again.

```bash
mkdir -p ~/.secretd/.node
secretd configure-secret node-master-cert.der "$SEED"
```

### 16. Add persistent peers to your configuration file.

You can also use Chain of Secrets' node:

```bash
perl -i -pe 's/persistent_peers = ""/persistent_peers = "64b03220d97e5dc21ec65bf7ee1d839afb6f7193\@bootstrap.secrettestnet.io:26656"/' ~/.secretd/config/config.toml
```

### 17. Listen for incoming RPC requests so that light nodes can connect to you:

```bash
perl -i -pe 's/laddr = .+?26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' ~/.secretd/config/config.toml
```

### 18. Enable `secret-node` as a system service:

```bash
sudo systemctl enable secret-node
```

### 19. Start `secret-node` as a system service:

```bash
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

Be sure to point your CLI to your running node instead of the bootstrap node

```
secretcli config node tcp://localhost:26657
```

If someone wants to add you as a peer, have them add the above address to their `persistent_peers` in their `~/.secretd/config/config.toml`.

And if someone wants to use your node from their `secretcli` then have them run:

```bash
secretcli config chain-id holodeck-2
secretcli config output json
secretcli config indent true
secretcli config node tcp://<your-public-ip>:26657
```

### 22. Optional: make your full node a validator

Your full node is now part of the network, storing and verifying chain data and Secret Contracts, and helping to distribute transactions and blocks. It's usable as a sentry node, for people to connect their CLI or light clients, or just to support the network.

It is however not producing blocks yet, and you can't delegate funds to it for staking. To do that that you'll have to turn it into a validator by submitting a `create-validator` transaction.

On the full node, get the pubkey of the node:

```bash
secretd tendermint show-validator
```

The pubkey is an 83-character string starting with `secretvalconspub...`.

On the CLI machine, run the following command. The account you use becomes the operator account for your validator, which you'll use to collect rewards, participate in on-chain governance, etc, so make sure you keep good backups of the key. `<moniker>` is the name for your validator which is shown e.g. in block explorers.

```bash
secretcli tx staking create-validator \
  --amount=<amount-to-delegate-to-yourself>uscrt \
  --pubkey=<pubkey of the full node> \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --min-self-delegation="1" \
  --moniker="<moniker>" \
  --from=$INSERT_YOUR_KEY_NAME
```

The `create-validator` command allows using some more parameters. For more info on these and the additional parameters, run `secretcli tx staking create-validator --help`.

After you submitted the transaction, check you've been added as a validator:

```bash
secretcli q staking validators | grep moniker
```

Congratulations! You are now running a validator on the Secret Network testnet.
