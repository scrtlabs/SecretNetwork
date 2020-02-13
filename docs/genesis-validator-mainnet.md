# How to join EnigmaChain as a mainnet genesis validator

This document details how to join the EnigmaChain `mainnet` as a genesis validator.

## Requirements

- Ubuntu/Debian host (with ZFS or LVM to be able to add more storage easily)
- A public IP address
- Open ports `TCP 26656 - 26660`

## Installation

### 0. Download the [EnigmaChain package installer](https://github.com/enigmampc/enigmachain/releases/download/v0.0.1/enigmachain_0.0.1_amd64.deb) (Debian/Ubuntu):

**Note**: The new binaries are now called `enigmacli` & `enigmad`.

```bash
wget -O enigmachain_0.0.1_amd64.deb https://github.com/enigmampc/enigmachain/releases/download/v0.0.1/enigmachain_0.0.1_amd64.deb
echo "13b06329543dcbe6ca896406887afb79f7f8b975e5d5585db1943e4520b77521 enigmachain_0.0.1_amd64.deb" | sha256sum --check
```

### 1. Make sure you don't have a previous installation (from testnet):

**Note:** If you will be using the same key from testnet you can export if with `enigmacli keys export <key-alias> > my.key` and later import it with `enigmacli keys import <key-alias> my.key`.

```bash
sudo dpkg -r enigmachain
sudo rm -rf ~/.enigmad ~/.enigmacli
sudo rm -rf ~/.engd ~/.engcli
sudo rm -rf "$(which enigmad)"
sudo rm -rf "$(which enigmacli)"
sudo rm -rf "$(which engcli)"
sudo rm -rf "$(which engd)"
```

### 2. Install the enigmachain package:

```bash
sudo dpkg -i enigmachain_0.0.1_amd64.deb
```

### 3. Update the configuration file that sets up the system service with your current user as the user this service will run as.

_Note: Even if we are running this command and the previous one with sudo, this package does not need to be run as root_.

```bash
sudo perl -i -pe "s/XXXXX/$USER/" /etc/systemd/system/enigma-node.service
```

### 4. Add the following configuration settings (some of these avoid having to type some flags all the time):

```bash
enigmacli config chain-id "enigma-1"
enigmacli config output json
enigmacli config indent true
enigmacli config trust-node true # true if you trust the full-node you are connecting to, false otherwise
```

### 5. Generate a new key pair for yourself (change `<key-alias>` with any word of your choice, this is just for your internal/personal reference):

**Note:** If you will be using the same key you exported in step 1, you can import it with `enigmacli keys import <key-alias> my.key`.

```bash
enigmacli keys add <key-alias>
```

### 6. Output your key address:

```bash
enigmacli keys show <key-alias> -a
```

### 7. Request tokens be allocated in genesis to the address displayed above.

**:warning:Note:warning:: You must finish this step before Thursday 2020-02-13 15:00:00 UTC**

### 8. Download a copy of the Genesis Block file: `genesis.json`

```bash
mkdir -p ~/.enigmad/config/
wget -O ~/.enigmad/config/genesis.json https://gist.githubusercontent.com/assafmo/be72c7275bfeaeeb53f6bb8eb995207b/raw/0035d6ef7b8212c77439c71bee777c934758373d/genesis.json
```

### 9. Validate the checksum for the `genesis.json` file you have just downloaded in the previous step:

```bash
echo "7ae57ee77b5ad4369790b357a26afc54d709234953ea6c5422e26f8dbf2dbd00 $HOME/.enigmad/config/genesis.json" | sha256sum --check
```

### 10. Validate that the `genesis.json` is a valid genesis file:

```bash
enigmad validate-genesis
```

### 11. Set your moniker, which is your public validator name. Replace `<MONIKER>` with your own:

```bash
perl -i -pe 's/moniker = ".*?"/moniker = "<MONIKER>"/' ~/.enigmad/config/config.toml
```

### 12. Inspect `~/.enigmad/config/genesis.json` to make sure you agree with the parameters and the amount of `uSCRT` allocated to your account. Remember that 1`SCRT` equals to 1,000,000`uSCRT`.

### 13. Create a genesis validator transaction file:

```bash
enigmad gentx --name <key-alias> --amount <amount-to-stake>uscrt --ip <your-PUBLIC-ip-or-dns>
```

**Note**: You should stake at least 100000000 uscrt (100 SCRT). After the mainnet launch you can [transfer the rest of your funds to a ledger](https://github.com/enigmampc/enigmachain/blob/master/Ledger.md) and [delegate more funds](https://gist.github.com/assafmo/0d3c789aa51de9217b3937b3e5671686#staking-more-tokens) to your validator from there.

### 14. Now upload the genesis validator transaction file you created in step 13 and send us the pastebin link:

```bash
sudo apt install -y pastebinit
```

```bash
pastebinit -b pastebin.com ~/.enigmad/config/gentx/*.json
```

### 15. Download a copy of the new Genesis Block file `genesis.json`

This genesis.json includes all the genesis stake transactions from all the validators.

```bash
wget -O ~/.enigmad/config/genesis.json "https://raw.githubusercontent.com/enigmampc/enigmachain/master/enigma-1-genesis.json"
```

### 16. Validate the checksum for the `genesis.json` file you have just downloaded in the previous step:

```bash
echo "86cd9864f5b8e7f540c5edd3954372df94bd23de62e06d5c33a84bd5f3d29114 $HOME/.enigmad/config/genesis.json" | sha256sum --check
```

### 17. Validate that the `genesis.json` is a valid genesis file:

```bash
enigmad validate-genesis
```

### 18. Enable `enigma-node` as a system service:

```bash
sudo systemctl enable enigma-node
```

### 19. Start `enigma-node` as a system service:

```bash
sudo systemctl start enigma-node
```

### 20. If everything above worked correctly, the following command will show your node **waiting** for at least 67% of the voting power to come online. This can take a while so be patient.

```
journalctl -f -u enigma-node
Feb 13 13:31:15 ip-172-31-44-28 enigmad[18653]: I[2020-02-13|13:31:15.208] starting ABCI with Tendermint
^C
```

(this is for debugging purposes only, kill this command anytime with Ctrl-C)

### 21. At some point (could take hours) the network will come alive. The following command will show your node streaming blocks (this is for debugging purposes only, kill this command anytime with Ctrl-C):

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

### 22. Check that you have your tokens minus your stake:

```bash
enigmacli q account $(enigmacli keys show -a <key_alias>)
```

### 23. Check that you have been added as a validator:

```bash
enigmacli q staking validators
```

If the above is too verbose, just run: `enigmacli q staking validators | grep moniker`. You should see your moniker listed.

## Staking more tokens

(remember 1 SCRT = 1,000,000 uSCRT)

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
