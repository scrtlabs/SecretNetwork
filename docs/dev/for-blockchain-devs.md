# Install

**Requirement**: Go version needs to be [1.13 or higher](https://golang.org/dl/).

# Developers Quick Start

## Local installation

```bash
git clone https://github.com/enigmampc/SecretNetwork
cd SecretNetwork
go mod tidy
make install # installs secretd and secretcli
```

```bash
secretcli config chain-id enigma-testnet # now we won't need to type --chain-id enigma-testnet every time
secretcli config output json
secretcli config indent true
secretcli config trust-node true # true if you trust the full-node you are connecting to, false otherwise

secretd init banana --chain-id enigma-testnet # banana==moniker==user-agent of this node
perl -i -pe 's/"stake"/"uscrt"/g' ~/.secretd/config/genesis.json # change the default staking denom from stake to uscrt

secretcli keys add a
secretcli keys add b

secretd add-genesis-account $(secretcli keys show -a a) 1000000000000uscrt # 1 SCRT == 10^6 uSCRT
secretd add-genesis-account $(secretcli keys show -a b) 2000000000000uscrt # 1 SCRT == 10^6 uSCRT

secretd validate-genesis # make sure genesis file is correct

# `secretd export` to send genesis.json to validators

secretd gentx --name a --amount 1000000uscrt # generate a genesis transaction - this makes a a validator on genesis which stakes 1000000uscrt (1 SCRT)

secretd collect-gentxs # input the genTx into the genesis file, so that the chain is aware of the validators

secretd validate-genesis # make sure genesis file is correct

# `secretd export` to send genesis.json to validators

secretd start --pruning nothing # starts a node
```

## Docker installation

```bash
git clone https://github.com/enigmampc/SecretNetwork
cd SecretNetwork
docker build -t secretdev -f Dockerfile_devnet .

docker run -d -p 26657:26657 -p 26656:26656 -p 1317:1317 \
 --name secretdev secretdev
```

# Delegation & Rewards

## `b` is a delegator of `a`

Now `a` is a validator with 1 SCRT (1000000uscrt) staked.  
This is how `b` can delegate 0.00001 SCRT to `a`:

```bash
secretcli tx staking delegate $(secretcli keys show a --bech=val -a) 10uscrt --from b
```

This is how to see `b`'s rewards from delegating to `a`:

```bash
secretcli q distribution rewards $(secretcli keys show -a b)
```

This is how `b` can withdraw its rewards:

```bash
secretcli tx distribution withdraw-rewards $(secretcli keys show --bech=val -a a) --from b
```

## `a` is a validator and has `b` as a delegator

`a` was set up as a validator from genesis.  
This is how to see `a`'s rewards from being a validator:

```bash
secretcli q distribution rewards $(secretcli keys show -a a)
```

This is how to see `a`'s commissions from being a validator:

```bash
secretcli q distribution commission $(secretcli keys show -a --bech=val a)
```

This is how `a` can withdraw its rewards + its commissions from being a validator:

```bash
secretcli tx distribution withdraw-rewards $(secretcli keys show --bech=val -a a) --from a --commission
```

(To withdraw only rewards omit the `--commission`)

# Run a node (after genesis)

First, init your environment:

```bash
secretd init [moniker] --chain-id enigma-testnet
```

Now you need a valid running node to send you their `genesis.json` file (usually at `~/.secretd/config/genesis.json`).  
Once you have the valid `genesis.json`, put it in `~/.secretd/config/genesis.json` (overwrite the existing file if needed).  
Next, edit your `~/.secretd/config/config.toml`, set the `persistent_peers`:

```bash
persistent_peers = "[id]@[peer_node_ip]:26656" # `id` can be aquired from your first peer by running `secretcli status`
```

That's it! Once you're done, just run:

```bash
secretd start --pruning nothing
```

You will see your local blockchain replica starting to catch up with your peer's one.

Congrats, you are now up and running!

**Note:** you can also run `secretd start --pruning nothing --p2p.persistent_peers [id]@[peer_node_ip]:26656` instead of editing the conf file.  
**Note**: If anything goes wrong, delete the `~/.secretd` and `~/.secretcli` dirs and start again.

# Join as a new Validator

After you have a private node up and running, run the following command:

```bash
secretcli tx staking create-validator \
  --amount=<num of coins> \ # This is the amount of coins you put at stake. i.e. 100000uscrt
  --pubkey=$(secretd tendermint show-validator) \
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
secretcli q tendermint-validator-set
```
