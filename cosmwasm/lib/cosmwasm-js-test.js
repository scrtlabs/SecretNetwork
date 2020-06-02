#!/usr/bin/env node
const path = require("path");
const cosmwasmjs = require(path.resolve(
  __dirname,
  "../../cosmwasm-js/packages/sdk/build/"
));
const client = new cosmwasmjs.CosmWasmClient("http://localhost:1337");

(async () => {
  const contracts = await client.getContracts(1);

  const res = await client.queryContractSmart(contracts[0].address, {
    balance: { address: "enigma1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rp5vqd4" },
  });
  console.log(res);
})();
