import { sha256 } from "@noble/hashes/sha256";
import { execSync } from "child_process";
import * as fs from "fs";
import {
  fromBase64,
  fromUtf8,
  MsgExecuteContract,
  ProposalType,
  SecretNetworkClient,
  stringToCoin,
  stringToCoins,
  toBase64,
  toHex,
  toUtf8,
  TxResponse,
  TxResultCode,
  Wallet,
} from "secretjs";
import {
  QueryBalanceRequest,
  QueryBalanceResponse,
} from "secretjs//dist/protobuf/cosmos/bank/v1beta1/query";
import { MsgSend } from "secretjs/dist/protobuf/cosmos/bank/v1beta1/tx";
import { MsgSend as MsgSendMsg } from "secretjs/dist/tx/bank";
import { AminoWallet } from "secretjs/dist/wallet_amino";
import {
  ibcDenom,
  sleep,
  storeContracts,
  waitForBlocks,
  waitForIBCChannel,
  waitForIBCConnection,
  Contract,
  instantiateContracts,
  cleanBytes,
} from "./utils";

type Account = {
  address: string;
  mnemonic: string;
  walletAmino: AminoWallet;
  walletProto: Wallet;
  secretjs: SecretNetworkClient;
};

const accountsCount = 30;

// @ts-ignore
// accounts on secretdev-1
const accounts: Account[] = new Array(accountsCount);
const contracts = {
  "secretdev-1": {
    v1: new Contract("v1"),
    v010: new Contract("v010"),
  },
  "secretdev-2": {
    v1: new Contract("v1"),
    v010: new Contract("v010"),
  },
};

// let accounts;
// let accounts2;
// let readonly;

let v1Wasm: Uint8Array;
let v010Wasm: Uint8Array;

let readonly: SecretNetworkClient;

// @ts-ignore
// accounts on secretdev-2
const accounts2: Account[] = new Array(3);
let readonly2: SecretNetworkClient;

beforeAll(async () => {
  const mnemonics = [
    "grant rice replace explain federal release fix clever romance raise often wild taxi quarter soccer fiber love must tape steak together observe swap guitar",
    "jelly shadow frog dirt dragon use armed praise universe win jungle close inmate rain oil canvas beauty pioneer chef soccer icon dizzy thunder meadow",
  ];

  // Create clients for all of the existing wallets in secretdev-1
  for (let i = 0; i < mnemonics.length; i++) {
    const mnemonic = mnemonics[i];
    const walletAmino = new AminoWallet(mnemonic);
    accounts[i] = {
      address: walletAmino.address,
      mnemonic: mnemonic,
      walletAmino,
      walletProto: new Wallet(mnemonic),
      secretjs: new SecretNetworkClient({
        url: "http://localhost:1317",
        wallet: walletAmino,
        walletAddress: walletAmino.address,
        chainId: "secretdev-1",
      }),
    };
  }

  // Create clients for all of the existing wallets in secretdev-2
  for (let i = 0; i < mnemonics.length; i++) {
    const mnemonic = mnemonics[i];
    const walletAmino = new AminoWallet(mnemonic);
    accounts2[i] = {
      address: walletAmino.address,
      mnemonic: mnemonic,
      walletAmino,
      walletProto: new Wallet(mnemonic),
      secretjs: new SecretNetworkClient({
        url: "http://localhost:2317",
        wallet: walletAmino,
        walletAddress: walletAmino.address,
        chainId: "secretdev-2",
      }),
    };
  }

  // Create temporary wallets to fit all other usages (See TXCount test)
  for (let i = mnemonics.length; i < accountsCount; i++) {
    const wallet = new AminoWallet();
    const [{ address }] = await wallet.getAccounts();
    const walletProto = new Wallet(wallet.mnemonic);

    accounts[i] = {
      address: address,
      mnemonic: wallet.mnemonic,
      walletAmino: wallet,
      walletProto: walletProto,
      secretjs: new SecretNetworkClient({
        url: "http://localhost:1317",
        chainId: "secretdev-1",
        wallet: wallet,
        walletAddress: address,
      }),
    };
  }

  // Send 100k SCRT from account 0 to each of accounts 1-itrations

  const { secretjs } = accounts[0];

  let t: TxResponse;
  try {
    t = await secretjs.tx.bank.multiSend(
      {
        inputs: [
          {
            address: secretjs.address,
            coins: stringToCoins(`${100_000 * 1e6 * (accountsCount - 1)}uscrt`),
          },
        ],
        outputs: accounts.slice(1).map(({ address }) => ({
          address,
          coins: stringToCoins(`${100_000 * 1e6}uscrt`),
        })),
      },
      {
        gasLimit: 200_000,
      }
    );
  } catch (e) {
    throw new Error(`Failed to multisend: ${e.stack}`);
  }

  if (t.code !== 0) {
    console.error(`failed to multisend coins`);
    throw new Error("Failed to multisend coins to initial accounts");
  }

  readonly = new SecretNetworkClient({
    chainId: "secretdev-1",
    url: "http://localhost:1317",
  });

  readonly2 = new SecretNetworkClient({
    chainId: "secretdev-2",
    url: "http://localhost:2317",
  });
  await waitForBlocks("secretdev-1");

  v1Wasm = fs.readFileSync(
    `${__dirname}/contract-v1/contract.wasm`
  ) as Uint8Array;
  contracts["secretdev-1"].v1.codeHash = toHex(sha256(v1Wasm));

  v010Wasm = fs.readFileSync(
    `${__dirname}/contract-v0.10/contract.wasm`
  ) as Uint8Array;
  contracts["secretdev-1"].v010.codeHash = toHex(sha256(v010Wasm));

  console.log("Storing contracts on secretdev-1...");
  let tx: TxResponse = await storeContracts(accounts[0].secretjs, [
    v1Wasm,
    v010Wasm,
  ]);

  contracts["secretdev-1"].v1.codeId = Number(
    tx.arrayLog.find((x) => x.key === "code_id").value
  );
  contracts["secretdev-1"].v010.codeId = Number(
    tx.arrayLog.reverse().find((x) => x.key === "code_id").value
  );

  console.log("Instantiating contracts on secretdev-1...");
  tx = await instantiateContracts(accounts[0].secretjs, [
    contracts["secretdev-1"].v1,
    contracts["secretdev-1"].v010,
  ]);

  contracts["secretdev-1"].v1.address = tx.arrayLog.find(
    (x) => x.key === "contract_address"
  ).value;
  contracts["secretdev-1"].v1.ibcPortId =
    "wasm." + contracts["secretdev-1"].v1.address;

  contracts["secretdev-1"].v010.address = tx.arrayLog
    .reverse()
    .find((x) => x.key === "contract_address").value;

  // create a second validator for MsgRedelegate tests
  const { validators } = await readonly.query.staking.validators({});
  if (validators.length === 1) {
    tx = await accounts[1].secretjs.tx.staking.createValidator(
      {
        delegator_address: accounts[1].address,
        commission: {
          max_change_rate: 0.01,
          max_rate: 0.1,
          rate: 0.05,
        },
        description: {
          moniker: "banana",
          identity: "papaya",
          website: "watermelon.com",
          security_contact: "info@watermelon.com",
          details: "We are the banana papaya validator",
        },
        pubkey: toBase64(new Uint8Array(32).fill(1)),
        min_self_delegation: "1",
        initial_delegation: stringToCoin("1uscrt"),
      },
      { gasLimit: 100_000 }
    );
    expect(tx.code).toBe(TxResultCode.Success);
  }
});

test("/cosmos/base/node/v1beta1/config", async () => {
  const { minimum_gas_price } = await accounts[0].secretjs.query.node.config(
    {}
  );

  expect(minimum_gas_price).toBe("0.1uscrt");
});

