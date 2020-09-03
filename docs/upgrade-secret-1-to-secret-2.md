# :warning: WIP :warning:

## Bootstrap validator

Must be running [`v0.2.2`](https://github.com/enigmampc/SecretNetwork/releases/tag/v0.2.2).

1. Export state on the old machine

   ```bash
   # TODO: update $HEIGHT to be as close as possible to 2020-09-15T14:00:00Z
   # Use this one-liner and play with the block time and block height until
   # you get as closest to 2020-09-15T14:00:00 UTC.
   # We should probably settle on a final $HEIGHT before Sep 14th.
   # curl -s http://bootstrap.mainnet.enigma.co:26657/status | jq -r '6.22*(1246800-(.result.sync_info.latest_block_height | tonumber))' | xargs -I {} date -u -d "now + {} sec"
   export HEIGHT=1246800

   secretd export --height $HEIGHT --for-zero-height --jail-whitelist secretvaloper13l72vhjngmg55ykajxdnlalktwglyqjqaz0tdu |
       jq -Sc -f <(
           echo '.chain_id = "secret-2" |'
           echo '.genesis_time = "2020-09-15T14:00:00Z" |'
           echo '.consensus_params.block.max_gas = "10000000" |'
           echo '.app_state.distribution.params = { "secret_foundation_tax": "0.15", "secret_foundation_address": "secret1TODO" } |'
           echo '.app_state.register = { "reg_info": null, "node_exch_cert": null, "io_exch_cert": null } |'
           echo '.app_state.compute = { "codes": null, "contracts": null }'
       ) > genesis.json
   ```

2. Install `secretnetwork_1.0.0_amd64.deb` on the new SGX machine
3. Copy `~/.secretd/config/priv_validator_key.json` to the new SGX machine
4. Export the self-delegator wallet from the old machine and import to the new SGX machine
5. Copy `genesis.json` from the old to `~/.secretd/config/genesis.json` on the new machine
6. `secretd validate-genesis`
7. `secretd init-bootstrap`
8. `secretd validate-genesis`
9. `secretd start --bootstrap`

## Every other validator

1.  Install `secretnetwork_1.0.0_amd64.deb` on the new SGX machine
2.  Copy `~/.secretd/config/priv_validator_key.json` to the new SGX machine
3.  Export the self-delegator wallet from the old machine and import to the new SGX machine
4.  `secretd init $MONIKER --chain-id secret-2`
5.  `wget -O ~/.secretd/config/genesis.json "https://github.com/enigmampc/SecretNetwork/releases/download/v1.0.0/genesis.json"`
6.  `secretd validate-genesis`
7.  `cd ~`
8.  `secretd init-enclave`
9.  `PUBLIC_KEY=$(secretd parse attestation_cert.der 2> /dev/null | cut -c 3-)`
10. Configure `secretcli`:

    ```bash
    secretcli config chain-id secret-2
    secretcli config node tcp://TODO:26657
    secretcli config output json
    secretcli config indent true
    secretcli config trust-node true
    ```

11. `secretcli tx register auth ./attestation_cert.der --from $YOUR_KEY_NAME --gas 250000 --gas-prices TODOuscrt`
12. `SEED=$(secretcli query register seed "$PUBLIC_KEY" | cut -c 3-)`
13. `secretcli query register secret-network-params`
14. `mkdir -p ~/.secretd/.node`
15. `secretd configure-secret node-master-cert.der "$SEED"`
16. `perl -i -pe 's/persistent_peers = ""/persistent_peers = "TODO\@TODO:26656"/' ~/.secretd/config/config.toml`
17. `perl -i -pe 's/laddr = .+?26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' ~/.secretd/config/config.toml`
18. `sudo systemctl enable secret-node`
19. `sudo systemctl start secret-node` (Now your new node is up)
20. `secretcli config node tcp://localhost:26657`
21. Wait until you're done catching up: `watch 'secretcli status | jq ".sync_info.catching_up == false"'` (This should output `true`)
22. `secretcli tx slashing unjail --from $YOUR_KEY_NAME --gas-prices TODOuscrt` :tada:
23. Profit.

([Ref](testnet/run-full-node-testnet.md))
