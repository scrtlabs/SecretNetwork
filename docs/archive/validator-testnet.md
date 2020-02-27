# How to join EnigmaChain as a testnet validator

This document details how to join the EnigmaChain `testnet`

## Requirements

- Ubuntu/Debian host
- A public IP address
- Open ports `TCP 26655 - 26660`

## Installation

### 1. Download the [EnigmaChain package installer](https://enigmaco-website.s3.amazonaws.com/enigmachain_0.0.1_amd64.deb) (Debian/Ubuntu)

```
wget https://enigmaco-website.s3.amazonaws.com/enigmachain_0.0.1_amd64.deb
```

### 2. Make sure you don't have a previous installation (from testnet):

**Note:** If you will be using the same key from testnet you can export it with `enigmacli keys export <key-alias> > my.key` and later import it with `enigmacli keys import <key-alias> my.key`.

```bash
sudo dpkg -r enigmachain
sudo rm -rf ~/.enigmad ~/.enigmacli
sudo rm -rf ~/.engd ~/.engcli
sudo rm -rf "$(which enigmad)"
sudo rm -rf "$(which enigmacli)"
sudo rm -rf "$(which engcli)"
sudo rm -rf "$(which engd)"
```

### 3. Install the above package:

```
sudo dpkg -i enigmachain_0.0.1_amd64.deb
```

### 4. Update the configuration file that sets up the system service with your current user as the user this service will run as.

_Note: Even if we are running this command and the previous one with sudo, this package does not need to be run as root_.

```
sudo perl -i -pe "s/XXXXX/$(logname)/" /etc/systemd/system/enigma-node.service
```

### 5. Initialize your installation. Choose a **moniker** for yourself that will be public, and replace `<MONIKER>` with your moniker below

```
engd init <MONIKER> --chain-id enigma-testnet
```

### 6. Download a copy of the Genesis Block file: `genesis.json`

```
wget -O ~/.engd/config/genesis.json https://gist.githubusercontent.com/lacabra/29ea80e279a70a8b3315baa0157cfe97/raw/faa8356fc2e4b08e29abb9eeb26237cd7eb9984f/genesis.json
```

### 7. Validate the checksum for the `genesis.json` file you have just downloaded in the previous step:

```
echo "6724d80b5eaa6b2d8b181ed8021d5c68e5fea96139ce51d3a073e7ef0f13e37f $HOME/.engd/config/genesis.json" | sha256sum --check
```

### 8. Validate that the `genesis.json` is a valid genesis file:

```
engd validate-genesis
```

### 9. Add `bootstrap.enigmachain.enigma.co` as a persistent peer in your configuration file.

If you are curious, you can query the RPC endpoint on that node http://bootstrap.enigmachain.enigma.co:26657/ (please note that the RPC port `26657` is different from the P2P port `26656` below)

```
perl -i -pe 's/persistent_peers = ""/persistent_peers = "6795f5e88edab2e225389eb9b6d6a2f715ddbcd2\@bootstrap.enigmachain.enigma.co:26656"/' ~/.engd/config/config.toml
```

### 10. Enable `enigma-node` as a system service:

```
sudo systemctl enable enigma-node
```

### 11. Start `enigma-node` as a system service:

```
sudo systemctl start enigma-node
```

### 12. If everything above worked correctly, the following command will show your node streaming blocks (this is for debugging purposes only, kill this command anytime with Ctrl-C):

