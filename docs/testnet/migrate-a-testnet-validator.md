# Migrate a Validator

:warning: :warning: :warning:

Please make sure you [backup your validator](run-full-node-testnet.md) before you migrate it.

### 1. [Run a new full node](run-full-node-testnet.md) on a new machine.

### 2. Confirm you have the recovery seed phrase information for the active key running on the old machine

You can also back it up with:

On the validator node on the old machine:

```bash
secretcli keys export mykey
```

This prints the private key to `stderr`, you can then paste in into the file `mykey.backup`.

### 3. Recover the active key of the old machine on the new machine

This can be done with the mnemonics:

On the full node on the new machine:

```bash
secretcli keys add mykey --recover
```

Or with the backup file `mykey.backup` from the previous step:

On the full node on the new machine:

```bash
secretcli keys import mykey mykey.backup
```

### 4. Wait for the new full node on the new machine to finish catching-up.

To check on the new full node if it finished catching-up:

On the full node on the new machine:

```bash
secretcli status | jq .sync_info
```

(`catching_up` should equal `false`)

### 5. After the new node have caught-up, stop the validator node and then stop the new full node.

To prevert double signing, you should stop the validator node and only then stop the new full node.

Please read about [the dangers in running a validator](/validators-and-full-nodes/join-validator-mainnet.md#dangers-in-running-a-validator).

On the validator node on the old machine:

```bash
sudo systemctl stop secret-node
```

On the full node on the new machine:

```bash
sudo systemctl stop secret-node
```

### 6. Move the validator's private key from the old machine to the new machine.

On the old machine the file is `~/.secretd/config/priv_validator_key.json`.

You can copy it manually or for example you can copy the file to the new machine using ssh:

On the validator node on the old machine:

```bash
scp ~/.secretd/config/priv_validator_key.json ubuntu@new_machine_ip:~/.secretd/config/priv_validator_key.json
```

### 7. On the new server start the new full node which is now your validator node.

On the new machine:

```bash
sudo systemctl start secret-node
```
