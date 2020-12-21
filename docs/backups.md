# How to backup and restore everything

- [How to backup and restore everything](#how-to-backup-and-restore-everything)
- [Wallet](#wallet)
  - [Mnemonics](#mnemonics)
  - [Export](#export)
- [Client Transaction Encryption Key for Secret Contracs](#client-transaction-encryption-key-for-secret-contracs)
- [Validator Private Key](#validator-private-key)
- [Full Node Private key](#full-node-private-key)
- [Full Node Data](#full-node-data)

# Wallet

## Mnemonics

When you create a new key, you'll receive the mnemonic phrase that can be used to restore that key. Backup the mnemonic phrase:

```console
$ secretcli keys add mykey
{
  "name": "mykey",
  "type": "local",
  "address": "secret1zjqdn0j7fzsx5ldv0lf3ejfjkey0ce8e7vglnm",
  "pubkey": "secretpub1addwnpepqtu8pwkdft3cz65u0m84vh6k5kqmqf73lzs7we5dkx296mqlx7z6524jcxf",
  "mnemonic": "banner genuine height east ghost oak toward reflect asset marble else explain foster car nest make van divide twice culture announce shuffle net peanut"
}
```

To restore the key:

```console
$ secretcli keys add mykey-restored --recover
> Enter your bip39 mnemonic
banner genuine height east ghost oak toward reflect asset marble else explain foster car nest make van divide twice culture announce shuffle net peanut
{
  "name": "mykey-restored",
  "type": "local",
  "address": "secret1zjqdn0j7fzsx5ldv0lf3ejfjkey0ce8e7vglnm",
  "pubkey": "secretpub1addwnpepqtu8pwkdft3cz65u0m84vh6k5kqmqf73lzs7we5dkx296mqlx7z6524jcxf"
}

$ secretcli keys list
[
  {
    "name": "mykey-restored",
    "type": "local",
    "address": "secret1zjqdn0j7fzsx5ldv0lf3ejfjkey0ce8e7vglnm",
    "pubkey": "secretpub1addwnpepqtu8pwkdft3cz65u0m84vh6k5kqmqf73lzs7we5dkx296mqlx7z6524jcxf"
  },
  {
    "name": "mykey",
    "type": "local",
    "address": "secret1zjqdn0j7fzsx5ldv0lf3ejfjkey0ce8e7vglnm",
    "pubkey": "secretpub1addwnpepqtu8pwkdft3cz65u0m84vh6k5kqmqf73lzs7we5dkx296mqlx7z6524jcxf"
  }
]
```

Note: If the mnemonics were generated using a `secretci` version of `v0.0.x` or `v0.2.x`, you'll need to use this command: `secretcli keys add mykey-restored --recover --hd-path "44'/118'/0'/0/0"`.

## Export

To backup a local key without the mnemonic phrase, backup the output of `secretcli keys export`:

```console
$ secretcli keys export mykey
Enter passphrase to decrypt your key:
Enter passphrase to encrypt the exported key:
-----BEGIN TENDERMINT PRIVATE KEY-----
kdf: bcrypt
salt: 14559BB13D881A86E0F4D3872B8B2C82
type: secp256k1

3OkvaNgdxSfThr4VoEJMsa/znHmJYm0sDKyyZ+6WMfdzovDD2BVLUXToutY/6iw0
AOOu4v0/1+M6wXs3WUwkKDElHD4MOzSPrM3YYWc=
=JpKI
-----END TENDERMINT PRIVATE KEY-----

$ echo "\
-----BEGIN TENDERMINT PRIVATE KEY-----
kdf: bcrypt
salt: 14559BB13D881A86E0F4D3872B8B2C82
type: secp256k1

3OkvaNgdxSfThr4VoEJMsa/znHmJYm0sDKyyZ+6WMfdzovDD2BVLUXToutY/6iw0
AOOu4v0/1+M6wXs3WUwkKDElHD4MOzSPrM3YYWc=
=JpKI
-----END TENDERMINT PRIVATE KEY-----" > mykey.export
```

To restore the key:

```console
$ secretcli keys import mykey-imported ./mykey.export
Enter passphrase to decrypt your key:

$ secretcli keys list
[
  {
    "name": "mykey-imported",
    "type": "local",
    "address": "secret1zjqdn0j7fzsx5ldv0lf3ejfjkey0ce8e7vglnm",
    "pubkey": "secretpub1addwnpepqtu8pwkdft3cz65u0m84vh6k5kqmqf73lzs7we5dkx296mqlx7z6524jcxf"
  },
  {
    "name": "mykey-restored",
    "type": "local",
    "address": "secret1zjqdn0j7fzsx5ldv0lf3ejfjkey0ce8e7vglnm",
    "pubkey": "secretpub1addwnpepqtu8pwkdft3cz65u0m84vh6k5kqmqf73lzs7we5dkx296mqlx7z6524jcxf"
  },
  {
    "name": "mykey",
    "type": "local",
    "address": "secret1zjqdn0j7fzsx5ldv0lf3ejfjkey0ce8e7vglnm",
    "pubkey": "secretpub1addwnpepqtu8pwkdft3cz65u0m84vh6k5kqmqf73lzs7we5dkx296mqlx7z6524jcxf"
  }
]
```

# Client Transaction Encryption Key for Secret Contracs

1. Backup `~/.secretcli/id_tx_io.json`.

This key encrypts you inputs and decrypts the outputs to/of Secret Contracts.

# Validator Private Key

1. Backup `~/.secretd/config/priv_validator_key.json`.
2. Backup the self-delegator wallet. See the [wallet section](#wallet).

Also see [Backup a Validator](validators-and-full-nodes/backup-a-validator.md) and [Migrate a Validator](validators-and-full-nodes/migrate-a-validator.md).

# Full Node Private key

1. Backup `~/.secretd/config/node_key.json`.

Why you might want to backup your node ID:

- In case you are a seed node.
- In case you are a persistent peer for other full nodes.
- In case you manage a setup of senty nodes and use node IDs in your config files.

# Full Node Data

1. Gracefully shut down the node:

   ```bash
   sudo systemctl stop secret-node
   ```

2. Backup the `~/.secretd/data/` directory except for the `~/.secretd/data/priv_validator_state.json` file.
3. Backup the `~/.secretd/.compute/` directory.
4. Restart the node:

   ```bash
   sudo systemctl start secret-node
   ```
