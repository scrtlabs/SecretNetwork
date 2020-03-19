# Install the `secretcli` Secret Blockchain light client

1. Get the latest release of `secretcli` for your OS: https://github.com/enigmampc/EnigmaBlockchain/releases/latest.

   ([How to verify releases](/docs/verify-releases.md))

2) Install:

   - Mac/Windows: Rename it from `secretcli-${VERSION}-${OS}` to `secretcli` or `secretcli.exe` and put it in your path.
   - Ubuntu/Debian: `sudo dpkg -i enigma*.deb`

3) Configure:

   ```shell
   # Set the mainnet chain-id
   secretcli config chain-id enigma-1
   ```

   ```shell
   enigmacli config output json
   ```

   ```shell
   enigmacli config indent true
   ```

   ```shell
   # Set the full node address
   secretcli config node tcp://bootstrap.mainnet.enigma.co:26657
   ```

   ```shell
   # Verify everything you receive from the full node
   secretcli config trust-node false
   ```

4) Check the installation:

   ```shell
   enigmacli status
   ```
