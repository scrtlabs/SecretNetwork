#!/bin/bash

enigmacli config chain-id enigma-testnet # now we won't need to type --chain-id enigma-testnet every time
enigmacli config output json
enigmacli config indent true
enigmacli config trust-node true # true if you trust the full-node you are connecting to, false otherwise

enigmad init banana --chain-id enigma-testnet # banana==moniker==user-agent of this node
perl -i -pe 's/"stake"/"uscrt"/g' ~/.enigmad/config/genesis.json # change the default staking denom from stake to uscrt

enigmacli keys add a --keyring-backend test
enigmacli keys add b --keyring-backend test

enigmad add-genesis-account $(enigmacli keys show -a a --keyring-backend test) 1000000000000uscrt # 1 SCRT == 10^6 uSCRT
enigmad add-genesis-account $(enigmacli keys show -a b --keyring-backend test) 2000000000000uscrt # 1 SCRT == 10^6 uSCRT

enigmad validate-genesis # make sure genesis file is correct

# `enigmad export` to send genesis.json to validators

enigmad gentx --name a --amount 1000000uscrt --keyring-backend test # generate a genesis transaction - this makes a a validator on genesis which stakes 1000000uscrt (1 SCRT)

enigmad collect-gentxs # input the genTx into the genesis file, so that the chain is aware of the validators

enigmad validate-genesis # make sure genesis file is correct

# `enigmad export` to send genesis.json to validators

enigmad start --pruning nothing # starts a node