#!/usr/bin/env node
const path = require("path");
const cosmwasmjs = require(path.resolve(
  __dirname,
  "../../cosmwasm-js/packages/sdk/build/"
));
const assert = require("assert").strict;

process.on("unhandledRejection", (error) => {
  console.error(error.message);
  process.exit(1);
});

(async () => {
  const seed = cosmwasmjs.EnigmaUtils.GenerateNewSeed();
  const client = new cosmwasmjs.CosmWasmClient("http://localhost:1317", seed);
  const contractAddr = (await client.getContracts(1))[0].address;
  const contractCodeHash = await client.getCodeHashByContractAddr(contractAddr);
  const pen = await cosmwasmjs.Secp256k1Pen.fromMnemonic(
    "cost member exercise evoke isolate gift cattle move bundle assume spell face balance lesson resemble orange bench surge now unhappy potato dress number acid"
  );
  const address = cosmwasmjs.pubkeyToAddress(
    cosmwasmjs.encodeSecp256k1Pubkey(pen.pubkey),
    "secret"
  );
  const signingClient = new cosmwasmjs.SigningCosmWasmClient(
    "http://localhost:1317",
    address,
    (signBytes) => pen.sign(signBytes),
    seed,
    {
      upload: {
        amount: [{ amount: "1000000", denom: "uscrt" }],
        gas: "1000000",
      },
      init: {
        amount: [{ amount: "500000", denom: "uscrt" }],
        gas: "500000",
      },
      exec: {
        amount: [{ amount: "200000", denom: "uscrt" }],
        gas: "200000",
      },
      send: {
        amount: [{ amount: "80000", denom: "uscrt" }],
        gas: "80000",
      },
    }
  );

  const execTx = await signingClient.execute(contractAddr, {
    a: { contract_addr: contractAddr, code_hash: contractCodeHash, x: 2, y: 3 },
  });

  const tx = await client.restClient.txById(execTx.transactionHash);

  assert.deepEqual(execTx.logs, tx.logs);
  assert.deepEqual(execTx.data, tx.data);
  assert.deepEqual(tx.data, Uint8Array.from([2, 3]));
  assert.deepEqual(tx.logs[0].events[1].attributes, [
    {
      key: "contract_address",
      value: contractAddr,
    },
    {
      key: "banana",
      value: "🍌",
    },
    {
      key: "contract_address",
      value: contractAddr,
    },
    {
      key: "kiwi",
      value: "🥝",
    },
    {
      key: "contract_address",
      value: contractAddr,
    },
    {
      key: "watermelon",
      value: "🍉",
    },
  ]);
  console.log("ok 👌");
})();
