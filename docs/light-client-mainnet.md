# Install the `secretcli` Secret Network light client

1. Get the latest release of `secretcli` for your OS: https://github.com/enigmampc/SecretNetwork/releases/latest.

2) Install:

   - Mac/Windows: Rename it from `secretcli-${VERSION}-${OS}` to `secretcli` or `secretcli.exe` and put it in your path.
   - Ubuntu/Debian: `sudo dpkg -i secret*.deb`

3) Configure:

   ```bash
   secretcli config chain-id secret-2
   secretcli config output json
   secretcli config indent true
   secretcli config node tcp://secret-2.node.enigma.co:26657
   secretcli config trust-node true
   ```

   `secret-2.node.scrt.network` is not a real node though.
   You currently have two options for getting your own secret node:
   1. [Rent or use a free-tier node from figment](https://figment.io/datahub/secret-network/).
   2. [Set up your own node](validators-and-full-nodes/run-full-node-mainnet.md).

4) Check the installation:

   ```bash
   secretcli status
   ```
