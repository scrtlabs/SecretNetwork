# Install the `enigmacli` Enigma Blockchain light client

1. Get the latest release of `enigmacli` for your OS: https://github.com/enigmampc/EnigmaBlockchain/releases/latest.

   ([How to verify releases](/docs/verify-releases.md))

2) Install:

   - Mac/Windows: Rename it from `enigmacli-${VERSION}-${OS}` to `enigmacli` or `enigmacli.exe` and put it in your path.
   - Ubuntu/Debian: `sudo dpkg -i enigmachain*.deb`

3) Configure:

   ```shell
   # Set the mainnet chain-id
   enigmacli config chain-id enigma-1
   ```

   ```shell
   enigmacli config output json
   ```

   ```shell
   enigmacli config indent true
   ```

   ```shell
   # Set the full node address
   enigmacli config node tcp://bootstrap.mainnet.enigma.co:26657
   ```

   ```shell
   # Verify everything you receive from the full node
   enigmacli config trust-node false
   ```

4) Check the installation:

   ```shell
   enigmacli status
   ```
