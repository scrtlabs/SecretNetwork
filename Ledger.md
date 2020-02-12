# Use your Ledger with Enigma chain!

## Prerequisites
* This guide assumes you have a verified, genuine Ledger Nano S device.
* If you don't, or you using your Ledger device for the first time, you should check Ledger's [Getting Started](https://support.ledger.com/hc/en-us/sections/360001415213-Getting-started) guide.
* We also advise you to check your Ledger's genuineness and upgrade your firmware to the newest one available.
* Have a machine with [Ledger Live](https://www.ledger.com/ledger-live) installed.
* Have the latest version of our latest binaries installed. You can get it [here](https://github.com/enigmampc/enigmachain/releases).

## Install Cosmos Ledger App

* Open Ledger Live and go to Settings (gear icon on the top right corner):
![](https://raw.githubusercontent.com/cosmos/ledger-cosmos/master/docs/img/cosmos_app1.png)

* Enable developer mode:                           
![](https://raw.githubusercontent.com/cosmos/ledger-cosmos/master/docs/img/cosmos_app2.png)

* Now go to Manager and search "Cosmos":
![](https://raw.githubusercontent.com/cosmos/ledger-cosmos/master/docs/img/cosmos_app3.png)

* Hit "Install" and wait for process to complete.

*Ref: https://github.com/cosmos/ledger-cosmos*

## Common commands

These are some basic examples for commands you can use with your Ledger. You may notice that most commands stay the same, you just need to add the `--ledger` flag.                      

**Note: To run these commands below, or any command that requires signing with your Ledger device, you need your Ledger to be opened on the Cosmos App:**                                                  
![](https://miro.medium.com/max/1536/1*Xfi5_ScAiFn6rr9YBjgFFw.jpeg)
*Ref: https://medium.com/cryptium-cosmos/how-to-store-your-cosmos-atoms-on-your-ledger-and-delegate-with-the-command-line-929eb29705f*

### Create an account

```bash
enigmacli keys add <account name> --ledger --account <account number on your Ledger>
```

### Add an account to `engcli` that already exists on your Ledger
*You'll use this when you, say, using a different machine.*

```bash
enigmacli keys add <account name> --ledger --account <account number on your Ledger> --recover
```

**Note! If you run the above command without the `--ledger` flag, the CLI will prompt you to enter your BIP39 mnemonic, which is your Ledger recovery phrase. YOU DO NOT WANT TO DO THIS. This will essentialy save your private key locally.**

### Send tokens

```bash
enigmacli tx send <account name or address> <to_address> <amount> --ledger
```

### Bond SCRT to a validator

```bash
enigmacli tx staking delegate <validator address> <amount to bond> --from <account key> --gas auto --gas-prices <gasPrice> --ledger
```

### Collect rewards and commission

```bash
enigmacli tx distribution withdraw-all-rewards --from <account name> --gas auto --commission --ledger
```

### Vote on proposals

```bash
enigmacli tx gov vote <proposal-id> <vote> --from <account name> --ledger
```