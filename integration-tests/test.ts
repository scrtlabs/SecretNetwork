import { SecretNetworkClient, toBase64, toHex, Wallet } from "secretjs";
import { MsgSend } from "secretjs/dist/protobuf_stuff/cosmos/bank/v1beta1/tx";
import { sha256 } from "@noble/hashes/sha256";
import * as fs from "fs";

// @ts-ignore
const accounts: {
  a: SecretNetworkClient;
  b: SecretNetworkClient;
  c: SecretNetworkClient;
  d: SecretNetworkClient;
} = {};

let v1CodeID: number;
let v1Address: string;
let v1CodeHash: string;

beforeAll(async () => {
  accounts.a = await SecretNetworkClient.create({
    chainId: "secretdev-1",
    grpcWebUrl: "http://localhost:9091",
    wallet: new Wallet(
      "grant rice replace explain federal release fix clever romance raise often wild taxi quarter soccer fiber love must tape steak together observe swap guitar"
    ),
    walletAddress: "secret1ap26qrlp8mcq2pg6r47w43l0y8zkqm8a450s03",
  });

  accounts.b = await SecretNetworkClient.create({
    chainId: "secretdev-1",
    grpcWebUrl: "http://localhost:9091",
    wallet: new Wallet(
      "jelly shadow frog dirt dragon use armed praise universe win jungle close inmate rain oil canvas beauty pioneer chef soccer icon dizzy thunder meadow"
    ),
    walletAddress: "secret1fc3fzy78ttp0lwuujw7e52rhspxn8uj52zfyne",
  });

  accounts.c = await SecretNetworkClient.create({
    chainId: "secretdev-1",
    grpcWebUrl: "http://localhost:9091",
    wallet: new Wallet(
      "chair love bleak wonder skirt permit say assist aunt credit roast size obtain minute throw sand usual age smart exact enough room shadow charge"
    ),
    walletAddress: "secret1ajz54hz8azwuy34qwy9fkjnfcrvf0dzswy0lqq",
  });

  accounts.d = await SecretNetworkClient.create({
    chainId: "secretdev-1",
    grpcWebUrl: "http://localhost:9091",
    wallet: new Wallet(
      "word twist toast cloth movie predict advance crumble escape whale sail such angry muffin balcony keen move employ cook valve hurt glimpse breeze brick"
    ),
    walletAddress: "secret1ldjxljw7v4vk6zhyduywh04hpj0jdwxsmrlatf",
  });

  console.log("Waiting for LocalSecret to start...");
  await waitForBlocks();

  const v1Wasm = fs.readFileSync(
    `${__dirname}/contract-v1/contract.wasm`
  ) as Uint8Array;
  v1CodeHash = toHex(sha256(v1Wasm));

  let tx = await accounts.a.tx.compute.storeCode(
    {
      sender: accounts.a.address,
      wasmByteCode: v1Wasm,
      source: "",
      builder: "",
    },
    { gasLimit: 5_000_000 }
  );
  if (tx.code !== 0) {
    console.log(tx.rawLog);
  }
  expect(tx.code).toBe(0);

  v1CodeID = Number(tx.arrayLog.find((x) => x.key === "code_id").value);

  tx = await accounts.a.tx.compute.instantiateContract(
    {
      sender: accounts.a.address,
      codeId: v1CodeID,
      codeHash: v1CodeHash,
      initMsg: { nop: {} },
      label: `v1-${Math.random()}`,
    },
    { gasLimit: 100_000 }
  );
  if (tx.code !== 0) {
    console.log(tx.rawLog);
  }
  expect(tx.code).toBe(0);

  v1Address = tx.arrayLog.find((x) => x.key === "contract_address").value;
});

