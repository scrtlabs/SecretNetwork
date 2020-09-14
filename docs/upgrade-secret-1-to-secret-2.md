- [Validators](#validators)
  - [1. Prepare your `secret-1` validtor to halt after block #1,246,400](#1-prepare-your-secret-1-validtor-to-halt-after-block-1246400)
  - [2. Install the new binaries on your SGX machine](#2-install-the-new-binaries-on-your-sgx-machine)
  - [3. Migrate your validator's signing key](#3-migrate-your-validators-signing-key)
  - [4. Migrate your validator's wallet](#4-migrate-your-validators-wallet)
  - [5. Set up your SGX machine and become a `secret-2` validator](#5-set-up-your-sgx-machine-and-become-a-secret-2-validator)
- [In case of an upgrade failure](#in-case-of-an-upgrade-failure)
- [Bootstrap validator](#bootstrap-validator)

# Validators

All coordination efforts will be done in the [#mainnet-validators](https://chat.scrt.network/channel/mainnet-validators) channel in the Secret Network Rocket.Chat.

:warning: Note that if you're new SGX machine has a previous `secretnetwork` installation on it (e.g. from the testnet), you will need to remove it before you continue:

```bash
cd ~
sudo apt purge -y secretnetwork
rm -rf .sgx_secrets/*
rm -rf .secretcli/*
rm -rf .secretd/*
```

## 1. Prepare your `secret-1` validtor to halt after block #1,246,400

On the old machine (`secret-1`):

```bash
perl -i -pe 's/^halt-height =.*/halt-height = 1246400/' ~/.secretd/config/app.toml

sudo systemctl restart secret-node.service
```

Note: Although halt height is 1246400 on `secret-1`, the halt time might not be exactly September 15th, 2020 at 14:00:00 UTC. The halt height was calculated on September 7th to be as close a possible to September 15th, 2020 at 14:00:00 UTC, using `secret-1` block time of 6.19 seconds.

## 2. Install the new binaries on your SGX machine

On the new SGX machine (`secret-2`):

```bash
cd ~

wget "https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.0/secretnetwork_1.0.0_amd64.deb" # TODO

echo "87aee80a112f429db0b8b6703b1eba33accea0f08af9e65339d14d92a5186b24 secretnetwork_1.0.0_amd64.deb" | sha256sum --check

sudo apt install -y ./secretnetwork_1.0.0_amd64.deb

secretd init $MONIKER
```

## 3. Migrate your validator's signing key

Copy your `~/.secretd/config/priv_validator_key.json` from the old machine (`secret-1`) to the new SGX machine (`secret-2`) at the same location.

## 4. Migrate your validator's wallet

Export the self-delegator wallet from the old machine (`secret-1`) and import to the new SGX machine (`secret-2`).

On the old machine (`secret-1`) use `secretcli export $YOUR_KEY_NAME`.  
On the new SGX machine (`secret-2`) use `secretcli import $YOUR_KEY_NAME $FROM_FILE_NAME`

Note that if you're recovering it using `secretcli keys add $YOUR_KEY_NAME --recover` you should also use `--hd-path "44'/118'/0'/0/0"`.

## 5. Set up your SGX machine and become a `secret-2` validator

On the new SGX machine (`secret-2`):

```bash
cd ~

secretd init $MONIKER --chain-id secret-2

wget -O ~/.secretd/config/genesis.json "https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.0/genesis.json" # TODO

echo "TODO .secretd/config/genesis.json" | sha256sum --check

secretd validate-genesis

secretd init-enclave

PUBLIC_KEY=$(secretd parse attestation_cert.der 2> /dev/null | cut -c 3-)
echo $PUBLIC_KEY

secretcli config chain-id secret-2
secretcli config node tcp://secret-2.node.enigma.co:26657
secretcli config trust-node true
secretcli config output json
secretcli config indent true

secretcli tx register auth ./attestation_cert.der --from $YOUR_KEY_NAME --gas 250000 --gas-prices 0.25uscrt

SEED=$(secretcli query register seed "$PUBLIC_KEY" | cut -c 3-)
echo $SEED

secretcli query register secret-network-params

mkdir -p ~/.secretd/.node

secretd configure-secret node-master-cert.der "$SEED"

perl -i -pe 's/persistent_peers = ""/persistent_peers = "bee0edb320d50c839349224b9be1575ca4e67948\@secret-2.node.enigma.co:26656"/' ~/.secretd/config/config.toml

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
secretcli tx slashing unjail --from $YOUR_KEY_NAME --gas-prices 0.25uscrt
```

You’re now a validator in `secret-2`! :tada:

To make sure your validator is unjailed, look for it in here:

```bash
secretcli q staking validators | jq -r '.[] | select(.status == 2) | .description.moniker'
```

([Ref for testnet instructions](testnet/run-full-node-testnet.md))

# In case of an upgrade failure

If after a few hours the Enigma team announces on the chat that the upgrade failed, we will relaunch `secret-1`.

1. On the old machine (`secret-1`):

   ```bash
   perl -i -pe 's/^halt-height =.*/halt-height = 0/' ~/.secretd/config/app.toml

   sudo systemctl restart secret-node.service
   ```

2. Wait for 67% of voting power to come back online.

# Bootstrap validator

Must be running [`v0.2.2`](https://github.com/enigmampc/SecretNetwork/releases/tag/v0.2.2).

1. Export state on the old machine

   ```bash
   export HALT_HEIGHT=1246400

   perl -i -pe "s/^halt-height =.*/halt-height = $HALT_HEIGHT/" ~/.secretd/config/app.toml
   sudo systemctl restart secret-node.service

   # Wait for $HALT_HEIGHT...

   secretd export --height $HALT_HEIGHT --for-zero-height --jail-whitelist secretvaloper13l72vhjngmg55ykajxdnlalktwglyqjqaz0tdu |
       jq -Sc -f <(
           echo '.chain_id = "secret-2" |'
           echo '.genesis_time = (now | todate) |'
           echo '.consensus_params.block.max_gas = "10000000" |'
           echo '.app_state.distribution.params.secret_foundation_tax = "0.15" |'
           echo '.app_state.distribution.params.secret_foundation_address = "secret1c7rjffp9clkvrzul20yy60yhy6arnv7sde0kjj" |'
           echo '.app_state.register = { "reg_info": null, "node_exch_cert": null, "io_exch_cert": null } |'
           echo '.app_state.compute = { "codes": null, "contracts": null }'
       ) > genesis_base.json
   ```

2. Install `secretnetwork_1.0.0_amd64.deb` on the new SGX machine
3. Copy `~/.secretd/config/priv_validator_key.json` to the new SGX machine
4. Export the self-delegator wallet from the old machine and import to the new SGX machine (Note that if you're recovering using `secretcli keys add $NAME --recover` you should also add `--hd-path "44'/118'/0'/0/0"`)
5. Copy `genesis_base.json` from the old to `~/.secretd/config/genesis.json` on the new machine
6. `secretd validate-genesis`
7. `secretd init-bootstrap`
8. `secretd validate-genesis`
9. `secretd start --bootstrap`
10. Publish `~/.secretd/config/genesis.json` (now contains initialized `register` state)
