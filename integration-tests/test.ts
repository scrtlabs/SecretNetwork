import { sha256 } from "@noble/hashes/sha256";
import { execSync } from "child_process";
import * as fs from "fs";
import {
  fromBase64,
  MsgExecuteContract,
  ProposalType,
  SecretNetworkClient,
  toBase64,
  toHex,
  toUtf8,
  Tx,
  TxResultCode,
  Wallet
} from "secretjs";
import {
  QueryBalanceRequest,
  QueryBalanceResponse,
} from "secretjs//dist/protobuf_stuff/cosmos/bank/v1beta1/query";
import { MsgSend } from "secretjs/dist/protobuf_stuff/cosmos/bank/v1beta1/tx";
import {
  ibcDenom,
  sleep,
  storeContracts,
  waitForBlocks,
  waitForIBCChannel,
  waitForIBCConnection,
  Contract, instantiateContracts, cleanBytes,
} from "./utils";

const contracts = {
  "secretdev-1": {
    v1: new Contract("v1"),
    v010: new Contract("v010"),
  },
  "secretdev-2": {
    v1: new Contract("v1"),
    v010: new Contract("v010"),
  }
}

// let accounts;
// let accounts2;
// let readonly;

let v1Wasm: Uint8Array;
let v010Wasm: Uint8Array;

// @ts-ignore
const accounts: {
  a: SecretNetworkClient;
  b: SecretNetworkClient;
  c: SecretNetworkClient;
  // avoid using account d which is used by the ibc relayer
} = {};
let readonly: SecretNetworkClient;

// @ts-ignore
// accounts on secretdev-2
const accounts2: {
  a: SecretNetworkClient;
  b: SecretNetworkClient;
  c: SecretNetworkClient;
  // avoid using account d which is used by the ibc relayer
} = {};
let readonly2: SecretNetworkClient;

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

  readonly = await SecretNetworkClient.create({
    chainId: "secretdev-1",
    grpcWebUrl: "http://localhost:9091",
  });

  accounts2.a = await SecretNetworkClient.create({
    chainId: "secretdev-2",
    grpcWebUrl: "http://localhost:9391",
    wallet: walletA,
    walletAddress: walletA.address,
  });

  accounts2.b = await SecretNetworkClient.create({
    chainId: "secretdev-2",
    grpcWebUrl: "http://localhost:9391",
    wallet: walletB,
    walletAddress: walletB.address,
  });

  accounts2.c = await SecretNetworkClient.create({
    chainId: "secretdev-2",
    grpcWebUrl: "http://localhost:9391",
    wallet: walletC,
    walletAddress: walletC.address,
  });

  readonly2 = await SecretNetworkClient.create({
    chainId: "secretdev-2",
    grpcWebUrl: "http://localhost:9391",
  });
  await waitForBlocks("secretdev-1");

  v1Wasm = fs.readFileSync(
    `${__dirname}/contract-v1/contract.wasm`
  ) as Uint8Array;
  contracts["secretdev-1"].v1.codeHash = toHex(sha256(v1Wasm));
  contracts["secretdev-2"].v1.codeHash = contracts["secretdev-1"].v1.codeHash;

  v010Wasm = fs.readFileSync(
    `${__dirname}/contract-v0.10/contract.wasm`
  ) as Uint8Array;
  contracts["secretdev-1"].v010.codeHash = toHex(sha256(v010Wasm));
  contracts["secretdev-2"].v010.codeHash = contracts["secretdev-1"].v010.codeHash;
  console.log("v1codehash", contracts["secretdev-1"].v1.codeHash);
  console.log("v010codeHash", contracts["secretdev-1"].v010.codeHash);

  console.log("Uploading contracts on chain 1...");
  let tx: Tx = await storeContracts(accounts.a, [v1Wasm, v010Wasm]);
  contracts["secretdev-1"].v1.codeId = Number(tx.arrayLog.find(x => x.key === "code_id").value);
  contracts["secretdev-1"].v010.codeId = Number(tx.arrayLog.reverse().find(x => x.key === "code_id").value);
  console.log("v1codeid", contracts["secretdev-1"].v1.codeId);
  console.log("v010codeid", contracts["secretdev-1"].v010.codeId);

  console.log("Instantiating contracts on chain 1...");
  tx = await instantiateContracts(accounts.a, [contracts["secretdev-1"].v1, contracts["secretdev-1"].v010]);
  contracts["secretdev-1"].v1.address = tx.arrayLog.find(x => x.key === "contract_address").value;
  contracts["secretdev-1"].v010.address = tx.arrayLog.reverse().find(x => x.key === "contract_address").value;

  // get contract's ibc_port_id
  // const info = (await readonly.query.compute.contractInfo(contracts["secretdev-1"].v1.address)).ContractInfo;
  // console.log("info", info);
  // console.log("info.ibcPortId", info.ibcPortId);
  // temp guessing:
  contracts["secretdev-1"].v1.ibcPortId = "wasm." + contracts["secretdev-1"].v1.address;

  // create a second validator for MsgRedelegate tests
  const { validators } = await readonly.query.staking.validators({});
  if (validators.length === 1) {
    tx = await accounts.b.tx.staking.createValidator(
      {
        selfDelegatorAddress: accounts.b.address,
        commission: {
          maxChangeRate: 0.01,
          maxRate: 0.1,
          rate: 0.05,
        },
        description: {
          moniker: "banana",
          identity: "papaya",
          website: "watermelon.com",
          securityContact: "info@watermelon.com",
          details: "We are the banana papaya validator",
        },
        pubkey: toBase64(new Uint8Array(32).fill(1)),
        minSelfDelegation: "1",
        initialDelegation: { amount: "1", denom: "uscrt" },
      },
      { gasLimit: 100_000 }
    );
    expect(tx.code).toBe(TxResultCode.Success);
  }
});

