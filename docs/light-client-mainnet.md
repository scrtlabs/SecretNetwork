# Install the `secretcli` Secret Network light client

1. Get the latest release of `secretcli` for your OS: https://github.com/enigmampc/SecretNetwork/releases/latest.

   ([How to verify releases](/verify-releases.md))

2) Install:

   - Mac/Windows: Rename it from `secretcli-${VERSION}-${OS}` to `secretcli` or `secretcli.exe` and put it in your path.
   - Ubuntu/Debian: `sudo dpkg -i secret*.deb`

3) Configure:

   ```bash
   # Set the mainnet chain-id
   secretcli config chain-id secret-1
   ```

   ```bash
   secretcli config output json
   ```

   ```bash
   secretcli config indent true
   ```

   ```bash
   # Set the full node address
   secretcli config node tcp://bootstrap.mainnet.enigma.co:26657
   ```

   ```bash
   # Verify everything you receive from the full node
   secretcli config trust-node false
   ```

4) Check the installation:

   ```bash
   secretcli status
   ```