# How to migrate a validator to a new server

Please make sure you [backup your validator](/docs/validators_and_full_nodes/backup-a-validator.md) before you migrate it.

### 1. [Run a new full node](/docs/validators_and_full_nodes/run-full-node-mainnet.md) on a new server.

### 2. Confirm you have the recovery seed phrase information for the active key running on the old machine

You can also back it up with:

```bash
# On the validator node on the old machine:
enigmacli keys export mykey > mykey.backup
```

### 3. Recover the active key of the old machine on the new machine

This can be done with the mnemonics:

```bash
# On the full node on the new machine:
enigmacli keys add mykey --recover
```

Or with the backup file `mykey.backup` from the previous step:

```bash
# On the full node on the new machine:
enigmacli keys import mykey mykey.backup
```

### 4. Wait for the new full node on the new machine to finish catching-up.

To check on the new full node if it finished catching-up:

```bash
# On the full node on the new machine:
enigmacli status | jq .sync_info
```

(`catching_up` should equal `false`)

### 5. After the new node have caught-up, stop the validator node and then stop the new full node.

To prevert double signing, you should stop the validator node and only then stop the new full node.

```bash
# On the validator node on the old machine:
sudo systemctl stop enigma-node
```

```bash
# On the full node on the new machine:
sudo systemctl stop enigma-node
```

### 6. Move the validator's private key from the old machine to the new machine.

On the old machine the file is `~/.enigmad/config/priv_validator_key.json`.

You can copy it manually or for example you can copy the file to the new machine using ssh:

```bash
# On the validator node on the old machine:
scp ~/.enigmad/config/priv_validator_key.json ubuntu@new_machine_ip:~/.enigmad/config/priv_validator_key.json
```

### 7. On the new server start the new full node which is now your validator node.

```bash
# On the new machine:
sudo systemctl start enigma-node
```