describe("BankMsg", () => {
  describe("Send", () => {
    test("v1", async () => {
      const tx = await accounts[0].secretjs.tx.compute.executeContract(
        {
          sender: accounts[0].address,
          contract_address: contracts["secretdev-1"].v1.address,
          code_hash: contracts["secretdev-1"].v1.codeHash,
          msg: {
            bank_msg_send: {
              to_address: accounts[1].address,
              amount: stringToCoins("1uscrt"),
            },
          },
          sent_funds: stringToCoins("1uscrt"),
        },
        { gasLimit: 250_000 }
      );
      if (tx.code !== TxResultCode.Success) {
        console.error(tx.rawLog);
      }
      expect(tx.code).toBe(TxResultCode.Success);
      expect(tx.arrayLog.filter((x) => x.type === "coin_spent")).toStrictEqual([
        {
          key: "spender",
          msg: 0,
          type: "coin_spent",
          value: accounts[0].address,
        },
        { key: "amount", msg: 0, type: "coin_spent", value: "1uscrt" },
        {
          key: "spender",
          msg: 0,
          type: "coin_spent",
          value: contracts["secretdev-1"].v1.address,
        },
        { key: "amount", msg: 0, type: "coin_spent", value: "1uscrt" },
      ]);
      expect(
        tx.arrayLog.filter((x) => x.type === "coin_received")
      ).toStrictEqual([
        {
          key: "receiver",
          msg: 0,
          type: "coin_received",
          value: contracts["secretdev-1"].v1.address,
        },
        { key: "amount", msg: 0, type: "coin_received", value: "1uscrt" },
        {
          key: "receiver",
          msg: 0,
          type: "coin_received",
          value: accounts[1].address,
        },
        { key: "amount", msg: 0, type: "coin_received", value: "1uscrt" },
      ]);
    });

    describe("v0.10", () => {
      test("success", async () => {
        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v010.address,
            code_hash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              bank_msg_send: {
                to_address: accounts[1].address,
                amount: stringToCoins("1uscrt"),
              },
            },
            sent_funds: stringToCoins("1uscrt"),
          },
          { gasLimit: 250_000 }
        );
        if (tx.code !== TxResultCode.Success) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(TxResultCode.Success);
        expect(
          tx.arrayLog.filter((x) => x.type === "coin_spent")
        ).toStrictEqual([
          {
            key: "spender",
            msg: 0,
            type: "coin_spent",
            value: accounts[0].address,
          },
          { key: "amount", msg: 0, type: "coin_spent", value: "1uscrt" },
          {
            key: "spender",
            msg: 0,
            type: "coin_spent",
            value: contracts["secretdev-1"].v010.address,
          },
          { key: "amount", msg: 0, type: "coin_spent", value: "1uscrt" },
        ]);
        expect(
          tx.arrayLog.filter((x) => x.type === "coin_received")
        ).toStrictEqual([
          {
            key: "receiver",
            msg: 0,
            type: "coin_received",
            value: contracts["secretdev-1"].v010.address,
          },
          { key: "amount", msg: 0, type: "coin_received", value: "1uscrt" },
          {
            key: "receiver",
            msg: 0,
            type: "coin_received",
            value: accounts[1].address,
          },
          { key: "amount", msg: 0, type: "coin_received", value: "1uscrt" },
        ]);
      });

      test("error", async () => {
        const { balance } = await readonly.query.bank.balance({
          address: contracts["secretdev-1"].v010.address,
          denom: "uscrt",
        });
        const contractBalance = Number(balance?.amount) ?? 0;

        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v010.address,
            code_hash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              bank_msg_send: {
                to_address: accounts[1].address,
                amount: stringToCoins(`${contractBalance + 1}uscrt`),
              },
            },
          },
          { gasLimit: 250_000 }
        );

        expect(tx.code).toBe(TxResultCode.ErrInsufficientFunds);
        expect(tx.rawLog).toContain(
          `${contractBalance}uscrt is smaller than ${contractBalance + 1}uscrt`
        );
      });
    });
  });

  describe("Burn", () => {
    test("always_fails", async () => {
      const tx = await accounts[0].secretjs.tx.compute.executeContract(
        {
          sender: accounts[0].address,
          contract_address: contracts["secretdev-1"].v1.address,
          code_hash: contracts["secretdev-1"].v1.codeHash,
          msg: {
            bank_msg_burn: {
              amount: stringToCoins("100000000uscrt"),
            },
          },
        },
        { gasLimit: 250_000 }
      );
      expect(tx.code).toBe(TxResultCode.ErrInvalidCoins);
      expect(tx.rawLog).toContain("Unknown variant of Bank");
    });
  });
});

describe("Env", () => {
  describe("TransactionInfo", () => {
    describe("TxCount", () => {
      test("execute", async () => {
        jest.setTimeout(10 * 60 * 1_000);
        let txProm: Promise<TxResponse>[] = new Array(2);
        let success: boolean;
        let shouldBreak: boolean = false;
        for (let j = 0; j < 20 && !shouldBreak; j += 2) {
          for (let i = 0; i < 2; i++) {
            let walletID = j + i + 3;
            success = true;

            txProm[i] = accounts[walletID].secretjs.tx.compute.executeContract(
              {
                sender: accounts[walletID].address,
                contract_address: contracts["secretdev-1"].v1.address,
                code_hash: contracts["secretdev-1"].v1.codeHash,
                msg: {
                  get_tx_id: {},
                },
              },
              { gasLimit: 250_000 }
            );
          }

          let txs = await Promise.all(txProm);

          for (let i = 0; i < 2; i++) {
            if (txs[i].code !== TxResultCode.Success) {
              console.error(txs[i].rawLog);
            }

            expect(txs[i].code).toBe(TxResultCode.Success);

            const { attributes } = txs[i].jsonLog[0].events.find(
              (x) => x.type === "wasm-count"
            );

            expect(attributes.length).toBe(2);

            const { value } = attributes.find((x) => x.key === "count-val");

            if (value !== i.toString()) {
              success = false;
              break;
            }
          }

          if (success) {
            break;
          }
        }

        expect(success).toBe(true);
      });
    });
  });
});

describe("CustomMsg", () => {
  test("v1", async () => {
    const tx = await accounts[0].secretjs.tx.compute.executeContract(
      {
        sender: accounts[0].address,
        contract_address: contracts["secretdev-1"].v1.address,
        code_hash: contracts["secretdev-1"].v1.codeHash,
        msg: {
          custom_msg: {},
        },
      },
      { gasLimit: 250_000 }
    );
    if (tx.code !== 10) {
      console.error(tx.rawLog);
    }
    expect(tx.code).toBe(10 /* WASM ErrInvalidMsg */);
    expect(tx.rawLog).toContain("invalid CosmosMsg from the contract");
  });

  test("v0.10", async () => {
    const tx = await accounts[0].secretjs.tx.compute.executeContract(
      {
        sender: accounts[0].address,
        contract_address: contracts["secretdev-1"].v010.address,
        code_hash: contracts["secretdev-1"].v010.codeHash,
        msg: {
          custom_msg: {},
        },
      },
      { gasLimit: 250_000 }
    );
    if (tx.code !== 10) {
      console.error(tx.rawLog);
    }
    expect(tx.code).toBe(10 /* WASM ErrInvalidMsg */);
    expect(tx.rawLog).toContain("invalid CosmosMsg from the contract");
  });
});

