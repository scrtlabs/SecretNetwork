# Romulus Upgrade Instructions

The [Romulus Upgrade Signal](https://explorer.cashmaney.com/proposals/13) passed on-chain, and this mandates a hard fork.

This document describes the steps required to perform the Romulus Upgrade to go from the `enigma-1` to `secret-1` chain. The upgrade is 
required for all full-node operators (both validators and non-validators).

The CoS team will post the official modified genesis file, but you'll be able to validate it with a step below.

The agreed upon block height for the Romulus Upgrade is *1,794,500*.

NOTE: Full-nodes includes any Sentry nodes that are part of a validator's network architecture.

- Preliminary
- Risks
- Recovery
- Upgrade Procedure

## Preliminary

The significant changes in this upgrade are the following:

- The _Secret Network_ re-branding.

The `enigmacli/enigmad` commands change to `secretcli/secretd`.

The `enigma1...` addresses change to `secret1...`. The addresses will go through a bech32 coverter supplied by the Enigma team to properly 
change the addresses. You'll see not only the address prefix change, but also the entire address. Wallet keys will then be exported using the 
`enigmacli` command and imported using `secretcli`. There may seem to be a bit of magic there, but it's been tested and works great!

- Addition of a tokenswap module.

This module has been added to allow the chain implementation of the _Burn ENG for SCRT!_ proposal: https://puzzle-staging.secretnodes.org/enigma/chains/enigma-1/governance/proposals/4

## Risks

To mitigate the risk of jailed/slashed validator addresses causing an issue when starting the new chain, CoS will run a script to modify the 
genesis file to ensure the correct staked amounts. This script is being provided by a key contributor to the Secret Network.

There is no risk of 'double-signing' unless you have two nodes running the same keys in parallel. Please ensure that is not the case for your nodes.

We are using a modified fork of the cosmos-sdk to address issues with using a genesis file created via an `export`. There is a risk that an export 
of the current chain state may reveal a new issue, not foreseen.

If necessary, the network can be relaunched with the old chain `enigma-1`. For this reason do not delete the existing `.enigmad` and `.enigmcli` 
directories. See the *Recovery* section below. 

## Recovery

In the event that something goes wrong and we need to revert back to the old chain, the following steps should be performed by all full-node 
operators and validators.

1. Stop the `secret-1` chain if running:

```bash
# may fail, but that's okay
$ sudo systemctl stop secret-node
```

2. Re-start the `enigma-1` chain:

```bash
$ sudo systemctl enable enigma-node
$ sudo systemctl start enigma-node
```

3. Remove `secretnetwork` package and directories:

```bash
$ rm -rf ~/.secretd
$ rm -rf ~/.secretcli
$ sudo dpkg -r secretnetwork
```

4. Monitor the enigma-node (once 2/3 of voting power is online, you'll see blocks streaming):

```bash
$ journalctl -u enigma-node -f
```

NOTE: you may have to put `sudo` in front of the `journalctl` command if you don't have permission to run it.


## Upgrade Procedure

### 1. Export `genesis.json` for the new fork:

```bash
sudo systemctl stop enigma-node
enigmad export --for-zero-height --height <agreed_upon_block_height> > exported_state.json
```

### 2. Inside `exported_state.json` Rename `chain_id` from `enigma-1` to the new agreed upon Chain ID (`secret-1`)

For example:

```bash
perl -i -pe 's/"enigma-1"/"secret-1"/' exported_state.json
```

### 3. Convert all `enigma` addresses to `secret` adresses

Using the CLI:

```bash
wget https://github.com/enigmampc/bech32.enigma.co/releases/download/cli/bech32-convert
chmod +x bech32-convert

cat exported_state.json | ./bech32-convert > secret-1-genesis.json
```

Or you can just paste `exported_state.json` into https://bech32.enigma.co and paste the result back into `secret-1-genesis.json`.

### 4. Use `jq` to make the `secret-1-genesis.json` more readable

```bash
jq . secret-1-genesis.json > secret-1-genesis-jq.json
mv secret-1-genesis-jq.json secret-1-genesis.json

```

NOTE: if you don't have `jq`, you can install it with `sudo apt-get install jq`

### 5. Add Tokenswap parameters

Modify the `secret-1-genesis.json` and add the following tokenswap parameters under `gov`:

```bash
	"tokenswap": {
		"params": {
			"minting_approver_address": "",
			"minting_multiplier": "1.000000000000000000",
			"minting_enabled": false
		},
		"swaps": null
	},
```

### 6. Compile the new `secret` binaries with `make deb` (or distribute them precompiled).

```bash
secretnetwork_0.2.0_amd64.deb
```

### 7. Setup new binaries:

```bash
sudo dpkg -i secretnetwork_0.2.0_amd64.deb # install secretd & secretcli and setup secret-node.service

secretcli config chain-id secret-1
secretcli config output json
secretcli config indent true
secretcli config trust-node true
```

### 8. Setup the new node/validator:

```bash
# args for secretd init doesn't matter because we're going to import the old config files
secretd init <moniker> --chain-id secret-1

# import old config files to the new node
cp ~/.enigmad/config/{app.toml,config.toml,addrbook.json} ~/.secretd/config

# import node's & validator's private keys to the new node
cp ~/.enigmad/config/{priv_validator_key.json,node_key.json} ~/.secretd/config

# set new_genesis.json from step 3 as the genesis.json of the new chain
cp new_genesis.json ~/.secretd/config/genesis.json

# at this point you should also validate sha256 checksums of ~/.secretd/config/* against ~/.enigmad/config/*

echo "9167cc828f5060507af42f553ee6c2f0270a4118c6bf1a0912171f4a14961143 $HOME/.secretd/config/genesis.json" | sha256sum --check
```

### 9. Start the new Secret Node! :tada:

```bash
sudo systemctl enable secret-node # enable on startup
sudo systemctl start secret-node
```

Once more than 2/3 of voting power comes online you'll start seeing blocks streaming on:

```bash
journalctl -u secret-node -f
```

If something goes wrong the network can relaunch the `enigma-node`, therefore it's not advisable to delete `~/.enigmad` & `~/.enigmacli` until the new chain is live and stable.

### 10. Import wallet keys from the old chain to the new chain:

(Ledger Nano S/X users shouldn't do anything, just use the new CLI with `--ledger --account <number>` as usual)

```bash
enigmacli keys export <key_name>
# this^ outputs stuff to stderr and also exports the key to stderr,
# so copy only the private key output to a file named `key.export`

secretcli import <key_name> key.export
```

### 11. When the new chain is live and everything works well, you can delete the files of the old chain:

- `rm -rf ~/.enigmad`
- `rm -rf ~/.enigmacli`
- `sudo dpkg -r enigma-blockchain`
