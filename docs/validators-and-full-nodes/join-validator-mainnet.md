# Join as a Validator

### How to become a validator on Secret Network

#### 1. [Run a new full node](run-full-node-mainnet.md) on a new machine.

#### 2. Set your `minimum-gas-price` parameter

We recommend starting with `0.25uscrt` per gas unit:

```bash
perl -i -pe 's/^minimum-gas-prices = .+?$/minimum-gas-prices = "0.25uscrt"/' ~/.secretd/config/app.toml
sudo systemctl restart secret-node
```

Your validator will not accept transactions that specify `--gas-price` lower than the `minimun-gas-price` you set here.

#### 3. Generate a new key pair for yourself (change `<key-alias>` with any word of your choice, this is just for your internal/personal reference):

```bash
secretcli keys add <key-alias>
```

**:warning:Note:warning:: Backup the mnemonics!**
**:warning:Note:warning:: Please make sure you also [backup your validator](backup-a-validator.md)**

**Note**: If you already have a key you can import it with the bip39 mnemonic with `secretcli keys add <key-alias> --recover` or with `secretcli keys export` (exports to `stderr`!!) & `secretcli keys import`.

#### 4. Transfer tokens to your delegator's address:

This is the `secret` wallet from which you delegate your funds to you own validator. You must delegate at least 1 SCRT (1000000uscrt) from this wallet to your validator.

To create a `secret` wallet, run:

```bash
secretcli keys add <key-alias>
```

Make sure to backup the mnemonic you got from the above command!

Then transfer funds to address you just created.

#### 5. Check that you have the funds:

```bash
secretcli q account $(secretcli keys show -a <key-alias>)
```

If you get the following message, it means that you have no tokens yet:

```bash
ERROR: unknown address: account secret1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx does not exist
```

#### 6. Join the network as a new validator: replace `<MONIKER>` with the moniker you configured in step 3 of [creating a full-node](https://github.com/enigmampc/SecretNetwork/blob/develop/docs/validators-and-full-nodes/run-full-node-mainnet.md), and adjust the amount you want to stake

(remember 1 SCRT = 1,000,000 uSCRT, and so the command below stakes 100k SCRT).

```bash
secretcli tx staking create-validator \
  --amount=<amount-to-delegate-to-yourself>uscrt \
  --pubkey=$(secretd tendermint show-validator) \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --min-self-delegation="1" \
  --gas-prices 0.25uscrt \
  --moniker=<MONIKER> \
  --from=<key-alias>
```

#### 7. Check that you have been added as a validator:

```bash
secretcli q staking validators | jq '.[] | select(.description.moniker == "<MONIKER>")'
```

Or run: `secretcli q staking validators | grep moniker`. You should see your moniker listed.

### Dangers in running a validator

There are a couple of scenarios that can lead to losing a precentage of your and your delegators' stake. These are called slashing events.

The following is updated as of March 23, 2020.

#### Slashing for downtime

Conditions for downtime:

- Signing less than 2500 blocks out of every 5000-block window. For a block time of 5.8 seconds, this roughly translates to being up for 4 hours out of every 8-hour window.

Penalties for downtime:

- Slashing of 1% of your and your delegators' staking amount.
- Jailing for 10 minutes of your validator node. You don't earn block rewards for this period and at the end must manually unjail your node with `secretcli tx slashing unjail --from <key-alias> --gas-prices 0.25uscrt`.

#### Slashing for double-signing

Conditions for double-signing:

- Your validator signs the same block height twice.

Penalties for double-signing:

- Slashing of 5% of your and your delegators' staking amount.
- Jailing forever (tombstoned) of your validator node. You cannot earn block rewards anymore with this validator and you and your delegators must redelegate your stake to a different validator.

### Protecting your validator agains DDoS attacks

See [Sentry Nodes](sentry-nodes.md).

### Staking more tokens

(remember 1 SCRT = 1,000,000 uSCRT)

In order to stake more tokens beyond those in the initial transaction, run:

```bash
secretcli tx staking delegate $(secretcli keys show <key-alias> --bech=val -a) <amount>uscrt --from <key-alias> --gas-prices 0.25uscrt
```

### Editing your Validator

```bash
secretcli tx staking edit-validator
  --moniker="<new-moniker>" \
  --website="https://scrt.network" \
  --identity=6A0D65E29A4CBC8E \
  --details="To infinity and beyond!" \
  --chain-id=<chain_id> \
  --gas="auto" \
  --gas-prices="0.025uscrt" \
  --from=<key_name> \
  --commission-rate="0.10"
```

### Seeing your rewards from being a validator

```bash
secretcli q distribution rewards $(secretcli keys show -a <key-alias>)
```

### Seeing your commissions from your delegators

```bash
secretcli q distribution commission $(secretcli keys show -a <key-alias> --bech=val)
```

### Withdrawing rewards

```bash
secretcli tx distribution withdraw-rewards $(secretcli keys show --bech=val -a <key-alias>) --from <key-alias> --gas-prices 0.25uscrt
```

### Withdrawing rewards+commissions

```bash
secretcli tx distribution withdraw-rewards $(secretcli keys show --bech=val -a <key-alias>) --from <key-alias> --commission --gas-prices 0.25uscrt
```

### Removing your validator

Currently deleting a validator is not possible. If you redelegate or unbond your self-delegations then your validator will become offline and all your delegators will start to unbond.

### Changing your validator's commission-rate

You are currently unable to modify the `--commission-max-rate` and `--commission-max-change-rate"` parameters.

Modifying the commision-rate can be done using this:

```
secretcli tx staking edit-validator --commission-rate="0.05" --from <key-alias> --gas-prices 0.25uscrt
```

### Slashing

##### Unjailing

To unjail your jailed validator

```bash
secretcli tx slashing unjail --from <key-alias> --gas-prices 0.25uscrt
```

##### Signing Info

To retrieve a validator's signing info:

```bash
secretcli q slashing signing-info <validator-conspub-key>
```

##### Query Parameters

You can get the current slashing parameters via:

```bash
secretcli q slashing params
```

##### Query Parameters

You can get the current slashing parameters via:

```bash
secretcli q slashing params
```
