# Re-Register a Full Node on Mainnet

These instructions are specifically for re-registering your node, which can be required after a BIOS update, or if you're having issues registering and setting up your node and need to redo those steps.

**Note**: Substitute your key name or alias for `<key_alias>` below.

```bash
secretd init-enclave --reset

PUBLIC_KEY=$(secretd parse attestation_cert.der 2> /dev/null | cut -c 3-)
echo $PUBLIC_KEY

secretcli config node tcp://secret-2.node.enigma.co:26657

YOUR_KEY_NAME=<key_alias>

secretcli tx register auth ./attestation_cert.der --from "$YOUR_KEY_NAME" --gas 250000 --gas-prices 0.25uscrt

SEED=$(secretcli query register seed "$PUBLIC_KEY" | cut -c 3-)
echo $SEED

secretcli query register secret-network-params

mkdir -p ~/.secretd/.node

secretd configure-secret node-master-cert.der "$SEED"

sudo systemctl start secret-node

journaltctl -f -u secret-node

```

Lastly, configure the CLI to point to your local node.

```bash
secretcli config node tcp://localhost:26657

```

And verify your node's status.

```bash
secretcli status
```

