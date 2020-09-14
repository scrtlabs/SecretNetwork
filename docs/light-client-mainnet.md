# Install the `secretcli` Secret Network light client

1. Get the latest release of `secretcli` for your OS: https://github.com/enigmampc/SecretNetwork/releases/latest.

   ([How to verify releases](verify-releases.md))

2) Install:

   - Mac/Windows: Rename it from `secretcli-${VERSION}-${OS}` to `secretcli` or `secretcli.exe` and put it in your path.
   - Ubuntu/Debian: `sudo dpkg -i secret*.deb`

3) Configure:

   ```bash
   secretcli config chain-id secret-1
   ```

   ```bash
   secretcli config output json
   ```

   ```bash
   secretcli config indent true
   ```

   ```bash
   secretcli config node tcp://bootstrap.mainnet.enigma.co:26657
   ```

   ```bash
   secretcli config trust-node true
   ```

4) Check the installation:

   ```bash
   secretcli status
   ```