describe("tx broadcast multi", () => {
  test("Send Multiple Messages Amino", async () => {
    const { validators } = await readonly.query.staking.validators({});
    const validator = validators[0].operator_address;

    let tx = await accounts[0].secretjs.tx.broadcast(
      [
        new MsgSendMsg({
          from_address: accounts[0].address,
          to_address: accounts[0].address,
          amount: stringToCoins("1uscrt"),
        }),

        new MsgExecuteContract({
          sender: accounts[0].address,
          contract_address: contracts["secretdev-1"].v1.address,
          code_hash: contracts["secretdev-1"].v1.codeHash,
          msg: {
            staking_msg_delegate: {
              validator: validator,
              amount: stringToCoin("1uscrt"),
            },
          },
          sent_funds: stringToCoins("1uscrt"),
        }),
      ],
      {
        broadcastCheckIntervalMs: 100,
        gasLimit: 5_000_000,
      }
    );
    if (tx.code !== TxResultCode.Success) {
      console.error(tx.rawLog);
    }

    expect(tx.code).toBe(TxResultCode.Success);
  });
});

describe("GovMsgVote", () => {
  let proposalId: number;

  beforeAll(async () => {
    let tx = await accounts[0].secretjs.tx.gov.submitProposal(
      {
        type: ProposalType.TextProposal,
        proposer: accounts[0].address,
        // on localsecret min deposit is 10 SCRT
        initial_deposit: stringToCoins("10000000uscrt"),
        content: {
          title: "Hi",
          description: "Hello",
        },
      },
      {
        broadcastCheckIntervalMs: 100,
        gasLimit: 5_000_000,
      }
    );
    if (tx.code !== TxResultCode.Success) {
      console.error(tx.rawLog);
    }
    expect(tx.code).toBe(TxResultCode.Success);

    proposalId = Number(
      tx.jsonLog?.[0].events
        .find((e) => e.type === "submit_proposal")
        ?.attributes.find((a) => a.key === "proposal_id")?.value
    );
    expect(proposalId).toBeGreaterThanOrEqual(1);
  });

  describe("v0.10", () => {
    test("success", async () => {
      const tx = await accounts[0].secretjs.tx.compute.executeContract(
        {
          sender: accounts[0].address,
          contract_address: contracts["secretdev-1"].v010.address,
          code_hash: contracts["secretdev-1"].v010.codeHash,
          msg: {
            gov_msg_vote: {
              proposal: proposalId,
              vote_option: "Yes",
            },
          },
        },
        { gasLimit: 250_000 }
      );
      if (tx.code !== TxResultCode.Success) {
        console.error(tx.rawLog);
      }
      expect(tx.code).toBe(TxResultCode.Success);

      const { attributes } = tx.jsonLog[0].events.find(
        (x) => x.type === "proposal_vote"
      );
      expect(attributes).toContainEqual({
        key: "proposal_id",
        value: String(proposalId),
      });
      expect(attributes).toContainEqual({
        key: "option",
        value: '{"option":1,"weight":"1.000000000000000000"}',
      });
    });

    test("error", async () => {
      const tx = await accounts[0].secretjs.tx.compute.executeContract(
        {
          sender: accounts[0].address,
          contract_address: contracts["secretdev-1"].v010.address,
          code_hash: contracts["secretdev-1"].v010.codeHash,
          msg: {
            gov_msg_vote: {
              proposal: proposalId + 1e6,
              vote_option: "Yes",
            },
          },
        },
        { gasLimit: 250_000 }
      );

      expect(tx.code).toBe(2 /* Gov ErrUnknownProposal */);
      expect(tx.rawLog).toContain(`${proposalId + 1e6}: unknown proposal`);
    });
  });

  describe("v1", () => {
    test("success", async () => {
      const tx = await accounts[0].secretjs.tx.compute.executeContract(
        {
          sender: accounts[0].address,
          contract_address: contracts["secretdev-1"].v1.address,
          code_hash: contracts["secretdev-1"].v1.codeHash,
          msg: {
            gov_msg_vote: {
              proposal: proposalId,
              vote_option: "yes",
            },
          },
        },
        { gasLimit: 250_000 }
      );
      if (tx.code !== TxResultCode.Success) {
        console.error(tx.rawLog);
      }
      expect(tx.code).toBe(TxResultCode.Success);

      const { attributes } = tx.jsonLog[0].events.find(
        (x) => x.type === "proposal_vote"
      );
      expect(attributes).toContainEqual({
        key: "proposal_id",
        value: String(proposalId),
      });
      expect(attributes).toContainEqual({
        key: "option",
        value: '{"option":1,"weight":"1.000000000000000000"}',
      });
    });

    test("error", async () => {
      const tx = await accounts[0].secretjs.tx.compute.executeContract(
        {
          sender: accounts[0].address,
          contract_address: contracts["secretdev-1"].v1.address,
          code_hash: contracts["secretdev-1"].v1.codeHash,
          msg: {
            gov_msg_vote: {
              proposal: proposalId + 1e6,
              vote_option: "yes",
            },
          },
        },
        { gasLimit: 250_000 }
      );

      expect(tx.code).toBe(2 /* Gov ErrUnknownProposal */);
      expect(tx.rawLog).toContain(`${proposalId + 1e6}: unknown proposal`);
    });
  });
});

