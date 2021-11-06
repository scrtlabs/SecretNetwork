#!/usr/bin/env node
const path = require("path");
const cosmwasmjs = require(path.resolve(
  __dirname,
  "../../cosmwasm-js/packages/sdk/build/"
));
const {MsgData} = require("../../cosmwasm-js/packages/sdk/build/ProtoEncoding");
const assert = require("assert").strict;

process.on("unhandledRejection", (error) => {
  console.error(error.message);
  process.exit(1);
});

async function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

(async () => {
  const seed = cosmwasmjs.EnigmaUtils.GenerateNewSeed();
  const client = new cosmwasmjs.CosmWasmClient("http://localhost:1317", seed);
  console.log("1")
  const contract = (await client.getContracts(1))[0].address;

  console.log("2")
  const resQuery = await client.queryContractSmart(contract, {
    balance: { address: "secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t" },
  });
  console.log("3")
  const initBalance = +resQuery.balance;

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

  console.log("4")
  const execTx = await signingClient.execute(contract, {
    transfer: {
      amount: "10",
      recipient: "secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t",
    },
  });
  console.log("5")
  await sleep(5000);

  const tx = await client.restClient.txById(execTx.transactionHash);

  assert.deepEqual(execTx.logs, tx.logs);
  assert.deepEqual(execTx.data, tx.data);
  assert.deepEqual(tx.data, Uint8Array.from([]));
  assert.deepEqual(tx.logs[0].events[1].attributes, [
    {
      key: "contract_address",
      value: contract,
    },
    {
      key: "action",
      value: "transfer",
    },
    {
      key: "sender",
      value: "secret18rxhudxdx6wen48rtnrf4jv5frf47qa9ws2ju3",
    },
    {
      key: "recipient",
      value: "secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t",
    },
  ]);

  console.log("6")
  const qRes = await client.queryContractSmart(contract, {
    balance: { address: "secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t" },
  });

  assert.equal(+qRes.balance, initBalance + 10);

  console.log("7")
  const qRes2 = await client.queryContractSmart(contract, {
    balance: { address: "secret18rxhudxdx6wen48rtnrf4jv5frf47qa9ws2ju3" },
  });

  console.log("8")
  try {
    await signingClient.execute(contract, {
      transfer: {
        amount: "1000",
        recipient: "secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t",
      },
    });
  } catch (err) {
    assert(
      err.message.includes(
        `Insufficient funds: balance=${qRes2.balance}, required=1000"`
      )
    );

    const txId = /Error when posting tx (.+?)\./.exec(err.message)[1];

    console.log(`Searching for TX ID: ${txId}`)
    await sleep(5000);
    const tx = await client.restClient.txById(txId);
    assert(
      tx.raw_log.includes(
        `Insufficient funds: balance=${qRes2.balance}, required=1000"`
      )
    );
  }

  console.log("8")
  try {
    await client.queryContractSmart(contract, {
      balance: { address: "blabla" },
    });
  } catch (err) {
    assert(
      err.message.includes("canonicalize_address errored: invalid length"),
      `'${err.message}' does not include 'canonicalize_address errored: invalid length'`
    );
  }

  console.log("9")
  console.log("ok 👌");
})();