```
journalctl -f -u enigma-node
-- Logs begin at Mon 2020-02-10 16:41:59 UTC. --
Feb 10 21:18:34 ip-172-31-41-58 engd[8814]: I[2020-02-10|21:18:34.307] Executed block                               module=state height=2629 validTxs=0 invalidTxs=0
Feb 10 21:18:34 ip-172-31-41-58 engd[8814]: I[2020-02-10|21:18:34.317] Committed state                              module=state height=2629 txs=0 appHash=34BC6CF2A11504A43607D8EBB2785ED5B20EAB4221B256CA1D32837EBC4B53C5
Feb 10 21:18:39 ip-172-31-41-58 engd[8814]: I[2020-02-10|21:18:39.382] Executed block                               module=state height=2630 validTxs=0 invalidTxs=0
Feb 10 21:18:39 ip-172-31-41-58 engd[8814]: I[2020-02-10|21:18:39.392] Committed state                              module=state height=2630 txs=0 appHash=17114C79DFAAB82BB2A2B67B63850864A81A048DBADC94291EB626F584A798EA
Feb 10 21:18:44 ip-172-31-41-58 engd[8814]: I[2020-02-10|21:18:44.458] Executed block                               module=state height=2631 validTxs=0 invalidTxs=0
Feb 10 21:18:44 ip-172-31-41-58 engd[8814]: I[2020-02-10|21:18:44.468] Committed state                              module=state height=2631 txs=0 appHash=D2472874A63CE166615E5E2FDFB4006ADBAD5B49C57C6B0309F7933CACC24B10
Feb 10 21:18:49 ip-172-31-41-58 engd[8814]: I[2020-02-10|21:18:49.532] Executed block                               module=state height=2632 validTxs=0 invalidTxs=0
Feb 10 21:18:49 ip-172-31-41-58 engd[8814]: I[2020-02-10|21:18:49.543] Committed state                              module=state height=2632 txs=0 appHash=A14A58E80FB24115DD41E6D787667F2FBBE003895D1B79334A240F52FCBD97F2
Feb 10 21:18:54 ip-172-31-41-58 engd[8814]: I[2020-02-10|21:18:54.613] Executed block                               module=state height=2633 validTxs=0 invalidTxs=0
Feb 10 21:18:54 ip-172-31-41-58 engd[8814]: I[2020-02-10|21:18:54.623] Committed state                              module=state height=2633 txs=0 appHash=C00112BB0D9E6812CEB4EFF07D2205D86FCF1FD68DFAB37829A64F68B5E3B192
Feb 10 21:18:59 ip-172-31-41-58 engd[8814]: I[2020-02-10|21:18:59.685] Executed block                               module=state height=2634 validTxs=0 invalidTxs=0
Feb 10 21:18:59 ip-172-31-41-58 engd[8814]: I[2020-02-10|21:18:59.695] Committed state                              module=state height=2634 txs=0 appHash=1F371F3B26B37A2173563CC928833162DDB753D00EC2BCE5EDC088F921AD0D80
^C
```

### 13. Add the following configuration settings (some of these avoid having to type some flags all the time):

```
engcli config chain-id enigma-testnet
engcli config output json
engcli config indent true
engcli config trust-node true # true if you trust the full-node you are connecting to, false otherwise
```

### 14. Generate a new key pair for yourself (change `<key-alias>` with any word of your choice, this is just for your internal/personal reference):

```
engcli keys add <key-alias>
```

**:warning:Note:warning:: Please backup the mnemonics!**

### 15. Output your node address:

```
engcli keys show <key-alias> -a
```

### 16. Request tokens to be sent to the address displayed above.

### 17. Check that you have the requested tokens:

```
engcli q account $(engcli keys show -a <key_alias>)
```

If you get the following message, it means that you have not tokens yet:

```
ERROR: unknown address: account enigmaxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx does not exist
```

### 18. Join the network as a new validator: replace `<MONIKER>` with your own from step 4 above, and adjust the amount you want to stake

(remember 1 SCRT = 1,000,000 uSCRT, and so the command below stakes 100k SCRT).

```
engcli tx staking create-validator \
  --amount=100000000000uscrt \
  --pubkey=$(engd tendermint show-validator) \
  --chain-id=enigma-testnet \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --min-self-delegation="1" \
  --gas=200000 \
  --gas-prices="0.025uscrt" \
  --moniker=<MONIKER> \
  --from=<key-alias>
```

### 19. Check that you have been added as a validator:

```bash
engcli q staking validators
```

If the above is too verbose, just run: `engcli q staking validators | grep moniker`. You should see your moniker listed.

## Staking more tokens

(remember 1 SCRT = 1,000,000 uSCRT)

In order to stake more tokens beyond those in the initial transaction, run:

```
engcli tx staking delegate $(engcli keys show <key-alias> --bech=val -a) 1uscrt --from <key-alias>
```

## Seeing your rewards from being a validator

```bash
engcli q distribution rewards $(engcli keys show -a <key-alias>)
```

## Seeing your commissions from your delegators

```bash
engcli q distribution commission $(engcli keys show -a <key-alias> --bech=val)
```

## Withdrawing rewards

```bash
engcli tx distribution withdraw-rewards $(engcli keys show --bech=val -a <key-alias>) --from <key-alias>
```

## Withdrawing rewards+commissions

```bash
engcli tx distribution withdraw-rewards $(engcli keys show --bech=val -a <key-alias>) --from <key-alias> --commission
```
