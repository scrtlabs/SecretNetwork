# HOWTO Rebranding

The [rebranding proposal](https://explorer.cashmaney.com/proposals/7) passed on-chain, and this mandates a hard fork.

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

### 4. Compile the new `secret` binaries with `make deb` (or distribute them precompiled).

### 5. Setup new binaries:

```bash
sudo dpkg -i precompiled_secret_package.deb # install secretd & secretcli and setup secret-node.service

secretcli config chain-id <new_chain_id>
secretcli config output json
secretcli config indent true
secretcli config trust-node true
```

### 6. Setup the new node/validaor:

```bash
# args for secretd init doesn't matter because we're going to import the old config files
secretd init <moniker> --chain-id <new_chain_id>

# import old config files to the new node
cp ~/.enigmad/config/{app.toml,config.toml,addrbook.json} ~/.secretd/config

# import node's & validator's private keys to the new node
cp ~/.enigmad/config/{priv_validator_key.json,node_key.json} ~/.secretd/config

# set new_genesis.json from step 3 as the genesis.json of the new chain
cp new_genesis.json ~/.secretd/config/genesis.json

# at this point you should also validate sha256 checksums of ~/.secretd/config/* against ~/.enigmad/config/*
```

### 7. Start the new Blockchain! :tada:

```bash
sudo systemctl enable secret-node # enable on startup
sudo systemctl start secret-node
```

Once more than 2/3 of voting power comes online you'll start seeing blocks streaming on:

```bash
journalctl -u secret-node -f
```

If something goes wrong the network can relaunch the `enigma-node`, therefore it's not advisable to delete `~/.enigmad` & `~/.enigmacli` until the new chain is live and stable.

### 8. Import wallet keys from the old chain to the new chain:

(Ledger Nano S/X users shouldn't do anything, just use the new CLI with `--ledger --account <number>` as usual)

```bash
enigmacli keys export <key_name>
# this^ outputs stuff to stderr and also exports the key to stderr,
# so copy only the private key output to a file named `key.export`

secretcli import <key_name> key.export
```

### 9. When the new chain is live and everything works well, you can delete the files of the old chain:

- `rm -rf ~/.enigmad`
- `rm -rf ~/.enigmacli`
- `sudo dpkg -r enigma-blockchain`
