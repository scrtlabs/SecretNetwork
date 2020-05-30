#!/bin/bash

secretcli config chain-id secret-testnet # now we won't need to type --chain-id secret-testnet every time
secretcli config output json
secretcli config indent true
secretcli config trust-node true # true if you trust the full-node you are connecting to, false otherwise

secretd init banana --chain-id secret-testnet # banana==moniker==user-agent of this node
perl -i -pe 's/"stake"/"uscrt"/g' ~/.secretd/config/genesis.json # change the default staking denom from stake to uscrt

secretcli keys add a --keyring-backend test
secretcli keys add b --keyring-backend test

secretd add-genesis-account $(secretcli keys show -a a --keyring-backend test) 1000000000000uscrt # 1 SCRT == 10^6 uSCRT
secretd add-genesis-account $(secretcli keys show -a b --keyring-backend test) 2000000000000uscrt # 1 SCRT == 10^6 uSCRT

secretd validate-genesis # make sure genesis file is correct

# `secretd export` to send genesis.json to validators

secretd gentx --name a --amount 1000000uscrt --keyring-backend test # generate a genesis transaction - this makes a a validator on genesis which stakes 1000000uscrt (1 SCRT)

secretd collect-gentxs # input the genTx into the genesis file, so that the chain is aware of the validators

secretd validate-genesis # make sure genesis file is correct

# `secretd export` to send genesis.json to validators

secretd start --pruning nothing # starts a node
