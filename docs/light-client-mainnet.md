# Install the `scrtcli` Secret Blockchain light client

1. Get the latest release of `scrtcli` for your OS: https://github.com/enigmampc/EnigmaBlockchain/releases/latest.

   ([How to verify releases](/docs/verify-releases.md))

2) Install:

   - Mac/Windows: Rename it from `scrtcli-${VERSION}-${OS}` to `scrtcli` or `scrtcli.exe` and put it in your path.
   - Ubuntu/Debian: `sudo dpkg -i enigma*.deb`

3) Configure:

   ```bash
   # Set the mainnet chain-id
   scrtcli config chain-id enigma-1
   ```

   ```bash
   scrtcli config output json
   ```

   ```bash
   scrtcli config indent true
   ```

   ```bash
   # Set the full node address
   scrtcli config node tcp://bootstrap.mainnet.enigma.co:26657
   ```

   ```bash
   # Verify everything you receive from the full node
   scrtcli config trust-node false
   ```

4) Check the installation:

   ```bash
   scrtcli status
   ```
