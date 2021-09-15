# Network Upgrade Instructions from `secret-2` to `secret-3`

:warning: Please read carefully before you begin the upgrade.

- [Network Upgrade Instructions from `secret-2` to `secret-3`](#network-upgrade-instructions-from-secret-2-to-secret-3)
- [Validators](#validators)
  - [1. Prepare your `secret-2` validator to halt at 2021-09-14 18:00:00 UTC.](#1-prepare-your-secret-2-validator-to-halt-at-2021-09-14-180000-utc)
  - [2. Install the new binaries on your SGX machine](#2-install-the-new-binaries-on-your-sgx-machine)
  - [3. Import the quicksync data](#3-import-the-quicksync-data)
  - [4. Migrate your validator's signing key](#4-migrate-your-validators-signing-key)
  - [5. Migrate your node's encrypted seed](#5-migrate-your-nodes-encrypted-seed)
  - [6. Migrate your validator's wallet](#6-migrate-your-validators-wallet)
  - [7. Set up your SGX machine and become a `secret-3` validator](#7-set-up-your-sgx-machine-and-become-a-secret-3-validator)
- [In case of an upgrade failure](#in-case-of-an-upgrade-failure)
- [Removing an installation](#removing-an-installation)
  - [Appendix: Registration on a new Secret-3 node](#appendix-registration-on-a-new-secret-3-node)

# Validators

:warning: Don't delete your `secret-2` machine, as we might have to relaunch it.

You're probably familiar with SGX by now:

- [Setup SGX](validators-and-full-nodes/setup-sgx.md)
- [Verify SGX](validators-and-full-nodes/verify-sgx.md)

## 1. Prepare your `secret-2` validator to halt at 2021-09-14 18:00:00 UTC.

On the old machine (`secret-2`):

```bash
perl -i -pe 's/^halt-time =.*/halt-time = 1631642400/' ~/.secretd/config/app.toml

sudo systemctl restart secret-node
```

:warning: If you do install `secret-3` on your `secret-2` machine, run:

- `sudo systemctl stop secret-node`
- `mv ~/.secretd ~/.secretd.backup`
- `secretd init <MONIKER>`
- `mkdir -p ~/.secretd/.node`
- `cp ~/.secretd.backup/.node/seed.json ~/.secretd/.node/seed.json`
- You'll be able to skip node regiatration steps later on

## 2. Install the new binaries on your SGX machine

On the new SGX machine (`secret-3`):

```bash
cd ~

wget "https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.5/secretnetwork_1.0.5_amd64.deb"

echo "6b0259f3669ab81d41424c1db5cea5440b00eb3426cac3f9246d0223bbf9f74c secretnetwork_1.0.5_amd64.deb" | sha256sum --check

sudo apt install -y ./secretnetwork_1.0.5_amd64.deb

sudo chmod +x /usr/local/bin/secretd

secretd init <MONIKER> --chain-id secret-3
```

## 3. Import the quicksync data

```bash
cd ~

wget "https://engfilestorage.blob.core.windows.net/quicksync-secret-3/quicksync.tar.xz"

echo "66fe25ae54a8c3957999300c5955ee74452c7826e0a5e0eabc2234058e5d601d quicksync.tar.xz" | sha256sum --check

pv quicksync.tar.xz | tar -xJf -
```

## 4. Migrate your validator's signing key

Copy your `~/.secretd/config/priv_validator_key.json` from the old machine (`secret-2`) to the new SGX machine (`secret-3`) at the same location.
e.g. `cp ~/.secretd.backup/config/priv_validator_key.json ~/.secretd/config/priv_validator_key.json`

## 5. Migrate your node's encrypted seed

Back up the file `seed.json` from `~/.secretd/.node/seed.json`:

You will need to copy this file from the old machine (`secret-2`) to the new SGX machine (`secret-3`) at the same location.

## 6. Migrate your validator's wallet

Export the self-delegator wallet from the old machine (`secret-2`) and import to the new SGX machine (`secret-3`).

On the old machine (`secret-2`) use `secretcli keys export "$YOUR_KEY_NAME"`.  
On the new SGX machine (`secret-3`) use `secretcli keys import "$YOUR_KEY_NAME" "$FROM_FILE_NAME"`

Notes:

1. If you're recovering the wallet using `secretcli keys add "$YOUR_KEY_NAME" --recover` you should also use `--hd-path "44'/118'/0'/0/0"`.
2. If the wallet is stored on a Ledger device, use `--legacy-hd-path` when importing it with `secretcli keys add`.

## 7. Set up your SGX machine and become a `secret-3` validator

```bash
cd ~

secretd init <MONIKER> --chain-id secret-3

wget -O ~/.secretd/config/genesis.json "https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.5/genesis.json"

echo "1c5682a609369c37e2ca10708fe28d78011c2006045a448cdb4e833ef160bf3f .secretd/config/genesis.json" | sha256sum --check

mkdir -p ~/.secretd/.node

cp ~/.secretd.backup/.node/seed.json ~/.secretd/.node/seed.json  # or wherever you stored the file

perl -i -pe 's/pruning =.*/pruning = "everything"/' ~/.secretd/config/app.toml

perl -i -pe 's/persistent_peers =.*/persistent_peers = "3612fb4f7b146f45e8f09a8b8c36ebc041934049\@185.56.139.85:26656,b8e2408b7f4cb556b71350ea4c6930b8db1e2599\@anode1.trivium.xiphiar.com:26656,e768e605f9a3a8eb7c36c36a6dbf9bd707ac0bd0\@bootstrap.secretnodes.org:26656,27db2f21cfcbfa40705d5c516858f51d5af07e03\@20.51.225.193:26656"/' ~/.secretd/config/config.toml

sudo systemctl enable secret-node

sudo systemctl start secret-node # (Now your new node is live and catching up)

secretcli config node tcp://localhost:26657
```

Now wait until you're done catching up. This is fast.  
Once the following command outputs `true` you can continue:

```bash
watch 'secretcli status | jq ".sync_info.catching_up == false"'
```

Once your node is done catching up, you can unjail your validator:

```bash
secretcli config chain-id secret-3
secretcli tx slashing unjail --from "$YOUR_KEY_NAME" --gas-prices 0.25uscrt
```

You’re now a validator in `secret-3`! :tada:

To make sure your validator is unjailed, look for it in here:

```bash
secretcli q staking validators | jq -r '.[] | select(.status == 2) | .description.moniker'
```

# In case of an upgrade failure

If after a few hours the Enigma team announces on the chat that the upgrade failed, we will relaunch `secret-2`.

1. On the old machine (`secret-2`):

   ```bash
   perl -i -pe 's/^halt-time =.*/halt-time = 0/' ~/.secretd/config/app.toml

   sudo systemctl restart secret-node
   ```

2. Wait for 67% of voting power to come back online.

# Removing an installation

You can remove previous `secretnetwork` installations and start fresh using:

```bash
cd ~
sudo systemctl stop secret-node
secretd unsafe-reset-all
sudo apt purge -y secretnetwork
rm -rf ~/.secretd/*
```

## Appendix: Registration on a new Secret-3 node

```bash
cd ~

rm ~/.secretd/config/genesis.json

secretd init <MONIKER> --chain-id secret-3

wget -O ~/.secretd/config/genesis.json "https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.5/genesis.json"

echo "1c5682a609369c37e2ca10708fe28d78011c2006045a448cdb4e833ef160bf3f .secretd/config/genesis.json" | sha256sum --check

secretd init-enclave # Can be skipped if you're installing secret-3 on your secret-2 machine

PUBLIC_KEY=$(secretd parse attestation_cert.der 2> /dev/null | cut -c 3-)
echo $PUBLIC_KEY

secretcli config chain-id secret-3
secretcli config node http://20.51.225.193:26657
secretcli config trust-node true
secretcli config output json
secretcli config indent true

secretcli tx register auth ./attestation_cert.der --from "$YOUR_KEY_NAME" --gas 250000 --gas-prices 0.25uscrt # Can be skipped if you're installing secret-3 on your secret-2 machine

SEED=$(secretcli query register seed "$PUBLIC_KEY" | cut -c 3-) # Can be skipped if you're installing secret-3 on your secret-2 machine
echo $SEED # Can be skipped if you're installing secret-3 on your secret-2 machine

secretcli query register secret-network-params # Can be skipped if you're installing secret-3 on your secret-2 machine

mkdir -p ~/.secretd/.node

secretd configure-secret node-master-cert.der "$SEED" # Can be skipped if you're installing secret-3 on your secret-2 machine

perl -i -pe 's/persistent_peers =.*/persistent_peers = "27db2f21cfcbfa40705d5c516858f51d5af07e03\@20.51.225.193:26656"/' ~/.secretd/config/config.toml

sudo systemctl enable secret-node

sudo systemctl start secret-node # (Now your new node is live and catching up)

secretcli config node tcp://localhost:26657
```
