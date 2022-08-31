## Secret IBC setup
Two local secrets can Inter-blockchainly communicate with each other via a Hermes relayer

### Build
```bash
docker build -f hermes.Dockerfile . --tag hermes:test
```

### Run
```bash
docker compose up
```

### Verify IBC transfers
Assuming you have a key 'a' which is not the relayer's key,
from localhost:
```bash
# be on the source network (secretdev-1)
secretcli config node http://localhost:26657

# check the initial balance of a
secretcli q bank balances <a-address>

# transfer to the destination network
secretcli tx ibc-transfer transfer transfer channel-0 secret1fc3fzy78ttp0lwuujw7e52rhspxn8uj52zfyne 2uscrt --from a

# check a's balance after transfer
scli q bank balances <a-address>

# switch to the destination network (secretdev-2)
secretcli config node http://localhost:36657

# check that you have an ibc-denom
secretcli q bank balances <dst-b> # should have 2 ibc denom
```
