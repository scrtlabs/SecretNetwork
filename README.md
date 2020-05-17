# HOWTO Rebranding

The [rebranding proposal](https://explorer.cashmaney.com/proposals/7) passed on-chain, and this mandates a hard fork.

The network needs to decide on a block number to fork from.
Since most nodes use `--pruning syncable` configuration, the node prunes most of the blocks, so state should be exported from a height that is a multiple of 100 (e.g. 100, 500, 131400, ...).

For better background, before reading this guide you might want to check out Cosmos' guide upgrading from `cosmoshub-2` to `cosmoshub-3`.

### 1. Export `genesis.json` for the new fork:

```bash
sudo systemctl stop enigma-node
enigmad export --for-zero-height --height <agreed_upon_block_height> > new_genesis.json
```

### 2. Inside `new_genesis.json` Rename `chain_id` from `enigma-1` to the new agreed upon Chain ID.

### 3. Convert all enigma addresses to secret adresses.

You can just paste `new_genesis.json` into https://bech32.enigma.co and paste the result back into `new_genesis.json`.
(TODO maybe use [this](https://github.com/enigmampc/bech32.enigma.co/blob/8c7cec466923295fcf2d10cacfc2dafd3932e255/src/App.js#L82-L116) to make a CLI)

### 4. Compile the new `scrt` binaries with `make deb` (or distribute them precompiled).

### 5. Setup new binaries:

```bash
sudo dpkg -i precompiled_scrt_package.deb # install scrtd & scrtcli and setup scrt-node.service

scrtcli config chain-id <new_chain_id>
scrtcli config output json
scrtcli config indent true
scrtcli config trust-node true
```

### 6. Setup the new node/validaor:

```bash
# args for scrtd init doesn't matter because we're going to import the old config files
scrtd init <moniker> --chain-id <new_chain_id>

# import old config files to the new node
cp ~/.enigmad/config/{app.toml,config.toml,addrbook.json} ~/.scrtd/config

# import node's & validator's private keys to the new node
cp ~/.enigmad/config/{priv_validator_key.json,node_key.json} ~/.scrtd/config

# set new_genesis.json from step 3 as the genesis.json of the new chain
cp new_genesis.json ~/.scrtd/config/genesis.json

# at this point you should also validate sha256 checksums of ~/.scrtd/config/* against ~/.enigmad/config/*
```

### 7. Start the new Blockchain! :tada:

```bash
sudo systemctl enable secret-node # enable on startup
sudo systemctl start secret-node
```

When more than 2/3 of voting power gets online you'll start to see blocks streaming on:

```bash
journalctl -u secret-node -f
```

If something goes wrong the network can relaunch the `enigma-node`, therefore it's not advisable to delete `~/.enigmad`, `~/.enigmacli` until the new chain is live and stable.

### 8. Import wallet keys from the old chain to the new chain:

(Ledger Nano S/X users should do anything, just use the new CLI with `--ledger --account <number>`)

```bash
enigmacli keys export <key_name>
# this^ outputs stuff the stderr and also exports the key to stderr,
# so copy only the private key output to file `key.export`

scrtcli import <key_name> key.export
```

### 9. When the new chain is live and everything works well, you can delete the files of the old chain:

- `rm -rf ~/.enigmad`
- `rm -rf ~/.enigmacli`
- `sudo dpkg -r enigma-blockchain`
