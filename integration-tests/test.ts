import { sha256 } from "@noble/hashes/sha256";
import * as fs from "fs";
import {
  fromBase64,
  MsgInstantiateContract,
  MsgStoreCode,
  SecretNetworkClient,
  toBase64,
  toHex,
  Wallet,
} from "secretjs";
import {
  QueryBalanceRequest,
  QueryBalanceResponse,
} from "secretjs//dist/protobuf_stuff/cosmos/bank/v1beta1/query";
import { MsgSend } from "secretjs/dist/protobuf_stuff/cosmos/bank/v1beta1/tx";
import { ibcDenom } from "./utils";

// @ts-ignore
// accounts on secretdev-1
const accounts: {
  a: SecretNetworkClient;
  b: SecretNetworkClient;
  c: SecretNetworkClient;
  // to prevent a sequence mismatch, avoid using account d which is used by the ibc relayer
} = {};

// @ts-ignore
// accounts on secretdev-2
const accounts2: {
  a: SecretNetworkClient;
  b: SecretNetworkClient;
  c: SecretNetworkClient;
  // to prevent a sequence mismatch, avoid using account d which is used by the ibc relayer
} = {};

let v1CodeID: number;
let v1Address: string;
let v1CodeHash: string;

let v010CodeID: number;
let v010Address: string;
let v010CodeHash: string;

beforeAll(async () => {
  const walletA = new Wallet(
    "grant rice replace explain federal release fix clever romance raise often wild taxi quarter soccer fiber love must tape steak together observe swap guitar"
  );

  const walletB = new Wallet(
    "jelly shadow frog dirt dragon use armed praise universe win jungle close inmate rain oil canvas beauty pioneer chef soccer icon dizzy thunder meadow"
  );

  const walletC = new Wallet(
    "chair love bleak wonder skirt permit say assist aunt credit roast size obtain minute throw sand usual age smart exact enough room shadow charge"
  );

  accounts.a = await SecretNetworkClient.create({
    chainId: "secretdev-1",
    grpcWebUrl: "http://localhost:9091",
    wallet: walletA,
    walletAddress: walletA.address,
  });

  accounts.b = await SecretNetworkClient.create({
    chainId: "secretdev-1",
    grpcWebUrl: "http://localhost:9091",
    wallet: walletB,
    walletAddress: walletB.address,
  });

  accounts.c = await SecretNetworkClient.create({
    chainId: "secretdev-1",
    grpcWebUrl: "http://localhost:9091",
    wallet: walletC,
    walletAddress: walletC.address,
  });

  accounts2.a = await SecretNetworkClient.create({
    chainId: "secretdev-2",
    grpcWebUrl: "http://localhost:9391",
    wallet: walletA,
    walletAddress: walletA.address,
  });

  accounts.b = await SecretNetworkClient.create({
    chainId: "secretdev-1",
    grpcWebUrl: "http://localhost:9091",
    wallet: walletB,
    walletAddress: walletB.address,
  });

  accounts.c = await SecretNetworkClient.create({
    chainId: "secretdev-1",
    grpcWebUrl: "http://localhost:9091",
    wallet: walletC,
    walletAddress: walletC.address,
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

  console.log("Uploading contracts...");
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

  v1CodeID = Number(tx.arrayLog.find((x) => x.key === "code_id").value);
  v010CodeID = Number(
    tx.arrayLog.reverse().find((x) => x.key === "code_id").value
  );

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
  const secretjs = await SecretNetworkClient.create({
    grpcWebUrl: "http://localhost:9091",
    chainId: "secretdev-1",
  });

  while (true) {
    try {
      const { block } = await secretjs.query.tendermint.getLatestBlock({});

      if (Number(block?.header?.height) >= 1) {
        console.log("Current block:", JSON.stringify(block.header.height));
        break;
      }
    } catch (e) {
      // console.error("block error:", e);
    }
    await sleep(100);
  }
}

// the docker compose opens the transfer channel so if we find an open channel that means that a client and a connection
// have already been set up
async function waitForIBC(chainId: string, grpcWebUrl: string) {
  const secretjs = await SecretNetworkClient.create({
    grpcWebUrl,
    chainId,
  });

  console.log("Looking for open channels on", chainId + "...");
  while (true) {
    try {
      const { channels } = await secretjs.query.ibc_channel.channels({});

      if (channels.length >= 1) {
        console.log("Found open channel on", chainId);
        break;
      }
    } catch (e) {
      // console.error("IBC error:", e, "on chain", chainId);
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

describe("IBC", () => {
  beforeAll(async () => {
    console.log("Waiting for IBC to set up...");
    await waitForIBC("secretdev-1", "http://localhost:9091");
    await waitForIBC("secretdev-2", "http://localhost:9391");
  }, 3 * 60 * 1000);

  test(
    "transfer sanity",
    async () => {
      const denom = ibcDenom(
        [
          {
            portId: "transfer",
            channelId: "channel-0",
          },
        ],
        "uscrt"
      );
      const { balance: balanceBefore } = await accounts2.a.query.bank.balance({
        address: accounts2.a.address,
        denom,
      });
      const amountBefore = Number(balanceBefore?.amount ?? "0");

      const result = await accounts.a.tx.ibc.transfer({
        receiver: accounts.a.address,
        sender: accounts.a.address,
        sourceChannel: "channel-0",
        sourcePort: "transfer",
        token: {
          denom: "uscrt",
          amount: "1",
        },
        timeoutTimestampSec: String(Math.floor(Date.now() / 1000 + 30)),
      });

      if (result.code !== 0) {
        console.error(result.rawLog);
      }

      expect(result.code).toBe(0);

      // TODO check ack on secretdev-1

      while (true) {
        try {
          const { balance: balanceAfter } =
            await accounts2.a.query.bank.balance({
              address: accounts2.a.address,
              denom,
            });
          const amountAfter = Number(balanceAfter?.amount ?? "0");

          if (amountAfter === amountBefore + 1) {
            break;
          }
        } catch (e) {
          // console.error("ibc denom balance error:", e);
        }
        await sleep(200);
      }
      expect(true).toBe(true);
    },
    1000 * 30
  );
});
