# How to join the Enigma Blockhain as a mainnet validator

### 1. [Run a new full node](/docs/validators-and-full-nodes/run-full-node-mainnet.md) on a new machine.

### 2. Generate a new key pair for yourself (change `<key-alias>` with any word of your choice, this is just for your internal/personal reference):

```bash
enigmacli keys add <key-alias>
```

**:warning:Note:warning:: Backup the mnemonics!**
**:warning:Note:warning:: Please make sure you also [backup your validator](/docs/validators-and-full-nodes/backup-a-validator.md)**

**Note**: If you already have a key you can import it with the bip39 mnemonic with `enigmacli keys add <key-alias> --recover` or with `enigmacli keys export` (exports to `stderr`!!) & `enigmacli keys import`.

### 3. Output your node address:

```bash
enigmacli keys show <key-alias> -a
```

### 4. Transfer tokens to the address displayed above.

### 5. Check that you have the requested tokens:

```bash
enigmacli q account $(enigmacli keys show -a <key_alias>)
```

If you get the following message, it means that you have no tokens yet:

```bash
ERROR: unknown address: account enigmaxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx does not exist
```

### 6. Join the network as a new validator: replace `<MONIKER>` with your own from step 3 above, and adjust the amount you want to stake

(remember 1 SCRT = 1,000,000 uSCRT, and so the command below stakes 100k SCRT).

```bash
enigmacli tx staking create-validator \
  --amount=100000000000uscrt \
  --pubkey=$(enigmad tendermint show-validator) \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --min-self-delegation="1" \
  --gas=200000 \
  --gas-prices="0.025uscrt" \
  --moniker=<MONIKER> \
  --from=<key-alias>
```

### 7. Check that you have been added as a validator:

```bash
enigmacli q staking validators
```

If the above is too verbose, just run: `enigmacli q staking validators | grep moniker`. You should see your moniker listed.

## Staking more tokens

(remember 1 SCRT = 1,000,000 uSCRT)

In order to stake more tokens beyond those in the initial transaction, run:

```bash
enigmacli tx staking delegate $(enigmacli keys show <key-alias> --bech=val -a) <amount>uscrt --from <key-alias>
```

## Renaming your moniker

```bash
enigmacli tx staking edit-validator --moniker <new-moniker> --from <key-alias>
```

## Seeing your rewards from being a validator

```bash
enigmacli q distribution rewards $(enigmacli keys show -a <key-alias>)
```

## Seeing your commissions from your delegators

```bash
enigmacli q distribution commission $(enigmacli keys show -a <key-alias> --bech=val)
```

## Withdrawing rewards

```bash
enigmacli tx distribution withdraw-rewards $(enigmacli keys show --bech=val -a <key-alias>) --from <key-alias>
```

## Withdrawing rewards+commissions

```bash
enigmacli tx distribution withdraw-rewards $(enigmacli keys show --bech=val -a <key-alias>) --from <key-alias> --commission
```