describe("Wasm", () => {
  describe("MsgInstantiateContract", () => {
    describe("v1", () => {
      test("success", async () => {
        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v1.address,
            code_hash: contracts["secretdev-1"].v1.codeHash,
            msg: {
              wasm_msg_instantiate: {
                code_id: contracts["secretdev-1"].v1.codeId,
                code_hash: contracts["secretdev-1"].v1.codeHash,
                msg: toBase64(toUtf8(JSON.stringify({ nop: {} }))),
                funds: [],
                label: `v1-${Date.now()}`,
              },
            },
          },
          { gasLimit: 250_000 }
        );

        if (tx.code !== TxResultCode.Success) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(TxResultCode.Success);

        const { attributes } = tx.jsonLog[0].events.find(
          (e) => e.type === "wasm"
        );
        expect(attributes.length).toBe(2);
        expect(attributes[0].key).toBe("contract_address");
        expect(attributes[0].value).toBe(contracts["secretdev-1"].v1.address);
        expect(attributes[1].key).toBe("contract_address");
        expect(attributes[1].value).not.toBe(
          contracts["secretdev-1"].v1.address
        );
      });

      test("error", async () => {
        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v1.address,
            code_hash: contracts["secretdev-1"].v1.codeHash,
            msg: {
              wasm_msg_instantiate: {
                code_id: contracts["secretdev-1"].v1.codeId,
                code_hash: contracts["secretdev-1"].v1.codeHash,
                msg: toBase64(toUtf8(JSON.stringify({ blabla: {} }))),
                funds: [],
                label: `v1-${Date.now()}`,
              },
            },
          },
          { gasLimit: 250_000 }
        );

        if (tx.code !== 2) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(2 /* WASM ErrInstantiateFailed */);

        expect(tx.rawLog).toContain("encrypted:");
        expect(tx.rawLog).toContain("instantiate contract failed");
      });
    });

    describe("v0.10", () => {
      test("success", async () => {
        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v010.address,
            code_hash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              wasm_msg_instantiate: {
                code_id: contracts["secretdev-1"].v010.codeId,
                callback_code_hash: contracts["secretdev-1"].v010.codeHash,
                msg: toBase64(toUtf8(JSON.stringify({ echo: {} }))),
                send: [],
                label: `v010-${Date.now()}`,
              },
            },
          },
          { gasLimit: 250_000 }
        );

        if (tx.code !== TxResultCode.Success) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(TxResultCode.Success);

        const { attributes } = tx.jsonLog[0].events.find(
          (e) => e.type === "wasm"
        );
        expect(attributes.length).toBe(2);
        expect(attributes[0].key).toBe("contract_address");
        expect(attributes[0].value).toBe(contracts["secretdev-1"].v010.address);
        expect(attributes[1].key).toBe("contract_address");
        expect(attributes[1].value).not.toBe(
          contracts["secretdev-1"].v010.address
        );
      });

      test("error", async () => {
        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v010.address,
            code_hash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              wasm_msg_instantiate: {
                code_id: contracts["secretdev-1"].v010.codeId,
                callback_code_hash: contracts["secretdev-1"].v010.codeHash,
                msg: toBase64(toUtf8(JSON.stringify({ blabla: {} }))),
                send: [],
                label: `v010-${Date.now()}`,
              },
            },
          },
          { gasLimit: 250_000 }
        );

        if (tx.code !== 2) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(2 /* WASM ErrInstantiateFailed */);

        expect(tx.rawLog).toContain("encrypted:");
        expect(tx.rawLog).toContain("instantiate contract failed");
      });
    });
  });

  describe("MsgExecuteContract", () => {
    describe("v1", () => {
      test("success", async () => {
        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v1.address,
            code_hash: contracts["secretdev-1"].v1.codeHash,
            msg: {
              wasm_msg_execute: {
                contract_addr: contracts["secretdev-1"].v1.address,
                code_hash: contracts["secretdev-1"].v1.codeHash,
                msg: toBase64(toUtf8(JSON.stringify({ nop: {} }))),
                funds: [],
              },
            },
          },
          { gasLimit: 250_000 }
        );

        if (tx.code !== TxResultCode.Success) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(TxResultCode.Success);

        const { attributes } = tx.jsonLog[0].events.find(
          (e) => e.type === "wasm"
        );
        expect(attributes.length).toBe(2);
        expect(attributes[0].key).toBe("contract_address");
        expect(attributes[0].value).toBe(contracts["secretdev-1"].v1.address);
        expect(attributes[1].key).toBe("contract_address");
        expect(attributes[1].value).toBe(contracts["secretdev-1"].v1.address);
      });

      test("error", async () => {
        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v1.address,
            code_hash: contracts["secretdev-1"].v1.codeHash,
            msg: {
              wasm_msg_execute: {
                contract_addr: contracts["secretdev-1"].v1.address,
                code_hash: contracts["secretdev-1"].v1.codeHash,
                msg: toBase64(toUtf8(JSON.stringify({ blabla: {} }))),
                funds: [],
              },
            },
          },
          { gasLimit: 250_000 }
        );

        if (tx.code !== 3) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(3 /* WASM ErrExecuteFailed */);

        expect(tx.rawLog).toContain("encrypted:");
        expect(tx.rawLog).toContain("execute contract failed");
      });
    });

    describe("v0.10", () => {
      test("success", async () => {
        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v010.address,
            code_hash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              wasm_msg_execute: {
                contract_addr: contracts["secretdev-1"].v010.address,
                callback_code_hash: contracts["secretdev-1"].v010.codeHash,
                msg: toBase64(toUtf8(JSON.stringify({ echo: {} }))),
                send: [],
              },
            },
          },
          { gasLimit: 250_000 }
        );

        if (tx.code !== TxResultCode.Success) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(TxResultCode.Success);

        const { attributes } = tx.jsonLog[0].events.find(
          (e) => e.type === "wasm"
        );
        expect(attributes.length).toBe(2);
        expect(attributes[0].key).toBe("contract_address");
        expect(attributes[0].value).toBe(contracts["secretdev-1"].v010.address);
        expect(attributes[1].key).toBe("contract_address");
        expect(attributes[1].value).toBe(contracts["secretdev-1"].v010.address);
      });

      test("error", async () => {
        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v010.address,
            code_hash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              wasm_msg_execute: {
                contract_addr: contracts["secretdev-1"].v010.address,
                callback_code_hash: contracts["secretdev-1"].v010.codeHash,
                msg: toBase64(toUtf8(JSON.stringify({ blabla: {} }))),
                send: [],
              },
            },
          },
          { gasLimit: 250_000 }
        );

        if (tx.code !== 3) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(3 /* WASM ErrExecuteFailed */);

        expect(tx.rawLog).toContain("encrypted:");
        expect(tx.rawLog).toContain("execute contract failed");
      });
    });
  });
});

