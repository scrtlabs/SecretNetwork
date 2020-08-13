---
title: 'Delegating (CLI)'
---
# Delegating (CLI)

## Delegate to a Validator

On mainnet, you can delegate `uscrt` to a validator. These delegators can receive part of the validator's fee revenue. Read more about the [Cosmos Token Model](https://github.com/cosmos/cosmos/raw/master/Cosmos_Token_Model.pdf).

### Query Validators

You can query the list of all validators of a specific chain:

```bash
secretcli q staking validators
```

If you want to get the information of a single validator you can check it with:

```bash
secretcli q staking validator <validator-address>
```

### Bond Tokens

On the Secret Network mainnet, we delegate `uscrt`, where `1scrt = 1000000uscrt`. Here's how you can bond tokens to a validator (_i.e._ delegate):

```bash
secretcli tx staking delegate \
	<validator-operator-address>
	<amount> \
	--from=<key-alias>
```

Example:

```
secretcli tx staking delegate \
	secretvaloper1l2rsakp388kuv9k8qzq6lrm9taddae7fpx59wm \
	1000uscrt \
	--from <key-alias>
```

`<validator-operator-address>` is the operator address of the validator to which you intend to delegate. If you are running a full node, you can find this with:

```bash
secretcli keys show <key-alias> --bech val
```

where `<key-alias>` is the name of the key you specified when you initialized `secretd`.

While tokens are bonded, they are pooled with all the other bonded tokens in the network. Validators and delegators obtain a percentage of shares that equal their stake in this pool.

### Withdraw Rewards

To withdraw the delegator rewards:

```bash
secretcli tx distribution withdraw-rewards <validator-operator-address> --from <key-alias>
```

### Query Delegations

Once you've submitted a delegation to a validator, you can see it's information by using the following command:

```bash
secretcli q staking delegation <delegator-address> <validator-operator-address>
```

Example:

```bash
secretcli q staking delegation \
	secret1gghjut3ccd8ay0zduzj64hwre2fxs9ld75ru9p \
	secretvaloper1gghjut3ccd8ay0zduzj64hwre2fxs9ldmqhffj
```

Or if you want to check all your current delegations with distinct validators:

```bash
secretcli q staking delegations <delegator-address>
```

### Unbond Tokens

::: danger Please Note
There currently is in place a _21_ days unbonding rule, during which no rewards are handed out.
:::

If for any reason the validator misbehaves, or you just want to unbond a certain
amount of tokens, use this following command.

```bash
secretcli tx staking unbond \
  <validator-address> \
  10uscrt \
  --from=<key-alias> \
  --chain-id=<chain-id>
```

The unbonding will be automatically completed when the unbonding period has passed.

### Query Unbonding-Delegations

Once you begin an unbonding-delegation, you can see it's information by using the following command:

```bash
secretcli q staking unbonding-delegation <delegator-address> <validator-operator-address>
```

Or if you want to check all your current unbonding-delegations with distinct validators:

```bash
secretcli q staking unbonding-delegations <delegator-address>
```

Additionally, you can get all the unbonding-delegations from a particular validator:

```bash
secretcli q staking unbonding-delegations-from <validator-operator-address>
```

### Redelegate Tokens

A redelegation is a type delegation that allows you to bond illiquid tokens from one validator to another:

```bash
secretcli tx staking redelegate \
  <src-validator-operator-address> \
  <dst-validator-operator-address> \
  10uscrt \
  --from=<key-alias> \
  --chain-id=<chain-id>
```

Here you can also redelegate a specific `shares-amount` or a `shares-fraction` with the corresponding flags.

The redelegation will be automatically completed when the unbonding period has passed.

### Query Redelegations

Once you begin an redelegation, you can see it's information by using the following command:

```bash
secretcli q staking redelegation <delegator-address> <src-valoper-address> <dst-valoper-address>
```

Or if you want to check all your current unbonding-delegations with distinct validators:

```bash
secretcli q staking redelegations <delegator-address>
```

Additionally, you can get all the outgoing redelegations from a particular validator:

```bash
  secretcli q staking redelegations-from <validator-operator-address>
```

### Query Parameters

Parameters define high level settings for staking. You can get the current values by using:

```bash
secretcli q staking params
```

With the above command you will get the values for:

- Unbonding time
- Maximum numbers of validators
- Coin denomination for staking

Example:

```bash
$ secretcli q staking params

{

"unbonding_time": "1814400000000000",

"max_validators": 50,

"max_entries": 7,

"historical_entries": 0,

"bond_denom": "uscrt"

}
```

All these values will be subject to updates though a `governance` process by `ParameterChange` proposals.

### Query Pool

A staking `Pool` defines the dynamic parameters of the current state. You can query them with the following command:

```bash
secretcli q staking pool
```

With the `pool` command you will get the values for:

- Not-bonded and bonded tokens
- Token supply
- Current annual inflation and the block in which the last inflation was processed
- Last recorded bonded shares

### Query Delegations To Validator

You can also query all of the delegations to a particular validator:

```bash
  secretcli q staking delegations-to <validator-operator-address>
```

Example:

```bash
$ secretcli q staking delegations-to secretvaloper1gghjut3ccd8ay0zduzj64hwre2fxs9ldmqhffj

```
