# How to join EnigmaChain as a mainnet validator

This document details how to join the EnigmaChain `mainnet` as a validator.

## Requirements

- Ubuntu/Debian host (with ZFS or LVM to be able to add more storage easily)
- A public IP address
- Open ports `TCP 26656 - 26660`

## Installation

1. Download the [EnigmaChain package installer](https://github.com/enigmampc/enigmachain/releases/download/v0.0.1/enigmachain_0.0.1_amd64.deb) (Debian/Ubuntu):

```bash
wget -O enigmachain_0.0.1_amd64.deb https://github.com/enigmampc/enigmachain/releases/download/v0.0.1/enigmachain_0.0.1_amd64.deb
echo "13b06329543dcbe6ca896406887afb79f7f8b975e5d5585db1943e4520b77521 enigmachain_0.0.1_amd64.deb" | sha256sum --check
```

2. Install the enigmachain package:

```bash
sudo dpkg -i enigmachain_0.0.1_amd64.deb
```

3. Update the configuration file that sets up the system service with your current user as the user this service will run as. _Note: Even if we are running this command and the previous one with sudo, this package does not need to be run as root_.

```bash
sudo perl -i -pe "s/XXXXX/$USER/" /etc/systemd/system/enigma-node.service
```

4. Initialize your installation of the enigmachain. Choose a **moniker** for yourself that will be public, and replace `<MONIKER>` with your moniker below

```bash
enigmad init <MONIKER> --chain-id enigma-1
```

5. Download a copy of the Genesis Block file: `genesis.json`

```bash
wget -O ~/.enigmad/config/genesis.json "https://raw.githubusercontent.com/enigmampc/enigmachain/master/enigma-1-genesis.json"
```

6. Validate the checksum for the `genesis.json` file you have just downloaded in the previous step:

```
echo "86cd9864f5b8e7f540c5edd3954372df94bd23de62e06d5c33a84bd5f3d29114 $HOME/.enigmad/config/genesis.json" | sha256sum --check
```

7. Validate that the `genesis.json` is a valid genesis file:

```
enigmad validate-genesis
```

8. Add `bootstrap.enigmachain.enigma.co` as a persistent peer in your configuration file. If you are curious, you can query the RPC endpoint on that node http://bootstrap.mainnet.enigma.co:26657/ (please note that the RPC port `26657` is different from the P2P port `26656` below)

```
perl -i -pe 's/persistent_peers = ""/persistent_peers = "201cff36d13c6352acfc4a373b60e83211cd3102\@bootstrap.mainnet.enigma.co:26656"/' ~/.enigmad/config/config.toml
```

9. Enable `enigma-node` as a system service:

```
sudo systemctl enable enigma-node
```

10. Start `enigma-node` as a system service:

```
sudo systemctl start enigma-node
```

11. If everything above worked correctly, the following command will show your node streaming blocks (this is for debugging purposes only, kill this command anytime with Ctrl-C):

```
journalctl -f -u enigma-node
-- Logs begin at Mon 2020-02-10 16:41:59 UTC. --
Feb 10 21:18:34 ip-172-31-41-58 enigmad[8814]: I[2020-02-10|21:18:34.307] Executed block                               module=state height=2629 validTxs=0 invalidTxs=0
Feb 10 21:18:34 ip-172-31-41-58 enigmad[8814]: I[2020-02-10|21:18:34.317] Committed state                              module=state height=2629 txs=0 appHash=34BC6CF2A11504A43607D8EBB2785ED5B20EAB4221B256CA1D32837EBC4B53C5
Feb 10 21:18:39 ip-172-31-41-58 enigmad[8814]: I[2020-02-10|21:18:39.382] Executed block                               module=state height=2630 validTxs=0 invalidTxs=0
Feb 10 21:18:39 ip-172-31-41-58 enigmad[8814]: I[2020-02-10|21:18:39.392] Committed state                              module=state height=2630 txs=0 appHash=17114C79DFAAB82BB2A2B67B63850864A81A048DBADC94291EB626F584A798EA
Feb 10 21:18:44 ip-172-31-41-58 enigmad[8814]: I[2020-02-10|21:18:44.458] Executed block                               module=state height=2631 validTxs=0 invalidTxs=0
Feb 10 21:18:44 ip-172-31-41-58 enigmad[8814]: I[2020-02-10|21:18:44.468] Committed state                              module=state height=2631 txs=0 appHash=D2472874A63CE166615E5E2FDFB4006ADBAD5B49C57C6B0309F7933CACC24B10
Feb 10 21:18:49 ip-172-31-41-58 enigmad[8814]: I[2020-02-10|21:18:49.532] Executed block                               module=state height=2632 validTxs=0 invalidTxs=0
Feb 10 21:18:49 ip-172-31-41-58 enigmad[8814]: I[2020-02-10|21:18:49.543] Committed state                              module=state height=2632 txs=0 appHash=A14A58E80FB24115DD41E6D787667F2FBBE003895D1B79334A240F52FCBD97F2
Feb 10 21:18:54 ip-172-31-41-58 enigmad[8814]: I[2020-02-10|21:18:54.613] Executed block                               module=state height=2633 validTxs=0 invalidTxs=0
Feb 10 21:18:54 ip-172-31-41-58 enigmad[8814]: I[2020-02-10|21:18:54.623] Committed state                              module=state height=2633 txs=0 appHash=C00112BB0D9E6812CEB4EFF07D2205D86FCF1FD68DFAB37829A64F68B5E3B192
Feb 10 21:18:59 ip-172-31-41-58 enigmad[8814]: I[2020-02-10|21:18:59.685] Executed block                               module=state height=2634 validTxs=0 invalidTxs=0
Feb 10 21:18:59 ip-172-31-41-58 enigmad[8814]: I[2020-02-10|21:18:59.695] Committed state                              module=state height=2634 txs=0 appHash=1F371F3B26B37A2173563CC928833162DDB753D00EC2BCE5EDC088F921AD0D80
^C
```

12. Add the following configuration settings (some of these avoid having to type some flags all the time):

```
enigmacli config chain-id enigma-1
enigmacli config output json
enigmacli config indent true
enigmacli config trust-node true # true if you trust the full-node you are connecting to, false otherwise
```

13. Generate a new key pair for yourself (change `<key-alias>` with any word of your choice, this is just for your internal/personal reference):

```
enigmacli keys add <key-alias>
```

14. Output your node address:

```
enigmacli keys show <key-alias> -a
```

15. Transfer tokens to the address displayed above.

16. Check that you have the requested tokens:

```
enigmacli q account $(enigmacli keys show -a <key_alias>)
```

If you get the following message, it means that you have no tokens yet:

```
ERROR: unknown address: account enigmaxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx does not exist
```

17. Join the network as a new validator: replace `<MONIKER>` with your own from step 4 above, and adjust the amount you want to stake (remember 1 SCRT = 1,000,000 uSCRT, and so the command below stakes 100k SCRT).

```
enigmacli tx staking create-validator \
  --amount=100000000000uscrt \
  --pubkey=$(enigmad tendermint show-validator) \
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

18. Check that you have been added as a validator:

```bash
enigmacli q staking validators
```

If the above is too verbose, just run: `enigmacli q staking validators | grep moniker`. You should see your moniker listed.

## Staking more tokens

In order to stake more tokens beyond those in the initial transaction, run:

```bash
enigmacli tx staking delegate $(enigmacli keys show <key-alias> --bech=val -a) <amount>uscrt --from <key-alias>
```

## Renaming your moniker

```bash
enigmacli tx staking edit-validator --moniker <new-moniker> --from <key-alias>
```

## Seeing your rewards from being a validator

```bash
enigmacli q distribution rewards $(enigmacli keys show -a <key-alias>)
```

## Seeing your commissions from your delegators

```bash
enigmacli q distribution commission $(enigmacli keys show -a <key-alias> --bech=val)
```

## Withdrawing rewards

```bash
enigmacli tx distribution withdraw-rewards $(enigmacli keys show --bech=val -a <key-alias>) --from <key-alias>
```

## Withdrawing rewards+commissions

```bash
enigmacli tx distribution withdraw-rewards $(enigmacli keys show --bech=val -a <key-alias>) --from <key-alias> --commission
```