describe("StakingMsg", () => {
  describe("Delegate", () => {
    describe("v1", () => {
      test("success", async () => {
        const [delegationStatus, tx, validator] = await delegate_for_test(
          accounts[0],
          contracts["secretdev-1"].v1
        );
        expect(delegationStatus).toBeTruthy();

        const { attributes } = tx.jsonLog[0].events.find(
          (e) => e.type === "delegate"
        );
        expect(attributes).toContainEqual({ key: "amount", value: "1uscrt" });
        expect(attributes).toContainEqual({
          key: "validator",
          value: validator,
        });
      });

      test("error", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validator = validators[0].operator_address;

        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v1.address,
            code_hash: contracts["secretdev-1"].v1.codeHash,
            msg: {
              staking_msg_delegate: {
                validator: validator + "garbage",
                amount: stringToCoin("1uscrt"),
              },
            },
            sent_funds: stringToCoins("1uscrt"),
          },
          { gasLimit: 250_000 }
        );

        expect(tx.code).toBe(TxResultCode.ErrInvalidAddress);
        expect(tx.rawLog).toContain(
          `${validator + "garbage"}: invalid address`
        );
      });
    });

    describe("v0.10", () => {
      test("success", async () => {
        const [delegationStatus, tx, validator] = await delegate_for_test(
          accounts[0],
          contracts["secretdev-1"].v010
        );
        expect(delegationStatus).toBeTruthy();

        const { attributes } = tx.jsonLog[0].events.find(
          (e) => e.type === "delegate"
        );
        expect(attributes).toContainEqual({ key: "amount", value: "1uscrt" });
        expect(attributes).toContainEqual({
          key: "validator",
          value: validator,
        });
      });

      test("error", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validator = validators[0].operator_address;

        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v010.address,
            code_hash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              staking_msg_delegate: {
                validator: validator + "garbage",
                amount: stringToCoin("1uscrt"),
              },
            },
            sent_funds: stringToCoins("1uscrt"),
          },
          { gasLimit: 250_000 }
        );

        expect(tx.code).toBe(TxResultCode.ErrInvalidAddress);
        expect(tx.rawLog).toContain(
          `${validator + "garbage"}: invalid address`
        );
      });
    });
  });

  describe("Undelegate", () => {
    test("success", async () => {
      const { validators } = await readonly.query.staking.validators({});
      const validator = validators[0].operator_address;

      const tx = await accounts[0].secretjs.tx.broadcast(
        [
          new MsgExecuteContract({
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v1.address,
            code_hash: contracts["secretdev-1"].v1.codeHash,
            msg: {
              staking_msg_delegate: {
                validator,
                amount: stringToCoin("1uscrt"),
              },
            },
            sent_funds: stringToCoins("1uscrt"),
          }),
          new MsgExecuteContract({
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v1.address,
            code_hash: contracts["secretdev-1"].v1.codeHash,
            msg: {
              staking_msg_undelegate: {
                validator,
                amount: stringToCoin("1uscrt"),
              },
            },
            sent_funds: stringToCoins("1uscrt"),
          }),
        ],
        { gasLimit: 350_000 }
      );
      if (tx.code !== TxResultCode.Success) {
        console.error(tx.rawLog);
      }
      expect(tx.code).toBe(TxResultCode.Success);

      const { attributes } = tx.jsonLog[1].events.find(
        (e) => e.type === "unbond"
      );
      expect(attributes).toContainEqual({ key: "amount", value: "1uscrt" });
      expect(attributes).toContainEqual({
        key: "validator",
        value: validator,
      });
    });

    test("error", async () => {
      const { validators } = await readonly.query.staking.validators({});
      const validator = validators[0].operator_address;

      const tx = await accounts[0].secretjs.tx.compute.executeContract(
        {
          sender: accounts[0].address,
          contract_address: contracts["secretdev-1"].v010.address,
          code_hash: contracts["secretdev-1"].v010.codeHash,
          msg: {
            staking_msg_undelegate: {
              validator: validator + "garbage",
              amount: stringToCoin("1uscrt"),
            },
          },
          sent_funds: stringToCoins("1uscrt"),
        },
        { gasLimit: 250_000 }
      );

      expect(tx.code).toBe(TxResultCode.ErrInvalidAddress);
      expect(tx.rawLog).toContain(`${validator + "garbage"}: invalid address`);
    });

    describe("v0.10", () => {
      test("success", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validator = validators[0].operator_address;

        const tx = await accounts[0].secretjs.tx.broadcast(
          [
            new MsgExecuteContract({
              sender: accounts[0].address,
              contract_address: contracts["secretdev-1"].v010.address,
              code_hash: contracts["secretdev-1"].v010.codeHash,
              msg: {
                staking_msg_delegate: {
                  validator,
                  amount: stringToCoin("1uscrt"),
                },
              },
              sent_funds: stringToCoins("1uscrt"),
            }),
            new MsgExecuteContract({
              sender: accounts[0].address,
              contract_address: contracts["secretdev-1"].v010.address,
              code_hash: contracts["secretdev-1"].v010.codeHash,
              msg: {
                staking_msg_undelegate: {
                  validator,
                  amount: stringToCoin("1uscrt"),
                },
              },
              sent_funds: stringToCoins("1uscrt"),
            }),
          ],
          { gasLimit: 350_000 }
        );
        if (tx.code !== TxResultCode.Success) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(TxResultCode.Success);

        const { attributes } = tx.jsonLog[1].events.find(
          (e) => e.type === "unbond"
        );
        expect(attributes).toContainEqual({ key: "amount", value: "1uscrt" });
        expect(attributes).toContainEqual({
          key: "validator",
          value: validator,
        });
      });

      test("error", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validator = validators[0].operator_address;

        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v010.address,
            code_hash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              staking_msg_undelegate: {
                validator: validator + "garbage",
                amount: stringToCoin("1uscrt"),
              },
            },
            sent_funds: stringToCoins("1uscrt"),
          },
          { gasLimit: 250_000 }
        );

        expect(tx.code).toBe(TxResultCode.ErrInvalidAddress);
        expect(tx.rawLog).toContain(
          `${validator + "garbage"}: invalid address`
        );
      });
    });
  });

  describe("Redelegate", () => {
    describe("v1", () => {
      test("success", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validatorA = validators[0].operator_address;
        const validatorB = validators[1].operator_address;

        const tx = await accounts[0].secretjs.tx.broadcast(
          [
            new MsgExecuteContract({
              sender: accounts[0].address,
              contract_address: contracts["secretdev-1"].v1.address,
              code_hash: contracts["secretdev-1"].v1.codeHash,
              msg: {
                staking_msg_delegate: {
                  validator: validatorA,
                  amount: stringToCoin("1uscrt"),
                },
              },
              sent_funds: stringToCoins("1uscrt"),
            }),
            new MsgExecuteContract({
              sender: accounts[0].address,
              contract_address: contracts["secretdev-1"].v1.address,
              code_hash: contracts["secretdev-1"].v1.codeHash,
              msg: {
                staking_msg_redelegate: {
                  src_validator: validatorA,
                  dst_validator: validatorB,
                  amount: stringToCoin("1uscrt"),
                },
              },
              sent_funds: stringToCoins("1uscrt"),
            }),
          ],
          { gasLimit: 350_000 }
        );
        if (tx.code !== TxResultCode.Success) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(TxResultCode.Success);

        const { attributes } = tx.jsonLog[1].events.find(
          (e) => e.type === "redelegate"
        );
        expect(attributes).toContainEqual({ key: "amount", value: "1uscrt" });
        expect(attributes).toContainEqual({
          key: "source_validator",
          value: validatorA,
        });
        expect(attributes).toContainEqual({
          key: "destination_validator",
          value: validatorB,
        });
      });

      test("error", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validator = validators[0].operator_address;

        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v1.address,
            code_hash: contracts["secretdev-1"].v1.codeHash,
            msg: {
              staking_msg_redelegate: {
                src_validator: validator,
                dst_validator: validator + "garbage",
                amount: stringToCoin("1uscrt"),
              },
            },
            sent_funds: stringToCoins("1uscrt"),
          },
          { gasLimit: 250_000 }
        );

        expect(tx.code).toBe(TxResultCode.ErrInvalidAddress);
        expect(tx.rawLog).toContain(
          `${validator + "garbage"}: invalid address`
        );
      });
    });

    describe("v0.10", () => {
      test("success", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validatorA = validators[0].operator_address;
        const validatorB = validators[1].operator_address;

        const tx = await accounts[0].secretjs.tx.broadcast(
          [
            new MsgExecuteContract({
              sender: accounts[0].address,
              contract_address: contracts["secretdev-1"].v010.address,
              code_hash: contracts["secretdev-1"].v010.codeHash,
              msg: {
                staking_msg_delegate: {
                  validator: validatorA,
                  amount: stringToCoin("1uscrt"),
                },
              },
              sent_funds: stringToCoins("1uscrt"),
            }),
            new MsgExecuteContract({
              sender: accounts[0].address,
              contract_address: contracts["secretdev-1"].v010.address,
              code_hash: contracts["secretdev-1"].v010.codeHash,
              msg: {
                staking_msg_redelegate: {
                  src_validator: validatorA,
                  dst_validator: validatorB,
                  amount: stringToCoin("1uscrt"),
                },
              },
              sent_funds: stringToCoins("1uscrt"),
            }),
          ],
          { gasLimit: 350_000 }
        );
        if (tx.code !== TxResultCode.Success) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(TxResultCode.Success);

        const { attributes } = tx.jsonLog[1].events.find(
          (e) => e.type === "redelegate"
        );
        expect(attributes).toContainEqual({ key: "amount", value: "1uscrt" });
        expect(attributes).toContainEqual({
          key: "source_validator",
          value: validatorA,
        });
        expect(attributes).toContainEqual({
          key: "destination_validator",
          value: validatorB,
        });
      });

      test("error", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validator = validators[0].operator_address;

        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v010.address,
            code_hash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              staking_msg_redelegate: {
                src_validator: validator,
                dst_validator: validator + "garbage",
                amount: stringToCoin("1uscrt"),
              },
            },
            sent_funds: stringToCoins("1uscrt"),
          },
          { gasLimit: 250_000 }
        );

        expect(tx.code).toBe(TxResultCode.ErrInvalidAddress);
        expect(tx.rawLog).toContain(
          `${validator + "garbage"}: invalid address`
        );
      });
    });
  });

  describe("Withdraw", () => {
    describe("v1", () => {
      test("success", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validator = validators[0].operator_address;

        const tx = await accounts[0].secretjs.tx.broadcast(
          [
            new MsgExecuteContract({
              sender: accounts[0].address,
              contract_address: contracts["secretdev-1"].v1.address,
              code_hash: contracts["secretdev-1"].v1.codeHash,
              msg: {
                staking_msg_delegate: {
                  validator: validator,
                  amount: stringToCoin("1uscrt"),
                },
              },
              sent_funds: stringToCoins("1uscrt"),
            }),
            new MsgExecuteContract({
              sender: accounts[0].address,
              contract_address: contracts["secretdev-1"].v1.address,
              code_hash: contracts["secretdev-1"].v1.codeHash,
              msg: {
                staking_msg_withdraw: {
                  validator: validator,
                },
              },
              sent_funds: stringToCoins("1uscrt"),
            }),
          ],
          { gasLimit: 250_000 }
        );
        if (tx.code !== TxResultCode.Success) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(TxResultCode.Success);

        const { attributes } = tx.jsonLog[1].events.find(
          (e) => e.type === "withdraw_rewards"
        );
        expect(attributes).toContainEqual({
          key: "validator",
          value: validator,
        });
      });

      test("set_withdraw_address", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validator = validators[0].operator_address;

        const tx = await accounts[0].secretjs.tx.broadcast(
          [
            new MsgExecuteContract({
              sender: accounts[0].address,
              contract_address: contracts["secretdev-1"].v1.address,
              code_hash: contracts["secretdev-1"].v1.codeHash,
              msg: {
                staking_msg_delegate: {
                  validator: validator,
                  amount: stringToCoin("1uscrt"),
                },
              },
              sent_funds: stringToCoins("1uscrt"),
            }),
            new MsgExecuteContract({
              sender: accounts[0].address,
              contract_address: contracts["secretdev-1"].v1.address,
              code_hash: contracts["secretdev-1"].v1.codeHash,
              msg: {
                set_withdraw_address: {
                  address: accounts[1].address,
                },
              },
              sent_funds: stringToCoins("1uscrt"),
            }),
          ],
          { gasLimit: 250_000 }
        );
        if (tx.code !== TxResultCode.Success) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(TxResultCode.Success);

        const { attributes } = tx.jsonLog[1].events.find(
          (e) => e.type === "set_withdraw_address"
        );
        expect(attributes).toContainEqual({
          key: "withdraw_address",
          value: accounts[1].address,
        });
      });

      test("error", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validator = validators[0].operator_address;

        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v1.address,
            code_hash: contracts["secretdev-1"].v1.codeHash,
            msg: {
              staking_msg_withdraw: {
                validator: validator + "garbage",
                recipient: accounts[0].address,
              },
            },
            sent_funds: stringToCoins("1uscrt"),
          },
          { gasLimit: 250_000 }
        );

        expect(tx.code).toBe(TxResultCode.ErrInternal);
      });
    });

    describe("v0.10", () => {
      test("success", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validator = validators[0].operator_address;

        const tx = await accounts[0].secretjs.tx.broadcast(
          [
            new MsgExecuteContract({
              sender: accounts[0].address,
              contract_address: contracts["secretdev-1"].v010.address,
              code_hash: contracts["secretdev-1"].v010.codeHash,
              msg: {
                staking_msg_delegate: {
                  validator: validator,
                  amount: stringToCoin("1uscrt"),
                },
              },
              sent_funds: stringToCoins("1uscrt"),
            }),
            new MsgExecuteContract({
              sender: accounts[0].address,
              contract_address: contracts["secretdev-1"].v010.address,
              code_hash: contracts["secretdev-1"].v010.codeHash,
              msg: {
                staking_msg_withdraw: {
                  validator: validator,
                  recipient: accounts[0].address,
                },
              },
              sent_funds: stringToCoins("1uscrt"),
            }),
          ],
          { gasLimit: 250_000 }
        );
        if (tx.code !== TxResultCode.Success) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(TxResultCode.Success);

        const { attributes } = tx.jsonLog[1].events.find(
          (e) => e.type === "withdraw_rewards"
        );
        expect(attributes).toContainEqual({
          key: "validator",
          value: validator,
        });
      });

      test("error", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validator = validators[0].operator_address;

        const tx = await accounts[0].secretjs.tx.compute.executeContract(
          {
            sender: accounts[0].address,
            contract_address: contracts["secretdev-1"].v010.address,
            code_hash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              staking_msg_withdraw: {
                validator: validator + "garbage",
                recipient: accounts[0].address,
              },
            },
            sent_funds: stringToCoins("1uscrt"),
          },
          { gasLimit: 250_000 }
        );

        expect(tx.code).toBe(TxResultCode.ErrInvalidAddress);
        expect(tx.rawLog).toContain(
          `${validator + "garbage"}: invalid address`
        );
      });
    });
  });
});

