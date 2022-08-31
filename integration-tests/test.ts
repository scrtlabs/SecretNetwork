import {
  fromBase64,
  MsgInstantiateContract,
  MsgStoreCode,
  SecretNetworkClient,
  toBase64,
  toHex,
  Wallet,
} from "secretjs";
import { MsgSend } from "secretjs/dist/protobuf_stuff/cosmos/bank/v1beta1/tx";
import {
  QueryBalanceRequest,
  QueryBalanceResponse,
} from "secretjs//dist/protobuf_stuff/cosmos/bank/v1beta1/query";
import { sha256 } from "@noble/hashes/sha256";
import * as fs from "fs";
import { cleanBytes } from "./utils";

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

let v010CodeID: number;
let v010Address: string;
let v010CodeHash: string;

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

  const v010Wasm = fs.readFileSync(
    `${__dirname}/contract-v0.10/contract.wasm`
  ) as Uint8Array;
  v010CodeHash = toHex(sha256(v010Wasm));

  let tx;
  tx = await accounts.a.tx.broadcast(
    [
      new MsgStoreCode({
        sender: accounts.a.address,
        wasmByteCode: v1Wasm,
        source: "",
        builder: "",
      }),
      new MsgStoreCode({
        sender: accounts.a.address,
        wasmByteCode: v010Wasm,
        source: "",
        builder: "",
      }),
    ],
    { gasLimit: 5_000_000 }
  );
  if (tx.code !== 0) {
    console.error(tx.rawLog);
  }
  expect(tx.code).toBe(0);
  console.log("tx", cleanBytes(tx));

  v1CodeID = Number(tx.arrayLog.find((x) => x.key === "code_id").value);
  console.log("v1CodeID:", v1CodeID);
  v010CodeID = Number(
    tx.arrayLog.reverse().find((x) => x.key === "code_id").value
  );
  console.log("v010CodeID:", v010CodeID);

  tx = await accounts.a.tx.broadcast(
    [
      new MsgInstantiateContract({
        sender: accounts.a.address,
        codeId: v1CodeID,
        codeHash: v1CodeHash,
        initMsg: { nop: {} },
        label: `v1-${Math.random()}`,
      }),
      new MsgInstantiateContract({
        sender: accounts.a.address,
        codeId: v010CodeID,
        codeHash: v010CodeHash,
        initMsg: { nop: {} },
        label: `v010-${Math.random()}`,
      }),
    ],
    { gasLimit: 200_000 }
  );
  if (tx.code !== 0) {
    console.error(tx.rawLog);
  }
  expect(tx.code).toBe(0);

  v1Address = tx.arrayLog.find((x) => x.key === "contract_address").value;
  v010Address = tx.arrayLog
    .reverse()
    .find((x) => x.key === "contract_address").value;
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
        console.log("blocks are running, current block:", JSON.stringify(block.header.height));
        break;
      }
    } catch (e) {
      console.error("block error:", e);
    }
    await sleep(100);
  }
}

describe("Bank::MsgSend", () => {
  test("v1", async () => {
    const tx = await accounts.a.tx.compute.executeContract(
      {
        sender: accounts.a.address,
        contractAddress: v1Address,
        codeHash: v1CodeHash,
        msg: {
          bank_msg_send: {
            to_address: accounts.b.address,
            amount: [{ amount: "1", denom: "uscrt" }],
          },
        },
        sentFunds: [{ amount: "1", denom: "uscrt" }],
      },
      { gasLimit: 250_000 }
    );
    if (tx.code !== 0) {
      console.error(tx.rawLog);
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
    const tx = await accounts.a.tx.compute.executeContract(
        {
          sender: accounts.a.address,
          contractAddress: v010Address,
          codeHash: v010CodeHash,
          msg: {
            bank_msg_send: {
              to_address: accounts.b.address,
              amount: [{ amount: "1", denom: "uscrt" }],
            },
          },
          sentFunds: [{ amount: "1", denom: "uscrt" }],
        },
        { gasLimit: 250_000 }
    );
    if (tx.code !== 0) {
      console.error(tx.rawLog);
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
        value: v010Address,
      },
      { key: "amount", msg: 0, type: "coin_spent", value: "1uscrt" },
    ]);
    expect(tx.arrayLog.filter((x) => x.type === "coin_received")).toStrictEqual(
        [
          {
            key: "receiver",
            msg: 0,
            type: "coin_received",
            value: v010Address,
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
      console.error(tx.rawLog);
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
});

describe("StargateQuery", () => {
  test("v1", async () => {
    const result: any = await accounts.a.query.compute.queryContract({
      contractAddress: v1Address,
      codeHash: v1CodeHash,
      query: {
        stargate: {
          path: "/cosmos.bank.v1beta1.Query/Balance",
          data: toBase64(
            QueryBalanceRequest.encode({
              address: accounts.a.address,
              denom: "uscrt",
            }).finish()
          ),
        },
      },
    });

    const response = QueryBalanceResponse.decode(fromBase64(result));
    expect(response?.balance?.denom).toBe("uscrt");
    expect(Number(response?.balance?.amount)).toBeGreaterThanOrEqual(1);
  });
});

describe("BankQuery", () => {
  describe("Balance", () => {
    test("v1", async () => {
      const result: any = await accounts.a.query.compute.queryContract({
        contractAddress: v1Address,
        codeHash: v1CodeHash,
        query: {
          bank_balance: {
            address: accounts.a.address,
            denom: "uscrt",
          },
        },
      });
      expect(result?.amount?.denom).toBe("uscrt");
      expect(Number(result?.amount?.amount)).toBeGreaterThanOrEqual(1);
    });

    test("v0.10", async () => {
      const result: any = await accounts.a.query.compute.queryContract({
        contractAddress: v010Address,
        codeHash: v010CodeHash,
        query: {
          bank_balance: {
            address: accounts.a.address,
            denom: "uscrt",
          },
        },
      });
      expect(result?.amount?.denom).toBe("uscrt");
      expect(Number(result?.amount?.amount)).toBeGreaterThanOrEqual(1);
    });
  });
});
