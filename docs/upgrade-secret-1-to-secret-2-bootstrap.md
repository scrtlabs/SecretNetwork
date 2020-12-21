# Bootstrap Validator for `secret-2`

- The bootstrap validator must be running [`v0.2.2`](https://github.com/enigmampc/SecretNetwork/releases/tag/v0.2.2).
- This document pairs with the instructions for all other validators: [Network Upgrade Instructions from `secret-1` to `secret-2`](upgrade-secret-1-to-secret-2.md).

1. Export state on the old machine

   ```bash
   HALT_HEIGHT=1246400

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
4. Export the self-delegator wallet from the old machine and import to the new SGX machine (Note that if you're recovering using `secretcli keys add $NAME --recover` you should also add `--hd-path "44'/118'/0'/0/0"`)
5. Copy `genesis_base.json` from the old to `~/.secretd/config/genesis.json` on the new machine
6. `secretd validate-genesis`
7. `secretd init-bootstrap`
8. `secretd validate-genesis`
9. `secretd start --bootstrap`
10. Publish `~/.secretd/config/genesis.json` (now contains initialized `register` state)
