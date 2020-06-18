# How to join the Secret Blockhain as a full node on mainnet

This document details how to join the EnigmaChain `mainnet` as a validator.

## Requirements

- Ubuntu/Debian host (with ZFS or LVM to be able to add more storage easily)
- A public IP address
- Open ports `TCP 26656 & 26657` _Note: If you're behind a router or firewall then you'll need to port forward on the network device._
- Reading https://docs.tendermint.com/master/tendermint-core/running-in-production.html

### While actual specs vary depending on the load and validators count, minimal requirements are:

1GB RAM
25GB of disk space
1.4 GHz CPU
SSD disks are preferable for applications with high transaction throughput.

### Recommended:

2GB RAM
100GB SSD
x64 2.0 GHz 2v CPU

## Installation

### 1. Download the Secret Blockchain package installer for Debian/Ubuntu:

```bash
wget https://github.com/enigmampc/SecretNetwork/releases/download/v0.0.3/enigma-blockchain_0.0.3_amd64.deb
```

([How to verify releases](/docs/verify-releases.md))

### 2. Install the package:

```bash
sudo dpkg -i enigma-blockchain_0.0.3_amd64.deb
```

### 3. Initialize your installation of the Secret Blockchain. Choose a **moniker** for yourself that will be public, and replace `<MONIKER>` with your moniker below

```bash
secretd init <MONIKER> --chain-id enigma-1
```

### 4. Download a copy of the Genesis Block file: `genesis.json`

```bash
wget -O ~/.secretd/config/genesis.json "https://raw.githubusercontent.com/enigmampc/SecretNetwork/master/enigma-1-genesis.json"
```

### 5. Validate the checksum for the `genesis.json` file you have just downloaded in the previous step:

```
echo "86cd9864f5b8e7f540c5edd3954372df94bd23de62e06d5c33a84bd5f3d29114 $HOME/.secretd/config/genesis.json" | sha256sum --check
```

### 6. Validate that the `genesis.json` is a valid genesis file:

```
secretd validate-genesis
```

### 7. Add persistent peers to your configuration file.

For an updated (partial) list of full nodes: http://bootstrap.mainnet.enigma.co/peers.txt
(Generated every minute with [this script](https://gist.github.com/assafmo/a39fdb535f74ce2d6493a1a3f695e4ca))

You can also use Enigma's node:

```
perl -i -pe 's/persistent_peers = ""/persistent_peers = "201cff36d13c6352acfc4a373b60e83211cd3102\@bootstrap.mainnet.enigma.co:26656"/' ~/.secretd/config/config.toml
```

This configuration updates automatically by your node when it learns of new nodes in the network.

### 8. Listen for incoming RPC requests so that light nodes can connect to you:

```bash
perl -i -pe 's/laddr = .+?26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' ~/.secretd/config/config.toml
```

### 9. Enable `enigma-node` as a system service:

```
sudo systemctl enable enigma-node
```

### 10. Start `enigma-node` as a system service:

```
sudo systemctl start enigma-node
```

### 11. If everything above worked correctly, the following command will show your node streaming blocks (this is for debugging purposes only, kill this command anytime with Ctrl-C):

```bash
journalctl -f -u enigma-node
```

```
-- Logs begin at Mon 2020-02-10 16:41:59 UTC. --
Feb 10 21:18:34 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:34.307] Executed block                               module=state height=2629 validTxs=0 invalidTxs=0
Feb 10 21:18:34 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:34.317] Committed state                              module=state height=2629 txs=0 appHash=34BC6CF2A11504A43607D8EBB2785ED5B20EAB4221B256CA1D32837EBC4B53C5
Feb 10 21:18:39 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:39.382] Executed block                               module=state height=2630 validTxs=0 invalidTxs=0
Feb 10 21:18:39 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:39.392] Committed state                              module=state height=2630 txs=0 appHash=17114C79DFAAB82BB2A2B67B63850864A81A048DBADC94291EB626F584A798EA
Feb 10 21:18:44 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:44.458] Executed block                               module=state height=2631 validTxs=0 invalidTxs=0
Feb 10 21:18:44 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:44.468] Committed state                              module=state height=2631 txs=0 appHash=D2472874A63CE166615E5E2FDFB4006ADBAD5B49C57C6B0309F7933CACC24B10
Feb 10 21:18:49 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:49.532] Executed block                               module=state height=2632 validTxs=0 invalidTxs=0
Feb 10 21:18:49 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:49.543] Committed state                              module=state height=2632 txs=0 appHash=A14A58E80FB24115DD41E6D787667F2FBBE003895D1B79334A240F52FCBD97F2
Feb 10 21:18:54 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:54.613] Executed block                               module=state height=2633 validTxs=0 invalidTxs=0
Feb 10 21:18:54 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:54.623] Committed state                              module=state height=2633 txs=0 appHash=C00112BB0D9E6812CEB4EFF07D2205D86FCF1FD68DFAB37829A64F68B5E3B192
Feb 10 21:18:59 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:59.685] Executed block                               module=state height=2634 validTxs=0 invalidTxs=0
Feb 10 21:18:59 ip-172-31-41-58 secretd[8814]: I[2020-02-10|21:18:59.695] Committed state                              module=state height=2634 txs=0 appHash=1F371F3B26B37A2173563CC928833162DDB753D00EC2BCE5EDC088F921AD0D80
^C
```

You are now a full node. :tada:

### 12. Add the following configuration settings (some of these avoid having to type some flags all the time):

```bash
secretcli config chain-id enigma-1
```

```bash
secretcli config output json
```

```bash
secretcli config indent true
```

```bash
secretcli config trust-node true # true if you trust the full-node you are connecting to, false otherwise
```

### 13. Get your node ID with:

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
secretcli config chain-id enigma-1
```

```bash
secretcli config output json
```

```bash
secretcli config indent true
```

```bash
secretcli config trust-node false
```

```bash
secretcli config node tcp://<your-public-ip>:26657
```
