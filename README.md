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