async function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
async function waitForBlocks() {
  while (true) {
    const secretjs = await SecretNetworkClient.create({
      grpcWebUrl: "http://localhost:9091",
      chainId: "secretdev-1",
    });

    try {
      const { block } = await secretjs.query.tendermint.getLatestBlock({});

      if (Number(block?.header?.height) >= 1) {
        break;
      }
    } catch (e) {
      // console.error(e);
    }
    await sleep(250);
  }
}

describe("BankMsg::Send", () => {
  test("v1", async () => {
    const tx = await accounts.a.tx.compute.executeContract(
      {
        sender: accounts.a.address,
        contractAddress: v1Address,
        codeHash: v1CodeHash,
        msg: {
          bank_msg: {
            to_address: accounts.b.address,
            amount: [{ amount: "1", denom: "uscrt" }],
          },
        },
        sentFunds: [{ amount: "1", denom: "uscrt" }],
      },
      { gasLimit: 250_000 }
    );
    if (tx.code !== 0) {
      console.log(tx.rawLog);
    }
    expect(tx.code).toBe(0);
    expect(tx.arrayLog.filter((x) => x.type === "coin_spent")).toStrictEqual([
      {
        key: "spender",
        msg: 0,
        type: "coin_spent",
        value: accounts.a.address,
      },
      { key: "amount", msg: 0, type: "coin_spent", value: "1uscrt" },
      {
        key: "spender",
        msg: 0,
        type: "coin_spent",
        value: v1Address,
      },
      { key: "amount", msg: 0, type: "coin_spent", value: "1uscrt" },
    ]);
    expect(tx.arrayLog.filter((x) => x.type === "coin_received")).toStrictEqual(
      [
        {
          key: "receiver",
          msg: 0,
          type: "coin_received",
          value: v1Address,
        },
        { key: "amount", msg: 0, type: "coin_received", value: "1uscrt" },
        {
          key: "receiver",
          msg: 0,
          type: "coin_received",
          value: accounts.b.address,
        },
        { key: "amount", msg: 0, type: "coin_received", value: "1uscrt" },
      ]
    );
  });

  test("v0.10", async () => {
    // TODO
  });
});

describe("StargateMsg", () => {
  test("v1", async () => {
    const tx = await accounts.a.tx.compute.executeContract(
      {
        sender: accounts.a.address,
        contractAddress: v1Address,
        codeHash: v1CodeHash,
        msg: {
          stargate_msg: {
            type_url: "/cosmos.bank.v1beta1.MsgSend",
            value: toBase64(
              MsgSend.encode({
                fromAddress: v1Address,
                toAddress: accounts.b.address,
                amount: [{ amount: "1", denom: "uscrt" }],
              }).finish()
            ),
          },
        },
        sentFunds: [{ amount: "1", denom: "uscrt" }],
      },
      { gasLimit: 250_000 }
    );
    if (tx.code !== 0) {
      console.log(tx.rawLog);
    }
    expect(tx.code).toBe(0);
    expect(tx.arrayLog.filter((x) => x.type === "coin_spent")).toStrictEqual([
      {
        key: "spender",
        msg: 0,
        type: "coin_spent",
        value: accounts.a.address,
      },
      { key: "amount", msg: 0, type: "coin_spent", value: "1uscrt" },
      {
        key: "spender",
        msg: 0,
        type: "coin_spent",
        value: v1Address,
      },
      { key: "amount", msg: 0, type: "coin_spent", value: "1uscrt" },
    ]);
    expect(tx.arrayLog.filter((x) => x.type === "coin_received")).toStrictEqual(
      [
        {
          key: "receiver",
          msg: 0,
          type: "coin_received",
          value: v1Address,
        },
        { key: "amount", msg: 0, type: "coin_received", value: "1uscrt" },
        {
          key: "receiver",
          msg: 0,
          type: "coin_received",
          value: accounts.b.address,
        },
        { key: "amount", msg: 0, type: "coin_received", value: "1uscrt" },
      ]
    );
  });

  test("v0.10", async () => {
    // TODO
  });
});
