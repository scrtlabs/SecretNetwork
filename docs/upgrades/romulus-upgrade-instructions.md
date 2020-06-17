# Romulus Upgrade Instructions

The [Romulus Upgrade Signal](https://puzzle.report/enigma/chains/enigma-1/governance/proposals/13) passed on-chain, and this mandates a hard fork.

This document describes the steps required to perform the Romulus Upgrade to go from the `enigma-1` to `secret-1` chain. The upgrade is 
required for all full-node operators (both validators and non-validators).

The Chain of Secrets (CoS) team will post the official modified genesis file, but you'll be able to validate it with a step below.

The agreed upon block height for the Romulus Upgrade is *1,794,500*.

_NOTE: Full-nodes includes any Sentry nodes that are part of a validator's network architecture._

This document describes the upgrade in the following sections:

- Preliminary
- Risks
- Recovery
- Upgrade Procedure


## Preliminary

The significant changes in this upgrade are the following:

- The _Secret Network_ re-branding.

The `enigmacli/enigmad` commands change to `secretcli/secretd`.

The `enigma1...` addresses change to `secret1...`. The addresses will go through a bech32 address converter supplied by the Enigma team to properly 
change the addresses. You'll see not only the address prefix change, but also the entire address. Wallet keys will then be exported using the 
`enigmacli` command and imported using `secretcli`. There may seem to be a bit of magic there, but it's been tested and works great!

- Addition of a tokenswap module.

This module has been added to allow the chain implementation of the _Burn ENG for SCRT!_ proposal: https://puzzle.report/enigma/chains/enigma-1/governance/proposals/4

## Risks

To mitigate the risk of jailed/slashed validator addresses causing an issue when starting the new chain, CoS will run a script to modify the 
genesis file to ensure the correct staked amounts. This script is being provided by a key contributor to the Secret Network.

There is no risk of 'double-signing' unless you have two nodes running the same keys in parallel. Please ensure that is not the case for your nodes.

We are using a modified fork of the cosmos-sdk to address issues with using a genesis file created via an `export`. There is a risk that an export 
of the current chain state may reveal a new issue.

If necessary, the network can be relaunched with the old chain `enigma-1`. For this reason do not delete the existing `.enigmad` and `.enigmcli` 
directories. See the *Recovery* section below. 

## Recovery

In the event that something goes wrong and we need to revert back to the old chain, the following steps should be performed by all full-node 
operators and validators.

1. Stop the `secret-1` chain if running:

```bash
# may fail, but that's okay
sudo systemctl stop secret-node
```

2. Re-start the `enigma-1` chain:

```bash
sudo systemctl enable enigma-node
sudo systemctl start enigma-node
```

3. Monitor the enigma-node (once 2/3 of voting power is online, you'll see blocks streaming):

```bash
journalctl -u enigma-node -f
```

NOTE: you may have to put `sudo` in front of the `journalctl` command if you don't have permission to run it.


## Upgrade Procedure

### 1. Gracefully Halt the `enigma-1` Chain

Change the configured halt height in `app.toml`:

```bash
perl -i -pe 's/^halt-height =.*/halt-height = 1794500/' ~/.enigmad/config/app.toml
```

Change the `enigma-node` service configuration to restart only on failure (so it doesn't keep trying to restart after the chain halt):

```bash
sudo perl -i -pe 's/^Restart=.*/Restart=on-failure/' /etc/systemd/system/enigma-node.service
```

Display the `enigma-node` service file:

```bash
cat /etc/systemd/system/enigma-node.service
```

It should look similar to below:

```bash
[Unit]
Description=Enigma node service
After=network.target

[Service]
Type=simple
ExecStart=/bin/enigmad start
User=ubuntu
Restart=on-failure
StartLimitInterval=0
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

Reload the `enigma-node` service configuration:

```bash
sudo systemctl daemon-reload
```

Restart the `enigma-node` service and monitor:

```bash
sudo systemctl restart enigma-node
```


*The chain will halt at block height _1794500_ at approximately 9am PDT, 12pm EDT and 4pm UTC.*

You may see dialing and connection errors as nodes are halted, which is expected. When the node is finally stopped, you'll see these messages:

```bash
halting node per configuration


exiting... 
```

### 2. Migrate Genesis

*** NOTE: *Chain of Secrets* performs these steps! ***

CoS will notify mainnet validators and full-node operators when this step is complete.

It is recommended that participants go through the steps to independently verify the migrated genesis file.

See the [Addendum](#addendum) for detailed instructions.


## All Full-Node Operators and Validators

### 3. Setup Secret Network 

Get the `secretnetwork` release:

```bash
wget -O secretnetwork_0.2.0_amd64.deb https://github.com/chainofsecrets/TheRomulusUpgrade/releases/download/v0.2.0/secretnetwork_0.2.0_amd64.deb
```

Install the release and configure:

```bash
sudo dpkg -i secretnetwork_0.2.0_amd64.deb # install secretd & secretcli and setup secret-node.service

```

Verify the package version for the Secret Network:

```bash
secretcli version --long | jq .
```

Below is the version information for the Secret Network.

```bash
{
  "name": "SecretNetwork",
  "server_name": "secretd",
  "client_name": "secretcli",
  "version": "0.2.0-220-g3d4eb01",
  "commit": "3d4eb015191ff8d7c5754f4588e0aabff20a1ab5",
  "build_tags": "ledger",
  "go": "go version go1.14.4 linux/amd64"
}
```

Configure the node:

```bash
secretcli config chain-id secret-1
secretcli config output json
secretcli config indent true
secretcli config trust-node true
```


### 4. Setup the new Node/Validator:

Get the new `secret-1` genesis file (this will be provided by CoS after Step #2 is completed):

```bash
wget -O https://github.com/chainofsecrets/TheRomulusUpgrade/blob/romulus-upgrade/secret-1-genesis.json
```

Validate the genesis file (replace \<sha256sum> with the checksum provided by CoS after Step #2 is completed):


```bash
echo "<sha256sum> secret-1-genesis.json" | sha256sum --check
```

Initialize and configure `secretd` with a placeholder moniker "blabla" because it will be overwritten with the correct `app.toml` file:

```bash
secretd init blabla --chain-id secret-1

cp ~/.enigmad/config/{app.toml,config.toml,addrbook.json} ~/.secretd/config
cp ~/.enigmad/config/{priv_validator_key.json,node_key.json} ~/.secretd/config

# set new_genesis.json from step 3 as the genesis.json of the new chain
cp secret-1-genesis.json ~/.secretd/config/genesis.json
```


### 5. Start the new Secret Node! :tada:

Enable the new node and start:

```bash
sudo systemctl enable secret-node
sudo systemctl start secret-node
```

Once more than 2/3 of voting power comes online you'll start seeing blocks streaming on:

```bash
journalctl -u secret-node -f
```

If something goes wrong the network can relaunch the `enigma-node`, therefore it's not advisable to delete `~/.enigmad` & `~/.enigmacli` until 
the new chain is live and stable.

### 6. Import Wallet Keys

(Ledger Nano S/X users shouldn't do anything, just use the new CLI with `--ledger --account <number>` as usual)

```bash
enigmacli keys export <key_name>
# this^ outputs stuff to stderr and also exports the key to stderr,
# so copy only the private key output to a file named `key.export`

secretcli keys import <key_name> key.export
```

### 7. Remove Old Chain

When the `secret-1` chain is live and stable, you can delete the files of the old `enigma-1` chain:

- `sudo systemctl disable enigma-node`
- `rm -rf ~/.enigmad`
- `rm -rf ~/.enigmacli`
- `sudo dpkg -r enigma-blockchain`


## <a name="#addendum"></a>Addendum

See the [Migrate Genesis instructions](romulus-migrate-genesis.md).