describe("StargateMsg", () => {
  test("v1", async () => {
    const tx = await accounts[0].secretjs.tx.compute.executeContract(
      {
        sender: accounts[0].address,
        contract_address: contracts["secretdev-1"].v1.address,
        code_hash: contracts["secretdev-1"].v1.codeHash,
        msg: {
          stargate_msg: {
            type_url: "/cosmos.bank.v1beta1.MsgSend",
            value: toBase64(
              MsgSend.encode({
                from_address: contracts["secretdev-1"].v1.address,
                to_address: accounts[1].address,
                amount: stringToCoins("1uscrt"),
              }).finish()
            ),
          },
        },
        sent_funds: stringToCoins("1uscrt"),
      },
      { gasLimit: 250_000 }
    );
    if (tx.code !== TxResultCode.Success) {
      console.error(tx.rawLog);
    }
    expect(tx.code).toBe(TxResultCode.Success);
    expect(tx.arrayLog.filter((x) => x.type === "coin_spent")).toStrictEqual([
      {
        key: "spender",
        msg: 0,
        type: "coin_spent",
        value: accounts[0].address,
      },
      { key: "amount", msg: 0, type: "coin_spent", value: "1uscrt" },
      {
        key: "spender",
        msg: 0,
        type: "coin_spent",
        value: contracts["secretdev-1"].v1.address,
      },
      { key: "amount", msg: 0, type: "coin_spent", value: "1uscrt" },
    ]);
    expect(tx.arrayLog.filter((x) => x.type === "coin_received")).toStrictEqual(
      [
        {
          key: "receiver",
          msg: 0,
          type: "coin_received",
          value: contracts["secretdev-1"].v1.address,
        },
        { key: "amount", msg: 0, type: "coin_received", value: "1uscrt" },
        {
          key: "receiver",
          msg: 0,
          type: "coin_received",
          value: accounts[1].address,
        },
        { key: "amount", msg: 0, type: "coin_received", value: "1uscrt" },
      ]
    );
  });
});

