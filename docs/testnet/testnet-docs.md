**_NOTE:_** You DO NOT want to use the v0.1.0 on an existing mainnet validator node. It will not work. <br>
**_NOTE 2:_** We recommend you to first remove older `enigmachain` installations to prevent collisions. <br>
**_NOTE 3:_** Before removing, changing or doing anything with the old `enigmad`,`enigmacli` installations - make sure to back up your keys and recovery seeds!

## Set up a light client

For smart contract testing and development, most of you would choose this option.
You would do this if you have no interest at all at running a full-node or a validator node on testnet.

1. Get the v0.1.0 pre-release from our repo: https://github.com/enigmampc/SecretNetwork/releases/tag/v0.1.0
   (Currently support is only for Debian/Ubuntu. Other OS distributions coming soon)
2. Uninstall previous releases:

   ```bash
   sudo dpkg -r enigmachain
   ```

3. Install:

   ```bash
   sudo dpkg -i enigma-blockchain_0.1.0_amd64.deb
   ```

4. Configure the client to point to the testnet nodes:

   ```bash
   enigmacli config chain-id enigma-testnet
   enigmacli config node tcp://bootstrap.testnet.enigma.co:26657
   ```

5. Check installation:

   ```bash
   enigmacli status
   ```

## Use smart contracts

The smart contracts module we embedded into enigma-blockchain is called `compute`.
run `enigmacli tx compute --help` for more info.

Smart Contracts docs will be posted soon, in the meantime you should check out [CosmWasm's docs](https://github.com/confio/cosmwasm) for info about writing and deploying smart contracts.

## Get some Testnet-SCRT

Please don't abuse this service—the number of available tokens is limited.

1. Head to https://faucet.testnet.enigma.co .
2. Generate a key-pair:

   ```bash
   enigmacli keys add [your-key-name]
   ```

3. Fill in your address and press `Send me tokens`.

**_NOTE:_** You can, technically, use the same address as your mainnet address, although **we strongly recommend against it to avoid confusions!** Just create another key with unmistakable name like `testnet-tom` for example.

## Run a full node

1. Run steps 1-3 of the previous section (light client guide) on your server.
2. Initialize your installation of the Secret Network. Choose a  **moniker**  for yourself that will be public, and replace  `<MONIKER>`  with your moniker below

   ```bash
   enigmad init <MONIKER> --chain-id enigma-testnet
   ```

3. Download a copy of the genesis file:

   ```bash
   wget -O ~/.enigmad/config/genesis.json "https://raw.githubusercontent.com/enigmampc/SecretNetwork/master/enigma-testnet-genesis.json"
   ```

4. Validate the checksum of the file:

   ```bash
   echo "cc7ab684b955dcc78baffd508530f0a119723836d24153b41d8669f0e4ec3caa $HOME/.enigmad/config/genesis.json" | sha256sum --check
   ```

5. Validate genesis:

   ```bash
   enigmad validate-genesis
   ```

6. Add the bootstrap node as a persistent peer:

   ```bash
   perl -i -pe 's/persistent_peers = ""/persistent_peers = "16e95298703bfbf6565a1cbb6691cf30129f52ca\@bootstrap.testnet.enigma.co:26656"/' ~/.enigmad/config/config.toml
   ```

7. Run your node:

   ```bash
   sudo systemctl enable enigma-node
   sudo systemctl start enigma-node
   ```

8. Verify success:

   ```bash
   journalctl -f -u enigma-node
   ```

Logs should look similar to this:

```bash
Mar 05 19:13:08 ip-172-31-44-28 enigmad[3083]: I[2020-03-05|19:13:08.623] Executed block                               module=state height=1920 validTxs=0 invalidTxs=0
Mar 05 19:13:08 ip-172-31-44-28 enigmad[3083]: I[2020-03-05|19:13:08.633] Committed state                              module=state height=1920 txs=0 appHash=079C94F8198AC7F25BF5CF453F12B56A73816A4D07BA01630D3138A66136B340
Mar 05 19:13:13 ip-172-31-44-28 enigmad[3083]: I[2020-03-05|19:13:13.698] Executed block                               module=state height=1921 validTxs=0 invalidTxs=0
Mar 05 19:13:13 ip-172-31-44-28 enigmad[3083]: I[2020-03-05|19:13:13.707] Committed state                              module=state height=1921 txs=0 appHash=1CB9AA6337DCF83F09687965CEF539FD25AA17F5BB8AF520575A891CFB05A178
Mar 05 19:13:18 ip-172-31-44-28 enigmad[3083]: I[2020-03-05|19:13:18.775] Executed block                               module=state height=1922 validTxs=0 invalidTxs=0
Mar 05 19:13:18 ip-172-31-44-28 enigmad[3083]: I[2020-03-05|19:13:18.784] Committed state                              module=state height=1922 txs=0 appHash=E27C56C5F1D3A85E1E75F3882877065B06BACFC5CED8FA401CE066B8FFEDF608
```

You have an active full node :tada:
