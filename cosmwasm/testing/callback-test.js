#!/usr/bin/env node
const path = require("path");
const cosmwasmjs = require(path.resolve(
  __dirname,
  "../../cosmwasm-js/packages/sdk/build/"
));
const assert = require("assert").strict;

(async () => {
  const client = new cosmwasmjs.CosmWasmClient("http://localhost:1337");
  const contract = (await client.getContracts(1))[0].address;

  const pen = await cosmwasmjs.Secp256k1Pen.fromMnemonic(
    "cost member exercise evoke isolate gift cattle move bundle assume spell face balance lesson resemble orange bench surge now unhappy potato dress number acid"
  );
  const address = cosmwasmjs.pubkeyToAddress(
    cosmwasmjs.encodeSecp256k1Pubkey(pen.pubkey),
    "enigma"
  );
  const signingClient = new cosmwasmjs.SigningCosmWasmClient(
    "http://localhost:1337",
    address,
    (signBytes) => pen.sign(signBytes),
    {
      upload: {
        amount: [{ amount: "25000", denom: "uscrt" }],
        gas: "1000000",
      },
      init: {
        amount: [{ amount: "12500", denom: "uscrt" }],
        gas: "500000",
      },
      exec: {
        amount: [{ amount: "5000", denom: "uscrt" }],
        gas: "200000",
      },
      send: {
        amount: [{ amount: "2000", denom: "uscrt" }],
        gas: "80000",
      },
    }
  );

  const execTx = await signingClient.execute(contract, {
    a: { contract_addr: contract, x: 2, y: 3 },
  });

  const tx = await client.restClient.txById(execTx.transactionHash);

  assert.deepEqual(execTx.logs, tx.logs);
  assert.deepEqual(execTx.data, tx.data);
  assert.deepEqual(tx.data.data, Uint8Array.from([65, 103, 77, 61]));

  assert.deepEqual(tx.logs[0].events[1].attributes, [
    {
      key: "contract_address",
      value: contract,
    },
    {
      key: "banana",
      value: "🍌",
    },
    {
      key: "contract_address",
      value: contract,
    },
    {
      key: "kiwi",
      value: "🥝",
    },
    {
      key: "contract_address",
      value: contract,
    },
    {
      key: "watermelon",
      value: "🍉",
    },
  ]);
  console.log("ok");
})();
