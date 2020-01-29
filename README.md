# Enigmachain

```bash
git clone https://github.com/enigmampc/Enigmachain
cd Enigmachain
go mod tidy
make install # installs engd and engcli
```

```bash
engd init assaf --chain-id enigma0 # assaf==moniker==user-agent?

engcli keys add a
engcli keys add b

engd add-genesis-account $(engcli keys show a -a) 1000eng,100000000stake
engd add-genesis-account $(engcli keys show b -a) 2000eng,200000000stake

engcli config chain-id enigma0 # now we won't need to type --chain-id enigma0 every time
engcli config output json
engcli config indent true
engcli config trust-node true # Set to true if you trust the full-node you are connecting to, false otherwise

engd gentx --name a # generate a genesis transaction

engd collect-gentxs # input the genTx into the genesis file, so that the chain is aware of the validators

engd validate-genesis # make sure genesis file is correct

engd start # hokos pokos
```
