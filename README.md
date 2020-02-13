# Enigma Chain

- [How to join EnigmaChain as a mainnet genesis validator](docs/genesis-validator-mainnet.md)
- [How to join EnigmaChain as a testnet validator](docs/validator-testnet.md)
- [Ledger support in EnigmaChain](docs/Ledger.md)

# Install

**Requirement**: Go version needs to be [1.13 or higher](https://golang.org/dl/).

```bash
git clone https://github.com/enigmampc/enigmachain
cd enigmachain
go mod tidy
make install # installs enigmad and enigmacli
```

# Developers Quick Start

```bash
enigmacli config chain-id enigma-testnet # now we won't need to type --chain-id enigma-testnet every time
enigmacli config output json
enigmacli config indent true
enigmacli config trust-node true # true if you trust the full-node you are connecting to, false otherwise

enigmad init banana --chain-id enigma-testnet # banana==moniker==user-agent of this node
perl -i -pe 's/"stake"/"uscrt"/g' ~/.enigmad/config/genesis.json # change the default staking denom from stake to uscrt

enigmacli keys add a
enigmacli keys add b

enigmad add-genesis-account $(enigmacli keys show -a a) 1000000000000uscrt # 1 SCRT == 10^6 uSCRT
enigmad add-genesis-account $(enigmacli keys show -a b) 2000000000000uscrt # 1 SCRT == 10^6 uSCRT

enigmad validate-genesis # make sure genesis file is correct

# `enigmad export` to send genesis.json to validators

enigmad gentx --name a --amount 1000000uscrt # generate a genesis transaction - this makes a a validator on genesis which stakes 1000000uscrt (1 SCRT)

enigmad collect-gentxs # input the genTx into the genesis file, so that the chain is aware of the validators

enigmad validate-genesis # make sure genesis file is correct

# `enigmad export` to send genesis.json to validators

enigmad start --pruning nothing # starts a node
```

# Delegation & Rewards

## `b` is a delegator of `a`

Now `a` is a validator with 1 SCRT (1000000uscrt) staked.  
This is how `b` can delegate 0.00001 SCRT to `a`:

```bash
enigmacli tx staking delegate $(enigmacli keys show a --bech=val -a) 10uscrt --from b
```

This is how to see `b`'s rewards from delegating to `a`:

```bash
enigmacli q distribution rewards $(enigmacli keys show -a b)
```

This is how `b` can withdraw its rewards:

```bash
enigmacli tx distribution withdraw-rewards $(enigmacli keys show --bech=val -a a) --from b
```

## `a` is a validator and has `b` as a delegator

`a` was set up as a validator from genesis.  
This is how to see `a`'s rewards from being a validator:

```bash
enigmacli q distribution rewards $(enigmacli keys show -a a)
```

This is how to see `a`'s commissions from being a validator:

```bash
enigmacli q distribution commission $(enigmacli keys show -a --bech=val a)
```

This is how `a` can withdraw its rewards + its commissions from being a validator:

```bash
enigmacli tx distribution withdraw-rewards $(enigmacli keys show --bech=val -a a) --from a --commission
```

(To withdraw only rewards omit the `--commission`)

# Run your own node (after genesis)

First, init your environment:

```bash
enigmad init [moniker] --chain-id enigma-testnet
```

Now you need a valid running node to send you their `genesis.json` file (usually at `~/.enigmad/config/genesis.json`).  
Once you have the valid `genesis.json`, put it in `~/.enigmad/config/genesis.json` (overwrite the existing file if needed).  
Next, edit your `~/.enigmad/config/config.toml`, set the `persistent_peers`:

```bash
persistent_peers = "[id]@[peer_node_ip]:26656" # `id` can be aquired from your first peer by running `enigmacli status`
```

That't it! Once you're done, just run:

```bash
enigmad start --pruning nothing
```

You will see you local bloackchain replica starting to catch up with your peer's one.

Congrats, you are now up and running!

**Note:** you can also run `enigmad start --pruning nothing --p2p.persistent_peers [id]@[peer_node_ip]:26656` instead of editing the conf file.  
**Note**: If anything goes wrong, delete the `~/.enigmad` and `~/.enigmacli` dirs and start again.

# Join as a new Validator

After you have a private node up and running, run the following command:

```bash
enigmacli tx staking create-validator \
  --amount=<num of coins> \ # This is the amount of coins you put at stake. i.e. 100000uscrt
  --pubkey=$(enigmad tendermint show-validator) \
  --moniker="<name-of-your-moniker>" \
  --chain-id=<chain-id> \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --min-self-delegation="1" \
  --gas="auto" \
  --gas-prices="0.025uscrt" \
  --from=<name or address> # Name or address of your existing account
```

To check if you got added to the validator-set by running:

```bash
enigmacli q tendermint-validator-set
```
