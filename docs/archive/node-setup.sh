enigmagozd init val6 --chain-id goz-enigma
cp ~/genesis.json ~/.enigmagozd/config
enigmagozd validate-genesis
perl -i -pe 's/persistent_peers = ""/persistent_peers = "80d53b83d370d2f121df235f1bd44e3b53e1bccc\@val1.goz.enigma.co:26656"/' ~/.enigmagozd/config/config.toml
perl -i -pe 's/laddr = .+?26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' ~/.enigmagozd/config/config.toml
sudo systemctl enable enigma-node
sudo systemctl start enigma-node
journalctl -f -u enigma-node

enigmagozcli config keyring-backend test
enigmagozcli config chain-id enigma-goz
enigmagozcli keys add val --recover
enigmagozcli tx staking create-validator \
  --amount=99000000000ugozscrt \
  --pubkey=$(enigmagozd tendermint show-validator) \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --min-self-delegation="1" \
  --gas=200000 \
  --gas-prices="0.025ugozscrt" \
  --moniker=val6 \
  --from=val

enigmagozcli q staking validators | grep moniker