# HOWTO Romulus Upgrade

The [rebranding proposal](https://explorer.cashmaney.com/proposals/13) passed on-chain, and this mandates a hard fork.

The network needs to decide on a block number to fork from.
Since most nodes use `--pruning syncable` configuration, the node prunes most of the blocks, so state should be exported from a height that is a multiple of 100 (e.g. 100, 500, 131400, ...).

For better background, before reading this guide you might want to check out Cosmos' guide upgrading from `cosmoshub-2` to `cosmoshub-3`: https://github.com/cosmos/gaia/blob/master/docs/migration/cosmoshub-2.md

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

### 8. Setup the new node/validaor:

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
