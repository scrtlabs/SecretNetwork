# Enigmachain

## Install

```bash
git clone https://github.com/enigmampc/Enigmachain
cd Enigmachain
go mod tidy
make install # installs engd and engcli
```

## Quick Start

```bash
engcli config chain-id enigma0 # now we won't need to type --chain-id enigma0 every time
engcli config output json
engcli config indent true
engcli config trust-node true # true if you trust the full-node you are connecting to, false otherwise

engd init banana --chain-id enigma0 # banana==moniker==user-agent of my node?

echo 12345678 | engcli keys add a
echo 12345678 | engcli keys add b

engd add-genesis-account $(engcli keys show -a a) 1000000000000ueng # 1 ENG == 10^6 uENG
engd add-genesis-account $(engcli keys show -a b) 2000000000000ueng # 1 ENG == 10^6 uENG

echo 12345678 | engd gentx --name a --amount 1000000ueng # generate a genesis transaction - this makes a a validator on genesis which stakes 1000000ueng ()

engd collect-gentxs # input the genTx into the genesis file, so that the chain is aware of the validators

engd validate-genesis # make sure genesis file is correct

engd start # hokos pokos
```

```bash
# Now a is a validator with 1 ENG (1000000ueng) staked.
# This is how b can delegate 0.00001 ENG to a
engcli tx staking delegate $(engcli keys show a --bech=val -a) 10ueng --from b
```

## Run your private node (on a running chain)

First, init your environment:
```bash
endg init [moniker] --chain-id enigma0
```

Now you need a valid running node to send you their `genesis.json` file (usually at `~/.engd/config/genesis.json`)
Once you have the valid `genesis.json`, put it in `~/.engd/config/genesis.json` (overwrite the existing file if needed).
Next, edit your `.engd/config/config.toml`, set the `persistent_peers`:
```bash
persistent_peers = "[id]@[peer_node_ip]:26656" # `id` can be aquired from your peer by running `engcli status`
```

That't it! Once you're done, just run:
```bash
engd start
```
You will see you local bloackchain replica starting to catch up with your peer's one.

Congrats, you are now up and running!

***Note**: If anything goes wrong, delete the `~/.engd` and `~/.engcli` dirs and start again.*

## Join as a Validator

After you have a private up and running, run the following command:

``` bash
engcli tx staking create-validator \
  --amount=<num of coins> \ # This is the amount of coins you put at stake. i.e. 100000ueng
  --pubkey=$(engd tendermint show-validator) \
  --moniker="<name-of-your-moniker>" \
  --chain-id=<chain-id> \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --min-self-delegation="1" \
  --gas="auto" \
  --gas-prices="0.025ueng" \
  --from=<name or address> # Name or address of your existing account
```

To check if you got added to the validator-set by running:
```bash
engcli q tendermint-validator-set
```