describe("BankMsg", () => {
  describe("Send", () => {
    test("v1", async () => {
      const tx = await accounts.a.tx.compute.executeContract(
        {
          sender: accounts.a.address,
          contractAddress: contracts["secretdev-1"].v1.address,
          codeHash: contracts["secretdev-1"].v1.codeHash,
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
      if (tx.code !== TxResultCode.Success) {
        console.error(tx.rawLog);
      }
      expect(tx.code).toBe(TxResultCode.Success);
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
          value: accounts.b.address,
        },
        { key: "amount", msg: 0, type: "coin_received", value: "1uscrt" },
      ]);
    });

    describe("v0.10", () => {
      test("success", async () => {
        const tx = await accounts.a.tx.compute.executeContract(
          {
            sender: accounts.a.address,
            contractAddress: contracts["secretdev-1"].v010.address,
            codeHash: contracts["secretdev-1"].v010.codeHash,
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
            value: accounts.a.address,
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
            value: accounts.b.address,
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

        const tx = await accounts.a.tx.compute.executeContract(
          {
            sender: accounts.a.address,
            contractAddress: contracts["secretdev-1"].v010.address,
            codeHash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              bank_msg_send: {
                to_address: accounts.b.address,
                amount: [
                  { amount: String(contractBalance + 1), denom: "uscrt" },
                ],
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
});

describe("CustomMsg", () => {
  test.skip("v1", async () => {
    // TODO
  });

  test("v0.10", async () => {
    const tx = await accounts.a.tx.compute.executeContract(
      {
        sender: accounts.a.address,
        contractAddress: contracts["secretdev-1"].v010.address,
        codeHash: contracts["secretdev-1"].v010.codeHash,
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

describe("GovMsgVote", () => {
  let proposalId: number;

  beforeAll(async () => {
    let tx = await accounts.a.tx.gov.submitProposal(
      {
        type: ProposalType.TextProposal,
        proposer: accounts.a.address,
        // on localsecret min deposit is 10 SCRT
        initialDeposit: [{ amount: String(10_000_000), denom: "uscrt" }],
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

  describe("v1", () => {
    test.skip("success", async () => {
      // TODO
    });
    test.skip("error", async () => {
      // TODO
    });
  });

  describe("v0.10", () => {
    test("success", async () => {
      const tx = await accounts.a.tx.compute.executeContract(
        {
          sender: accounts.a.address,
          contractAddress: contracts["secretdev-1"].v010.address,
          codeHash: contracts["secretdev-1"].v010.codeHash,
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
      const tx = await accounts.a.tx.compute.executeContract(
        {
          sender: accounts.a.address,
          contractAddress: contracts["secretdev-1"].v010.address,
          codeHash: contracts["secretdev-1"].v010.codeHash,
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
});

describe("Wasm", () => {
  describe("MsgInstantiateContract", () => {
    describe("v1", () => {
      test.skip("success", async () => {
        // TODO
      });
      test.skip("error", async () => {
        // TODO
      });
    });

    describe("v0.10", () => {
      test("success", async () => {
        const tx = await accounts.a.tx.compute.executeContract(
          {
            sender: accounts.a.address,
            contractAddress: contracts["secretdev-1"].v010.address,
            codeHash: contracts["secretdev-1"].v010.codeHash,
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
        expect(attributes[1].value).not.toBe(contracts["secretdev-1"].v010.address);
      });

      test("error", async () => {
        const tx = await accounts.a.tx.compute.executeContract(
          {
            sender: accounts.a.address,
            contractAddress: contracts["secretdev-1"].v010.address,
            codeHash: contracts["secretdev-1"].v010.codeHash,
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
      test.skip("success", async () => {
        // TODO
      });
      test.skip("error", async () => {
        // TODO
      });
    });

    describe("v0.10", () => {
      test("success", async () => {
        const tx = await accounts.a.tx.compute.executeContract(
          {
            sender: accounts.a.address,
            contractAddress: contracts["secretdev-1"].v010.address,
            codeHash: contracts["secretdev-1"].v010.codeHash,
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
        const tx = await accounts.a.tx.compute.executeContract(
          {
            sender: accounts.a.address,
            contractAddress: contracts["secretdev-1"].v010.address,
            codeHash: contracts["secretdev-1"].v010.codeHash,
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
      test.skip("success", async () => {
        // TODO
      });
      test.skip("error", async () => {
        // TODO
      });
    });

    describe("v0.10", () => {
      test("success", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validator = validators[0].operatorAddress;

        const tx = await accounts.a.tx.compute.executeContract(
          {
            sender: accounts.a.address,
            contractAddress: contracts["secretdev-1"].v010.address,
            codeHash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              staking_msg_delegate: {
                validator,
                amount: { amount: "1", denom: "uscrt" },
              },
            },
            sentFunds: [{ amount: "1", denom: "uscrt" }],
          },
          { gasLimit: 250_000 }
        );
        if (tx.code !== TxResultCode.Success) {
          console.error(tx.rawLog);
        }
        expect(tx.code).toBe(TxResultCode.Success);

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
        const validator = validators[0].operatorAddress;

        const tx = await accounts.a.tx.compute.executeContract(
          {
            sender: accounts.a.address,
            contractAddress: contracts["secretdev-1"].v010.address,
            codeHash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              staking_msg_delegate: {
                validator: validator + "garbage",
                amount: { amount: "1", denom: "uscrt" },
              },
            },
            sentFunds: [{ amount: "1", denom: "uscrt" }],
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
    describe("v1", () => {
      test.skip("success", async () => {
        // TODO
      });
      test.skip("error", async () => {
        // TODO
      });
    });

    describe("v0.10", () => {
      test("success", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validator = validators[0].operatorAddress;

        const tx = await accounts.a.tx.broadcast(
          [
            new MsgExecuteContract({
              sender: accounts.a.address,
              contractAddress: contracts["secretdev-1"].v010.address,
              codeHash: contracts["secretdev-1"].v010.codeHash,
              msg: {
                staking_msg_delegate: {
                  validator,
                  amount: { amount: "1", denom: "uscrt" },
                },
              },
              sentFunds: [{ amount: "1", denom: "uscrt" }],
            }),
            new MsgExecuteContract({
              sender: accounts.a.address,
              contractAddress: contracts["secretdev-1"].v010.address,
              codeHash: contracts["secretdev-1"].v010.codeHash,
              msg: {
                staking_msg_undelegate: {
                  validator,
                  amount: { amount: "1", denom: "uscrt" },
                },
              },
              sentFunds: [{ amount: "1", denom: "uscrt" }],
            }),
          ],
          { gasLimit: 250_000 }
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
        const validator = validators[0].operatorAddress;

        const tx = await accounts.a.tx.compute.executeContract(
          {
            sender: accounts.a.address,
            contractAddress: contracts["secretdev-1"].v010.address,
            codeHash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              staking_msg_undelegate: {
                validator: validator + "garbage",
                amount: { amount: "1", denom: "uscrt" },
              },
            },
            sentFunds: [{ amount: "1", denom: "uscrt" }],
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
      test.skip("success", async () => {
        // TODO
      });
      test.skip("error", async () => {
        // TODO
      });
    });

    describe("v0.10", () => {
      test("success", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validatorA = validators[0].operatorAddress;
        const validatorB = validators[1].operatorAddress;

        const tx = await accounts.a.tx.broadcast(
          [
            new MsgExecuteContract({
              sender: accounts.a.address,
              contractAddress: contracts["secretdev-1"].v010.address,
              codeHash: contracts["secretdev-1"].v010.codeHash,
              msg: {
                staking_msg_delegate: {
                  validator: validatorA,
                  amount: { amount: "1", denom: "uscrt" },
                },
              },
              sentFunds: [{ amount: "1", denom: "uscrt" }],
            }),
            new MsgExecuteContract({
              sender: accounts.a.address,
              contractAddress: contracts["secretdev-1"].v010.address,
              codeHash: contracts["secretdev-1"].v010.codeHash,
              msg: {
                staking_msg_redelegate: {
                  src_validator: validatorA,
                  dst_validator: validatorB,
                  amount: { amount: "1", denom: "uscrt" },
                },
              },
              sentFunds: [{ amount: "1", denom: "uscrt" }],
            }),
          ],
          { gasLimit: 250_000 }
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
        const validator = validators[0].operatorAddress;

        const tx = await accounts.a.tx.compute.executeContract(
          {
            sender: accounts.a.address,
            contractAddress: contracts["secretdev-1"].v010.address,
            codeHash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              staking_msg_redelegate: {
                src_validator: validator,
                dst_validator: validator + "garbage",
                amount: { amount: "1", denom: "uscrt" },
              },
            },
            sentFunds: [{ amount: "1", denom: "uscrt" }],
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
      test.skip("success", async () => {
        // TODO
      });
      test.skip("error", async () => {
        // TODO
      });
    });

    describe("v0.10", () => {
      test("success", async () => {
        const { validators } = await readonly.query.staking.validators({});
        const validator = validators[0].operatorAddress;

        const tx = await accounts.a.tx.broadcast(
          [
            new MsgExecuteContract({
              sender: accounts.a.address,
              contractAddress: contracts["secretdev-1"].v010.address,
              codeHash: contracts["secretdev-1"].v010.codeHash,
              msg: {
                staking_msg_delegate: {
                  validator: validator,
                  amount: { amount: "1", denom: "uscrt" },
                },
              },
              sentFunds: [{ amount: "1", denom: "uscrt" }],
            }),
            new MsgExecuteContract({
              sender: accounts.a.address,
              contractAddress: contracts["secretdev-1"].v010.address,
              codeHash: contracts["secretdev-1"].v010.codeHash,
              msg: {
                staking_msg_withdraw: {
                  validator: validator,
                  recipient: accounts.a.address,
                },
              },
              sentFunds: [{ amount: "1", denom: "uscrt" }],
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
        const validator = validators[0].operatorAddress;

        const tx = await accounts.a.tx.compute.executeContract(
          {
            sender: accounts.a.address,
            contractAddress: contracts["secretdev-1"].v010.address,
            codeHash: contracts["secretdev-1"].v010.codeHash,
            msg: {
              staking_msg_redelegate: {
                src_validator: validator,
                dst_validator: validator + "garbage",
                amount: { amount: "1", denom: "uscrt" },
              },
            },
            sentFunds: [{ amount: "1", denom: "uscrt" }],
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
    const tx = await accounts.a.tx.compute.executeContract(
      {
        sender: accounts.a.address,
        contractAddress: contracts["secretdev-1"].v1.address,
        codeHash: contracts["secretdev-1"].v1.codeHash,
        msg: {
          stargate_msg: {
            type_url: "/cosmos.bank.v1beta1.MsgSend",
            value: toBase64(
              MsgSend.encode({
                fromAddress: contracts["secretdev-1"].v1.address,
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
    if (tx.code !== TxResultCode.Success) {
      console.error(tx.rawLog);
    }
    expect(tx.code).toBe(TxResultCode.Success);
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
          value: accounts.b.address,
        },
        { key: "amount", msg: 0, type: "coin_received", value: "1uscrt" },
      ]
    );
  });
});

describe("StargateQuery", () => {
  test("v1", async () => {
    const result: any = await readonly.query.compute.queryContract({
      contractAddress: contracts["secretdev-1"].v1.address,
      codeHash: contracts["secretdev-1"].v1.codeHash,
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
      const result: any = await readonly.query.compute.queryContract({
        contractAddress: contracts["secretdev-1"].v1.address,
        codeHash: contracts["secretdev-1"].v1.codeHash,
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
      const result: any = await readonly.query.compute.queryContract({
        contractAddress: contracts["secretdev-1"].v010.address,
        codeHash: contracts["secretdev-1"].v010.codeHash,
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
    console.log("Uploading contracts on chain 2...");
    let tx: Tx = await storeContracts(accounts2.a, [v1Wasm, v010Wasm]);
    contracts["secretdev-2"].v1.codeId = Number(tx.arrayLog.find((x) => x.key === "code_id").value);
    contracts["secretdev-2"].v010.codeId = Number(tx.arrayLog.reverse().find((x) => x.key === "code_id").value);

    console.log("Instantiating contracts on chain 2...");
    tx = await instantiateContracts(accounts2.a, [contracts["secretdev-2"].v1, contracts["secretdev-2"].v010]);
    contracts["secretdev-2"].v1.address = tx.arrayLog.find((x) => x.key === "contract_address").value;
    contracts["secretdev-2"].v010.address = tx.arrayLog.reverse().find((x) => x.key === "contract_address").value;
    contracts["secretdev-2"].v1.ibcPortId = "wasm." + contracts["secretdev-2"].v1.address;

    console.log("Waiting for IBC to set up...");
    await waitForIBCConnection("secretdev-1", "http://localhost:9091");
    await waitForIBCConnection("secretdev-2", "http://localhost:9391");

    await waitForIBCChannel(
      "secretdev-1",
      "http://localhost:9091",
      "channel-0"
    );
    await waitForIBCChannel(
      "secretdev-2",
      "http://localhost:9391",
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

    expect(result.code).toBe(TxResultCode.Success);

    // checking ack/timeout on secretdev-1 might be cleaner
    while (true) {
      try {
        const { balance: balanceAfter } = await readonly2.query.bank.balance({
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
  }, 30_000 /* 30 seconds */);

  test.only("contracts sanity", async () => {
    const command = "docker exec ibc-relayer-1 hermes " +
      "--config /home/hermes-user/.hermes/alternative-config.toml " +
      "create channel " +
      "--a-chain secretdev-1 " +
      `--a-port ${contracts["secretdev-1"].v1.ibcPortId} ` +
      `--b-port ${contracts["secretdev-2"].v1.ibcPortId} ` +
      "--a-connection connection-0";

    console.log("calling docker exec on relayer with command", command);
    let result = execSync(command);
    console.log("finished executing command, result:", result.toString());

    let trimmedResult = result.toString().replace(/\s/g, "");

    let myRegexp = /ChannelId\("(channel-\d+)"/g;
    let channelId = myRegexp.exec(trimmedResult)[1];
    console.log("channelId", channelId);

    await waitForIBCChannel(
      "secretdev-1",
      "http://localhost:9091",
      channelId ,
    );

    await waitForIBCChannel(
      "secretdev-2",
      "http://localhost:9391",
      channelId ,
    );

    const tx = await accounts.a.tx.compute.executeContract(
      {
        sender: accounts.a.address,
        contractAddress: contracts["secretdev-1"].v1.address,
        codeHash: contracts["secretdev-1"].v1.codeHash,
        msg: {
          send_ibc_packet: {
            message: "hello from test"
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
    console.log("tx after triggering ibc send endpoint", JSON.stringify(cleanBytes(tx), null, 2));

    expect(
      tx.arrayLog.find(x => x.key === "packet_data").value
    ).toBe(
      `{"message":{"value":"${channelId}hello from test"}}`
    );

    // console.log("sleeping");
    // await sleep(10 * 1000);
    // console.log("sleeping end");

    while(true) {
      let queryResult: any = await accounts.a.query.compute.queryContract({
        contractAddress: contracts["secretdev-1"].v1.address,
        codeHash: contracts["secretdev-1"].v1.codeHash,
        query: {
          last_ibc_ack: {}
        },
      });

      console.log("queryResult1", queryResult); // should be ack

      queryResult = await accounts2.a.query.compute.queryContract({
        contractAddress: contracts["secretdev-2"].v1.address,
        codeHash: contracts["secretdev-2"].v1.codeHash,
        query: {
          last_ibc_ack: {}
        },
      });

      console.log("queryResult2", queryResult); // should be none

      queryResult = await accounts.a.query.compute.queryContract({
        contractAddress: contracts["secretdev-1"].v1.address,
        codeHash: contracts["secretdev-1"].v1.codeHash,
        query: {
          last_ibc_receive: {}
        },
      });

      console.log("queryResult3", queryResult); // should be none

      queryResult = await accounts2.a.query.compute.queryContract({
        contractAddress: contracts["secretdev-2"].v1.address,
        codeHash: contracts["secretdev-2"].v1.codeHash,
        query: {
          last_ibc_receive: {}
        },
      });

      console.log("queryResult4", queryResult); // should be receive

      queryResult = await accounts.a.query.compute.queryContract({
        contractAddress: contracts["secretdev-1"].v1.address,
        codeHash: contracts["secretdev-1"].v1.codeHash,
        query: {
          last_ibc_timeout: {}
        },
      });

      console.log("queryResult5", queryResult); // should be none

      queryResult = await accounts2.a.query.compute.queryContract({
        contractAddress: contracts["secretdev-2"].v1.address,
        codeHash: contracts["secretdev-2"].v1.codeHash,
        query: {
          last_ibc_timeout: {}
        },
      });

      console.log("queryResult6", queryResult); // should be receive

      await sleep(400);
    }
  }, 80_000 /* 80 seconds */);
});
