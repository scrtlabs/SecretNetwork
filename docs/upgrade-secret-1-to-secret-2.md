# :warning: WIP :warning:

- [:warning: WIP :warning:](#warning-wip-warning)
  - [Validators](#validators)
    - [In case of an upgrade failure](#in-case-of-an-upgrade-failure)
  - [Bootstrap validator](#bootstrap-validator)

## Validators

All coordination efforts will be done in the [#mainnet-validators](https://chat.scrt.network/channel/mainnet-validators) channel in the Secret Network Rocket.Chat.

1. On the old machine (`secret-1`):

   ```bash
   perl -i -pe 's/^halt-height =.*/halt-height = 1246400/' ~/.secretd/config/app.toml
   ```

   ```bash
   sudo systemctl restart secret-node.service
   ```

   Note: Although halt height is 1246400 on `secret-1`, the halt time might not be exactly September 15th, 2020 at 14:00:00 UTC. The halt height was calculated on September 7th to be as close a possible to September 15th, 2020 at 14:00:00 UTC, using `secret-1` block time of 6.19 seconds.

2. Install `secretnetwork_1.0.0_amd64.deb` on the new SGX machine
3. Copy `~/.secretd/config/priv_validator_key.json` to the new SGX machine
4. Export the self-delegator wallet from the old machine and import to the new SGX machine (Note that if you're recovering using `secretcli keys add $NAME --recover` you should also add `--hd-path "44'/118'/0'/0/0"`)
5. `secretd init $MONIKER --chain-id secret-2`
6. `wget -O ~/.secretd/config/genesis.json "https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.0/genesis.json"`
7. `secretd validate-genesis`
8. `cd ~`
9. `secretd init-enclave`
10. `PUBLIC_KEY=$(secretd parse attestation_cert.der 2> /dev/null | cut -c 3-)`
11. Configure `secretcli`:

    ```bash
    secretcli config chain-id secret-2
    secretcli config node tcp://TODO:26657
    secretcli config trust-node true
    secretcli config output json
    secretcli config indent true
    ```

12. `secretcli tx register auth ./attestation_cert.der --from $YOUR_KEY_NAME --gas 250000 --gas-prices TODOuscrt`
13. `SEED=$(secretcli query register seed "$PUBLIC_KEY" | cut -c 3-)`
14. `secretcli query register secret-network-params`
15. `mkdir -p ~/.secretd/.node`
16. `secretd configure-secret node-master-cert.der "$SEED"`
17. `perl -i -pe 's/persistent_peers = ""/persistent_peers = "TODO\@TODO:26656"/' ~/.secretd/config/config.toml`
18. `perl -i -pe 's/laddr = .+?26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' ~/.secretd/config/config.toml`
19. `sudo systemctl enable secret-node`
20. `sudo systemctl start secret-node` (Now your new node is up)
21. `secretcli config node tcp://localhost:26657`
22. Wait until you're done catching up: `watch 'secretcli status | jq ".sync_info.catching_up == false"'` (This should output `true`)
23. `secretcli tx slashing unjail --from $YOUR_KEY_NAME --gas-prices TODOuscrt` :tada:

([Ref](testnet/run-full-node-testnet.md))

### In case of an upgrade failure

TODO: Define what counts as a network upgrade failure.

1. On the old machine (`secret-1`):

   ```bash
   perl -i -pe 's/^halt-height =.*/halt-height = 0/' ~/.secretd/config/app.toml
   ```

   ```bash
   sudo systemctl restart secret-node.service
   ```

2. Wait for 67% of voting power to come back online.

## Bootstrap validator

Must be running [`v0.2.2`](https://github.com/enigmampc/SecretNetwork/releases/tag/v0.2.2).

1. Export state on the old machine

   ```bash
   export HALT_HEIGHT=1246400

   perl -i -pe "s/^halt-height =.*/halt-height = $HALT_HEIGHT/" ~/.secretd/config/app.toml
   sudo systemctl restart secret-node.service

   # Wait for $HALT_HEIGHT...

   secretd export --height $HALT_HEIGHT --for-zero-height --jail-whitelist secretvaloper13l72vhjngmg55ykajxdnlalktwglyqjqaz0tdu |
       jq -Sc -f <(
           echo '.chain_id = "secret-2" |'
           echo '.genesis_time = (now | todate) |'
           echo '.consensus_params.block.max_gas = "10000000" |'
           echo '.app_state.distribution.params.secret_foundation_tax = "0.15" |'
           echo '.app_state.distribution.params.secret_foundation_address = "secret1c7rjffp9clkvrzul20yy60yhy6arnv7sde0kjj" |'
           echo '.app_state.register = { "reg_info": null, "node_exch_cert": null, "io_exch_cert": null } |'
           echo '.app_state.compute = { "codes": null, "contracts": null }'
       ) > genesis_base.json
   ```

2. Install `secretnetwork_1.0.0_amd64.deb` on the new SGX machine
3. Copy `~/.secretd/config/priv_validator_key.json` to the new SGX machine
4. Export the self-delegator wallet from the old machine and import to the new SGX machine
5. Copy `genesis_base.json` from the old to `~/.secretd/config/genesis.json` on the new machine
6. `secretd validate-genesis`
7. `secretd init-bootstrap`
8. `secretd validate-genesis`
9. `secretd start --bootstrap`
10. Publish `~/.secretd/config/genesis.json` (now contains initialized `register` state)
