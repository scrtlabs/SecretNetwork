secretgozd init val6 --chain-id secret-goz
cp ~/genesis.json ~/.secretgozd/config
secretgozd validate-genesis
perl -i -pe 's/persistent_peers = ""/persistent_peers = "80d53b83d370d2f121df235f1bd44e3b53e1bccc\@val1.goz.enigma.co:26656"/' ~/.secretgozd/config/config.toml
perl -i -pe 's/laddr = .+?26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' ~/.secretgozd/config/config.toml
sudo systemctl enable secretgoz-node
sudo systemctl start secretgoz-node
journalctl -f -u secretgoz-node

secretgozcli config keyring-backend test
secretgozcli config chain-id secret-goz
secretgozcli keys add val --recover
secretgozcli tx staking create-validator \
  --amount=99000000000ugozscrt \
  --pubkey=$(secretgozd tendermint show-validator) \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --min-self-delegation="1" \
  --gas=200000 \
  --gas-prices="0.025ugozscrt" \
  --moniker=val6 \
  --from=val

secretgozcli q staking validators | grep moniker
