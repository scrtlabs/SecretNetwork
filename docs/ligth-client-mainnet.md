# How to use `enigmacli` as an EnigmaChain light client

1. Get the latest release of `enigmacli` for your OS: https://github.com/enigmampc/enigmachain/releases/latest.

   ([How to verify releases](/docs/verify-releases.md))

2) Install:

   - Mac/Windows: Rename it from `enigmacli-${VERSION}-${OS}` to `enigmacli` or `enigmacli.exe` and put it in your path.
   - Ubuntu/Debian: `sudo dpkg -i enigmachain*.deb`

3) Configure:

   ```bash
   # Set the mainnet chain-id
   enigmacli config chain-id enigma-1
   ```

   ```bash
   enigmacli config output json
   ```

   ```bash
   enigmacli config indent true
   ```

   ```bash
   # Set the full node address
   enigmacli config node tcp://bootstrap.mainnet.enigma.co:26657
   ```

   ```bash
   # Verify everything you receive from the full node
   enigmacli config trust-node false
   ```

4) Check the installation:

   ```bash
   enigmacli status
   ```