describe("StargateQuery", () => {
  test("v1", async () => {
    const result: any = await readonly.query.compute.queryContract({
      contract_address: contracts["secretdev-1"].v1.address,
      code_hash: contracts["secretdev-1"].v1.codeHash,
      query: {
        stargate: {
          path: "/cosmos.bank.v1beta1.Query/Balance",
          data: toBase64(
            QueryBalanceRequest.encode({
              address: accounts[0].address,
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
      const result: any = await readonly.query.compute.queryContract({
        contract_address: contracts["secretdev-1"].v1.address,
        code_hash: contracts["secretdev-1"].v1.codeHash,
        query: {
          bank_balance: {
            address: accounts[0].address,
            denom: "uscrt",
          },
        },
      });
      expect(result?.amount?.denom).toBe("uscrt");
      expect(Number(result?.amount?.amount)).toBeGreaterThanOrEqual(1);
    });

    test("v0.10", async () => {
      const result: any = await readonly.query.compute.queryContract({
        contract_address: contracts["secretdev-1"].v010.address,
        code_hash: contracts["secretdev-1"].v010.codeHash,
        query: {
          bank_balance: {
            address: accounts[0].address,
            denom: "uscrt",
          },
        },
      });
      expect(result?.amount?.denom).toBe("uscrt");
      expect(Number(result?.amount?.amount)).toBeGreaterThanOrEqual(1);
    });
  });

  describe("AllBalances", () => {
    test("v1", async () => {
      const result: any = await readonly.query.compute.queryContract({
        contract_address: contracts["secretdev-1"].v1.address,
        code_hash: contracts["secretdev-1"].v1.codeHash,
        query: {
          bank_all_balances: {
            address: accounts[0].address,
          },
        },
      });

      expect(result?.amount?.length).toBe(1);
      expect(result?.amount[0]?.denom).toBe("uscrt");
      expect(Number(result?.amount[0]?.amount)).toBeGreaterThanOrEqual(1);
    });
  });
});

async function delegate_for_test(
  account: Account,
  contract: Contract
): Promise<[boolean, TxResponse, string]> {
  const { validators } = await readonly.query.staking.validators({});
  const validator = validators[0].operator_address;

  const tx = await account.secretjs.tx.compute.executeContract(
    {
      sender: account.address,
      contract_address: contract.address,
      code_hash: contract.codeHash,
      msg: {
        staking_msg_delegate: {
          validator,
          amount: stringToCoin("1uscrt"),
        },
      },
      sent_funds: stringToCoins("1uscrt"),
    },
    { gasLimit: 250_000 }
  );
  if (tx.code !== TxResultCode.Success) {
    console.error(tx.rawLog);
  }

  return [tx.code === TxResultCode.Success, tx, validator];
}

async function undelegate_for_test(
  account: Account,
  contract: Contract,
  validator: string
): Promise<boolean> {
  const tx = await account.secretjs.tx.compute.executeContract(
    {
      sender: account.address,
      contract_address: contract.address,
      code_hash: contract.codeHash,
      msg: {
        staking_msg_undelegate: {
          validator,
          amount: stringToCoin("1uscrt"),
        },
      },
      sent_funds: stringToCoins("1uscrt"),
    },
    { gasLimit: 250_000 }
  );
  if (tx.code !== TxResultCode.Success) {
    console.error(tx.rawLog);
  }

  return tx.code === TxResultCode.Success;
}

describe("StakingQuery", () => {
  describe("BondedDemon", () => {
    test("v1", async () => {
      const result: any = await readonly.query.compute.queryContract({
        contract_address: contracts["secretdev-1"].v1.address,
        code_hash: contracts["secretdev-1"].v1.codeHash,
        query: {
          staking_bonded_denom: {
            address: accounts[0].address,
            denom: "uscrt",
          },
        },
      });
      expect(result?.denom).toBe("uscrt");
    });
  });

  describe("AllDelegations", () => {
    test("v1", async () => {
      const [delegationStatus, _, validator] = await delegate_for_test(
        accounts[1],
        contracts["secretdev-1"].v1
      );
      expect(delegationStatus).toBeTruthy();
      const result: any = await readonly.query.compute.queryContract({
        contract_address: contracts["secretdev-1"].v1.address,
        code_hash: contracts["secretdev-1"].v1.codeHash,
        query: {
          staking_all_delegations: {
            delegator: accounts[1].address,
          },
        },
      });

      expect(result?.delegations?.length).toBe(1);
      expect(result?.delegations[0]?.delegator).toBe(accounts[1].address);
      expect(result?.delegations[0]?.validator).toBe(validator);
      expect(result?.delegations[0]?.amount?.denom).toBe("uscrt");
      expect(Number(result?.delegations[0]?.amount?.amount)).toBe(1);

      expect(
        await undelegate_for_test(
          accounts[1],
          contracts["secretdev-1"].v1,
          validator
        )
      ).toBeTruthy();
    });
  });

  describe("Delegation", () => {
    test("v1", async () => {
      const [delegationStatus, _, validator] = await delegate_for_test(
        accounts[1],
        contracts["secretdev-1"].v1
      );
      expect(delegationStatus).toBeTruthy();
      const result: any = await readonly.query.compute.queryContract({
        contract_address: contracts["secretdev-1"].v1.address,
        code_hash: contracts["secretdev-1"].v1.codeHash,
        query: {
          staking_delegation: {
            delegator: accounts[1].address,
            validator: validator,
          },
        },
      });

      expect(result?.delegation?.delegator).toBe(accounts[1].address);
      expect(result?.delegation?.validator).toBe(validator);
      expect(result?.delegation?.amount?.denom).toBe("uscrt");
      expect(Number(result?.delegation?.amount?.amount)).toBe(1);

      expect(
        await undelegate_for_test(
          accounts[1],
          contracts["secretdev-1"].v1,
          validator
        )
      ).toBeTruthy();
    });
  });

  describe("AllValidators", () => {
    test("v1", async () => {
      const result: any = await readonly.query.compute.queryContract({
        contract_address: contracts["secretdev-1"].v1.address,
        code_hash: contracts["secretdev-1"].v1.codeHash,
        query: {
          staking_all_validators: {},
        },
      });

      expect(result?.validators?.length).toBe(1);
    });
  });

  describe("Validator", () => {
    test("v1", async () => {
      const { validators } = await readonly.query.staking.validators({});
      const validator = validators[0].operator_address;

      const result: any = await readonly.query.compute.queryContract({
        contract_address: contracts["secretdev-1"].v1.address,
        code_hash: contracts["secretdev-1"].v1.codeHash,
        query: {
          staking_validator: { address: validator },
        },
      });

      expect(result?.validator?.address).toBe(validator);
    });
  });
});

describe("IBCQuery", () => {
  describe("PortID", () => {
    test("v1", async () => {
      const result: any = await readonly.query.compute.queryContract({
        contract_address: contracts["secretdev-1"].v1.address,
        code_hash: contracts["secretdev-1"].v1.codeHash,
        query: {
          ibc_port_id: {},
        },
      });
      expect(result?.port_id).toBe(
        "wasm." + contracts["secretdev-1"].v1.address
      );
    });
  });
});

describe("WasmQuery", () => {
  describe("Smart", () => {
    test("v1", async () => {
      const b64encode = (str: string): string =>
        Buffer.from(str, "binary").toString("base64");
      const result: any = await readonly.query.compute.queryContract({
        contract_address: contracts["secretdev-1"].v1.address,
        code_hash: contracts["secretdev-1"].v1.codeHash,
        query: {
          wasm_smart: {
            contract_addr: contracts["secretdev-1"].v010.address,
            code_hash: contracts["secretdev-1"].v010.codeHash,
            msg: b64encode(
              JSON.stringify({
                bank_balance: {
                  address: accounts[0].address,
                  denom: "uscrt",
                },
              })
            ),
          },
        },
      });

      expect(result?.amount?.denom).toBe("uscrt");
      expect(Number(result?.amount?.amount)).toBeGreaterThanOrEqual(1);
    });
  });

  describe("ContractInfo", () => {
    test("v1", async () => {
      const result: any = await readonly.query.compute.queryContract({
        contract_address: contracts["secretdev-1"].v1.address,
        code_hash: contracts["secretdev-1"].v1.codeHash,
        query: {
          wasm_contract_info: {
            contract_addr: contracts["secretdev-1"].v010.address,
          },
        },
      });

      expect(result?.code_id).toBe(contracts["secretdev-1"].v010.codeId);
      expect(result?.creator).toBe(accounts[0].address);
      expect(result?.pinned).toBe(false);
      expect(result?.ibc_port).toBe(null);
    });
  });
});

describe("IBC", () => {
  beforeAll(async () => {
    console.log("Storing contracts on secretdev-2...");

    let tx: TxResponse = await storeContracts(accounts2[0].secretjs, [
      v1Wasm,
      v010Wasm,
    ]);
    if (tx.code !== TxResultCode.Success) {
      console.error(tx.rawLog);
    }
    expect(tx.code).toBe(TxResultCode.Success);

    contracts["secretdev-2"].v1.codeId = Number(
      tx.arrayLog.find((x) => x.key === "code_id").value
    );
    contracts["secretdev-2"].v010.codeId = Number(
      tx.arrayLog.reverse().find((x) => x.key === "code_id").value
    );

    contracts["secretdev-2"].v1.codeHash = contracts["secretdev-1"].v1.codeHash;
    contracts["secretdev-2"].v010.codeHash =
      contracts["secretdev-1"].v010.codeHash;

    console.log("Instantiating contracts on secretdev-2...");

    tx = await instantiateContracts(accounts2[0].secretjs, [
      contracts["secretdev-2"].v1,
      contracts["secretdev-2"].v010,
    ]);
    if (tx.code !== TxResultCode.Success) {
      console.error(tx.rawLog);
    }
    expect(tx.code).toBe(TxResultCode.Success);

    contracts["secretdev-2"].v1.address = tx.arrayLog.find(
      (x) => x.key === "contract_address"
    ).value;
    contracts["secretdev-2"].v1.ibcPortId =
      "wasm." + contracts["secretdev-2"].v1.address;

    contracts["secretdev-2"].v010.address = tx.arrayLog
      .reverse()
      .find((x) => x.key === "contract_address").value;

    console.log("Waiting for IBC to set up...");
    await waitForIBCConnection("secretdev-1", "http://localhost:1317");
    await waitForIBCConnection("secretdev-2", "http://localhost:2317");

    await waitForIBCChannel(
      "secretdev-1",
      "http://localhost:1317",
      "channel-0"
    );
    await waitForIBCChannel(
      "secretdev-2",
      "http://localhost:2317",
      "channel-0"
    );
  }, 180_000 /* 3 minutes */);

  test("transfer sanity", async () => {
    const denom = ibcDenom(
      [
        {
          portId: "transfer",
          channelId: "channel-0",
        },
      ],
      "uscrt"
    );
    const { balance: balanceBefore } = await readonly2.query.bank.balance({
      address: accounts2[0].address,
      denom,
    });
    const amountBefore = Number(balanceBefore?.amount ?? "0");

    const result = await accounts[0].secretjs.tx.ibc.transfer({
      receiver: accounts[0].address,
      sender: accounts[0].address,
      source_channel: "channel-0",
      source_port: "transfer",
      token: stringToCoin("1uscrt"),
      timeout_timestamp: String(Math.floor(Date.now() / 1000 + 30)),
    });
    if (result.code !== TxResultCode.Success) {
      console.error(result.rawLog);
    }
    expect(result.code).toBe(TxResultCode.Success);

    // checking ack/timeout on secretdev-1 might be cleaner
    while (true) {
      try {
        const { balance: balanceAfter } = await readonly2.query.bank.balance({
          address: accounts2[0].address,
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
  }, 30_000 /* 30 seconds */);

  test("contracts sanity", async () => {
    const command =
      "docker exec ibc-relayer-1 hermes " +
      "--config /home/hermes-user/.hermes/alternative-config.toml " +
      "create channel " +
      "--a-chain secretdev-1 " +
      `--a-port ${contracts["secretdev-1"].v1.ibcPortId} ` +
      `--b-port ${contracts["secretdev-2"].v1.ibcPortId} ` +
      "--a-connection connection-0";

    console.log("calling relayer with command:", command);
    const result = execSync(command);

    const trimmedResult = result.toString().replace(/\s/g, "");

    const myRegexp = /ChannelId\("(channel-\d+)"/g;
    const channelId = myRegexp.exec(trimmedResult)[1];

    await waitForIBCChannel("secretdev-1", "http://localhost:1317", channelId);

    await waitForIBCChannel("secretdev-2", "http://localhost:2317", channelId);

    const res: any = await readonly.query.compute.queryContract({
      contract_address: contracts["secretdev-1"].v1.address,
      code_hash: contracts["secretdev-1"].v1.codeHash,
      query: {
        ibc_list_channels: {
          port_id: "wasm." + contracts["secretdev-1"].v1.address,
        },
      },
    });
    expect(res?.channels?.length).toBe(1);
    expect(res?.channels[0]?.endpoint?.port_id).toBe(
      "wasm." + contracts["secretdev-1"].v1.address
    );
    expect(res?.channels[0]?.endpoint?.channel_id).toBe(channelId);

    const res2: any = await readonly.query.compute.queryContract({
      contract_address: contracts["secretdev-1"].v1.address,
      code_hash: contracts["secretdev-1"].v1.codeHash,
      query: {
        ibc_channel: {
          port_id: "wasm." + contracts["secretdev-1"].v1.address,
          channel_id: channelId,
        },
      },
    });
    expect(res2?.channel?.endpoint?.port_id).toBe(
      "wasm." + contracts["secretdev-1"].v1.address
    );
    expect(res2?.channel?.endpoint?.channel_id).toBe(channelId);

    const tx = await accounts[0].secretjs.tx.compute.executeContract(
      {
        sender: accounts[0].address,
        contract_address: contracts["secretdev-1"].v1.address,
        code_hash: contracts["secretdev-1"].v1.codeHash,
        msg: {
          send_ibc_packet: {
            message: "hello from test",
          },
        },
      },
      { gasLimit: 250_000 }
    );
    console.log("tx", tx);
    if (tx.code !== TxResultCode.Success) {
      console.error(tx.rawLog);
    }
    expect(tx.code).toBe(TxResultCode.Success);
    console.log(
      "tx after triggering ibc send endpoint",
      JSON.stringify(cleanBytes(tx), null, 2)
    );

    expect(tx.arrayLog.find((x) => x.key === "packet_data").value).toBe(
      `{"message":{"value":"${channelId}hello from test"}}`
    );

    const packetSendCommand =
      "docker exec ibc-relayer-1 hermes " +
      "--config /home/hermes-user/.hermes/alternative-config.toml " +
      "tx packet-recv --dst-chain secretdev-2 --src-chain secretdev-1 " +
      `--src-port ${contracts["secretdev-1"].v1.ibcPortId} ` +
      `--src-channel ${channelId}`;

    console.log(
      "calling docker exec on relayer with command",
      packetSendCommand
    );
    let packetSendResult = execSync(packetSendCommand);
    console.log(
      "finished executing command, result:",
      packetSendResult.toString()
    );

    const packetAckCommand =
      "docker exec ibc-relayer-1 hermes " +
      "--config /home/hermes-user/.hermes/alternative-config.toml " +
      "tx packet-ack --dst-chain secretdev-1 --src-chain secretdev-2 " +
      `--src-port ${contracts["secretdev-1"].v1.ibcPortId} ` +
      `--src-channel ${channelId}`;

    console.log(
      "calling docker exec on relayer with command",
      packetAckCommand
    );
    const packetAckResult = execSync(packetAckCommand);
    console.log(
      "finished executing command, result:",
      packetAckResult.toString()
    );

    let queryResult: any =
      await accounts[0].secretjs.query.compute.queryContract({
        contract_address: contracts["secretdev-1"].v1.address,
        code_hash: contracts["secretdev-1"].v1.codeHash,
        query: {
          last_ibc_ack: {},
        },
      });

    const ack = fromUtf8(fromBase64(queryResult));

    expect(ack).toBe(`recv${channelId}hello from test`);

    queryResult = await accounts2[0].secretjs.query.compute.queryContract({
      contract_address: contracts["secretdev-2"].v1.address,
      code_hash: contracts["secretdev-2"].v1.codeHash,
      query: {
        last_ibc_ack: {},
      },
    });

    expect(queryResult).toBe(`no ack yet`);

    queryResult = await accounts[0].secretjs.query.compute.queryContract({
      contract_address: contracts["secretdev-1"].v1.address,
      code_hash: contracts["secretdev-1"].v1.codeHash,
      query: {
        last_ibc_receive: {},
      },
    });

    expect(queryResult).toBe(`no receive yet`);

    queryResult = await accounts2[0].secretjs.query.compute.queryContract({
      contract_address: contracts["secretdev-2"].v1.address,
      code_hash: contracts["secretdev-2"].v1.codeHash,
      query: {
        last_ibc_receive: {},
      },
    });

    expect(queryResult).toBe(`${channelId}hello from test`);
  }, 80_000 /* 80 seconds */);
});
