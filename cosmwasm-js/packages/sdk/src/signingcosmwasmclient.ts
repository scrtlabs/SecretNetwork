import { Sha256 } from "@iov/crypto";
import { Encoding } from "@iov/encoding";
import pako from "pako";

import { isValidBuilder } from "./builder";
import { Account, CosmWasmClient, GetNonceResult, PostTxResult } from "./cosmwasmclient";
import { makeSignBytes } from "./encoding";
import { SecretUtils } from "./enigmautils";
import { Attribute, findAttribute, Log } from "./logs";
import { BroadcastMode } from "./restclient";
import {
  Coin,
  Msg,
  MsgExecuteContract,
  MsgInstantiateContract,
  MsgSend,
  MsgStoreCode,
  StdFee,
  StdSignature,
  StdTx,
} from "./types";
import { OfflineSigner } from "./wallet";
import { decodeTxData, MsgData } from "./ProtoEncoding";

export interface SigningCallback {
  (signBytes: Uint8Array): Promise<StdSignature>;
}

export interface FeeTable {
  readonly upload: StdFee;
  readonly init: StdFee;
  readonly exec: StdFee;
  readonly send: StdFee;
}

function singleAmount(amount: number, denom: string): readonly Coin[] {
  return [{ amount: amount.toString(), denom: denom }];
}

function prepareBuilder(buider: string | undefined): string | undefined {
  if (buider === undefined) {
    return undefined; // normalization needed by backend
  } else {
    if (!isValidBuilder(buider)) throw new Error("The builder (Docker Hub image with tag) is not valid");
    return buider;
  }
}

const defaultFees: FeeTable = {
  upload: {
    amount: singleAmount(250_000, "uscrt"),
    gas: String(1_000_000),
  },
  init: {
    amount: singleAmount(125_000, "uscrt"),
    gas: String(500_000),
  },
  exec: {
    amount: singleAmount(50_000, "uscrt"),
    gas: String(200_000),
  },
  send: {
    amount: singleAmount(20_000, "uscrt"),
    gas: String(80_000),
  },
};

export interface UploadMeta {
  /** The source URL */
  readonly source?: string;
  /** The builder tag */
  readonly builder?: string;
}

export interface UploadResult {
  /** Size of the original wasm code in bytes */
  readonly originalSize: number;
  /** A hex encoded sha256 checksum of the original wasm code (that is stored on chain) */
  readonly originalChecksum: string;
  /** Size of the compressed wasm code in bytes */
  readonly compressedSize: number;
  /** A hex encoded sha256 checksum of the compressed wasm code (that stored in the transaction) */
  readonly compressedChecksum: string;
  /** The ID of the code asigned by the chain */
  readonly codeId: number;
  readonly logs: readonly Log[];
  /** Transaction hash (might be used as transaction ID). Guaranteed to be non-empty upper-case hex */
  readonly transactionHash: string;
}

export interface InstantiateResult {
  /** The address of the newly instantiated contract */
  readonly contractAddress: string;
  readonly logs: readonly Log[];
  /** Transaction hash (might be used as transaction ID). Guaranteed to be non-empty upper-case hex */
  readonly transactionHash: string;
  readonly data: any;
}

export interface ExecuteResult {
  readonly logs: readonly Log[];
  /** Transaction hash (might be used as transaction ID). Guaranteed to be non-empty upper-case hex */
  readonly transactionHash: string;
  readonly data: any;
}

export class SigningCosmWasmClient extends CosmWasmClient {
  public readonly senderAddress: string;
  private readonly signer: OfflineSigner | SigningCallback;
  private readonly fees: FeeTable;

  /**
   * Creates a new client with signing capability to interact with a CosmWasm blockchain. This is the bigger brother of CosmWasmClient.
   *
   * This instance does a lot of caching. In order to benefit from that you should try to use one instance
   * for the lifetime of your application. When switching backends, a new instance must be created.
   *
   * @param apiUrl The URL of a Cosmos SDK light client daemon API (sometimes called REST server or REST API)
   * @param senderAddress The address that will sign and send transactions using this instance
   * @param signer An asynchronous callback to create a signature for a given transaction. This can be implemented using secure key stores that require user interaction. Or a newer OfflineSigner type that handles that stuff
   * @param seedOrEnigmaUtils
   * @param customFees The fees that are paid for transactions
   * @param broadcastMode Defines at which point of the transaction processing the postTx method (i.e. transaction broadcasting) returns
   */
  public constructor(
    apiUrl: string,
    senderAddress: string,
    signer: SigningCallback | OfflineSigner,
    seedOrEnigmaUtils?: Uint8Array | SecretUtils,
    customFees?: Partial<FeeTable>,
    broadcastMode = BroadcastMode.Block,
  ) {
    if (seedOrEnigmaUtils instanceof Uint8Array) {
      super(apiUrl, seedOrEnigmaUtils, broadcastMode);
    } else {
      super(apiUrl, undefined, broadcastMode);
    }

    this.anyValidAddress = senderAddress;
    this.senderAddress = senderAddress;
    //this.signCallback = signCallback ? signCallback : undefined;
    this.signer = signer;
    if (seedOrEnigmaUtils && !(seedOrEnigmaUtils instanceof Uint8Array)) {
      this.restClient.enigmautils = seedOrEnigmaUtils;
    }
    this.fees = { ...defaultFees, ...(customFees || {}) };

    // // Setup contract->hash cache
    // // This is only needed here and not in CosmWasmClient because we
    // // need code hashes before sending txs
    // this.restClient.listCodeInfo().then(async (codes) => {
    //   for (const code of codes) {
    //     this.restClient.codeHashCache.set(code.id, code.data_hash);
    //     const contracts = await this.restClient.listContractsByCodeId(code.id);
    //     for (const contract of contracts) {
    //       this.restClient.codeHashCache.set(contract.address, code.data_hash);
    //     }
    //   }
    // });
  }

  public async getNonce(address?: string): Promise<GetNonceResult> {
    return super.getNonce(address || this.senderAddress);
  }

  public async getAccount(address?: string): Promise<Account | undefined> {
    return super.getAccount(address || this.senderAddress);
  }

  async signAdapter(
    msgs: Msg[],
    fee: StdFee,
    chainId: string,
    memo: string,
    accountNumber: number,
    sequence: number,
  ): Promise<StdTx> {
    // offline signer interface
    if ("sign" in this.signer) {
      const signResponse = await this.signer.sign(this.senderAddress, {
        chain_id: chainId,
        account_number: String(accountNumber),
        sequence: String(sequence),
        fee: fee,
        msgs: msgs,
        memo: memo,
      });

      return {
        msg: msgs,
        fee: signResponse.signed.fee,
        memo: signResponse.signed.memo,
        signatures: [signResponse.signature],
      };
    } else {
      // legacy interface
      const signBytes = makeSignBytes(msgs, fee, chainId, memo, accountNumber, sequence);
      const signature = await this.signer(signBytes);
      return {
        msg: msgs,
        fee: fee,
        memo: memo,
        signatures: [signature],
      };
    }
  }

  /** Uploads code and returns a receipt, including the code ID */
  public async upload(
    wasmCode: Uint8Array,
    meta: UploadMeta = {},
    memo = "",
    fee: StdFee = this.fees.upload,
  ): Promise<UploadResult> {
    if (!memo) {
      memo = "";
    }
    if (!meta) {
      meta = {};
    }

    const source = meta.source || undefined;
    const builder = prepareBuilder(meta.builder);

    const compressed = pako.gzip(wasmCode, { level: 9 });
    const storeCodeMsg: MsgStoreCode = {
      type: "wasm/MsgStoreCode",
      value: {
        sender: this.senderAddress,
        // eslint-disable-next-line @typescript-eslint/camelcase
        wasm_byte_code: Encoding.toBase64(compressed),
      },
    };

    if (source && source.length > 0) {
      storeCodeMsg.value.source = source;
    }
    if (builder && builder.length > 0) {
      storeCodeMsg.value.builder = builder;
    }

    const { accountNumber, sequence } = await this.getNonce();
    const chainId = await this.getChainId();
    const signedTx = await this.signAdapter([storeCodeMsg], fee, chainId, memo, accountNumber, sequence);

    const result = await this.postTx(signedTx);
    let codeIdAttr;
    if (this.restClient.broadcastMode == BroadcastMode.Block) {
      codeIdAttr = findAttribute(result.logs, "message", "code_id");
    }

    return {
      originalSize: wasmCode.length,
      originalChecksum: Encoding.toHex(new Sha256(wasmCode).digest()),
      compressedSize: compressed.length,
      compressedChecksum: Encoding.toHex(new Sha256(compressed).digest()),
      codeId:
        this.restClient.broadcastMode == BroadcastMode.Block
          ? Number.parseInt((codeIdAttr as Attribute).value, 10)
          : -1,
      logs: result.logs,
      transactionHash: result.transactionHash,
    };
  }

  public async instantiate(
    codeId: number,
    initMsg: object,
    label: string,
    memo = "",
    transferAmount?: readonly Coin[],
    fee: StdFee = this.fees.init,
    contractCodeHash?: string,
  ): Promise<InstantiateResult> {
    if (!contractCodeHash) {
      contractCodeHash = await this.restClient.getCodeHashByCodeId(codeId);
    } else {
      this.restClient.codeHashCache.set(codeId, contractCodeHash);
    }

    if (!memo) {
      memo = "";
    }

    const instantiateMsg: MsgInstantiateContract = {
      type: "wasm/MsgInstantiateContract",
      value: {
        sender: this.senderAddress,
        code_id: String(codeId),
        label: label,
        init_msg: Encoding.toBase64(await this.restClient.enigmautils.encrypt(contractCodeHash, initMsg)),
        init_funds: transferAmount ?? [],
      },
    };
    const { accountNumber, sequence } = await this.getNonce();
    const chainId = await this.getChainId();
    const signedTx = await this.signAdapter([instantiateMsg], fee, chainId, memo, accountNumber, sequence);

    const nonce = Encoding.fromBase64(instantiateMsg.value.init_msg).slice(0, 32);
    let result;
    try {
      result = await this.postTx(signedTx);
    } catch (err) {
      try {
        const errorMessageRgx = /failed to execute message; message index: 0: encrypted: (.+?): (?:instantiate|execute|query) contract failed/g;

        const rgxMatches = errorMessageRgx.exec(err.message);
        if (rgxMatches == null || rgxMatches.length != 2) {
          throw err;
        }

        const errorCipherB64 = rgxMatches[1];
        const errorCipherBz = Encoding.fromBase64(errorCipherB64);

        const errorPlainBz = await this.restClient.enigmautils.decrypt(errorCipherBz, nonce);

        err.message = err.message.replace(errorCipherB64, Encoding.fromUtf8(errorPlainBz));
      } catch (decryptionError) {
        throw new Error(
          `Failed to decrypt the following error message: ${err.message}. Decryption error of the error message: ${decryptionError.message}`,
        );
      }

      throw err;
    }

    let contractAddress = "";
    if (this.restClient.broadcastMode == BroadcastMode.Block) {
      contractAddress = findAttribute(result.logs, "message", "contract_address")?.value;
    }

    const logs =
      this.restClient.broadcastMode == BroadcastMode.Block
        ? await this.restClient.decryptLogs(result.logs, [nonce])
        : [];

    return {
      contractAddress,
      logs: logs,
      transactionHash: result.transactionHash,
      data: result.data, // data is the address of the new contract, so nothing to decrypt
    };
  }

  public async multiExecute(
    inputMsgs: Array<{
      contractAddress: string;
      contractCodeHash?: string;
      handleMsg: object;
      transferAmount?: readonly Coin[];
    }>,
    memo: string = "",
    totalFee?: StdFee,
  ): Promise<ExecuteResult> {
    if (!memo) {
      memo = "";
    }

    const msgs: Array<MsgExecuteContract> = [];
    for (const inputMsg of inputMsgs) {
      let { contractCodeHash } = inputMsg;
      if (!contractCodeHash) {
        contractCodeHash = await this.restClient.getCodeHashByContractAddr(inputMsg.contractAddress);
      } else {
        this.restClient.codeHashCache.set(inputMsg.contractAddress, contractCodeHash);
      }

      const msg: MsgExecuteContract = {
        type: "wasm/MsgExecuteContract",
        value: {
          sender: this.senderAddress,
          contract: inputMsg.contractAddress,
          //callback_code_hash: "",
          msg: Encoding.toBase64(
            await this.restClient.enigmautils.encrypt(contractCodeHash, inputMsg.handleMsg),
          ),
          sent_funds: inputMsg.transferAmount ?? [],
          //callback_sig: null,
        },
      };

      msgs.push(msg);
    }

    const { accountNumber, sequence } = await this.getNonce();
    const fee = totalFee ?? {
      gas: String(Number(this.fees.exec.gas) * inputMsgs.length),
      amount: this.fees.exec.amount,
    };
    const chainId = await this.getChainId();
    const signedTx = await this.signAdapter(msgs, fee, chainId, memo, accountNumber, sequence);

    let result;
    try {
      result = await this.postTx(signedTx);
    } catch (err) {
      try {
        const errorMessageRgx = /failed to execute message; message index: (\d+): encrypted: (.+?): (?:instantiate|execute|query) contract failed/g;
        const rgxMatches = errorMessageRgx.exec(err.message);
        if (rgxMatches == null || rgxMatches.length != 3) {
          throw err;
        }

        const errorCipherB64 = rgxMatches[1];
        const errorCipherBz = Encoding.fromBase64(errorCipherB64);

        const msgIndex = Number(rgxMatches[2]);
        const nonce = Encoding.fromBase64(msgs[msgIndex].value.msg).slice(0, 32);

        const errorPlainBz = await this.restClient.enigmautils.decrypt(errorCipherBz, nonce);

        err.message = err.message.replace(errorCipherB64, Encoding.fromUtf8(errorPlainBz));
      } catch (decryptionError) {
        throw new Error(
          `Failed to decrypt the following error message: ${err.message}. Decryption error of the error message: ${decryptionError.message}`,
        );
      }

      throw err;
    }

    const nonces = msgs.map((msg) => Encoding.fromBase64(msg.value.msg).slice(0, 32));

    // //const data = await this.restClient.decryptDataField(result.data, nonces);
    // const dataFields: MsgData[] = decodeTxData(Encoding.fromHex(result.data));
    //
    // let data = Uint8Array.from([]);
    // if (dataFields[0].data) {
    //   // dataFields[0].data = JSON.parse(decryptedData.toString());
    //   // @ts-ignore
    //   data = await this.restClient.decryptDataField(Encoding.toHex(Encoding.fromBase64(dataFields[0].data)), nonces);
    // }
    //
    // const logs = await this.restClient.decryptLogs(result.logs, nonces);

    let data = Uint8Array.from([]);
    if (this.restClient.broadcastMode == BroadcastMode.Block) {
      const dataFields: MsgData[] = decodeTxData(Encoding.fromHex(result.data));

      if (dataFields[0].data) {
        // decryptedData =
        // dataFields[0].data = JSON.parse(decryptedData.toString());
        // @ts-ignore
        data = await this.restClient.decryptDataField(
          Encoding.toHex(Encoding.fromBase64(dataFields[0].data)),
          nonces,
        );
      }
    }

    const logs =
      this.restClient.broadcastMode == BroadcastMode.Block
        ? await this.restClient.decryptLogs(result.logs, nonces)
        : [];

    return {
      logs: logs,
      transactionHash: result.transactionHash,
      // @ts-ignore
      data: data,
    };
  }

  public async execute(
    contractAddress: string,
    handleMsg: object,
    memo = "",
    transferAmount?: readonly Coin[],
    fee: StdFee = this.fees.exec,
    contractCodeHash?: string,
  ): Promise<ExecuteResult> {
    if (!contractCodeHash) {
      contractCodeHash = await this.restClient.getCodeHashByContractAddr(contractAddress);
    } else {
      this.restClient.codeHashCache.set(contractAddress, contractCodeHash);
    }

    if (!memo) {
      memo = "";
    }

    const executeMsg: MsgExecuteContract = {
      type: "wasm/MsgExecuteContract",
      value: {
        sender: this.senderAddress,
        contract: contractAddress,
        msg: Encoding.toBase64(await this.restClient.enigmautils.encrypt(contractCodeHash, handleMsg)),
        sent_funds: transferAmount ?? [],
      },
    };
    const { accountNumber, sequence } = await this.getNonce();

    const chainId = await this.getChainId();
    const signedTx = await this.signAdapter([executeMsg], fee, chainId, memo, accountNumber, sequence);

    const encryptionNonce = Encoding.fromBase64(executeMsg.value.msg).slice(0, 32);

    let result;
    try {
      result = await this.postTx(signedTx);
    } catch (err) {
      try {
        const errorMessageRgx = /failed to execute message; message index: 0: encrypted: (.+?): (?:instantiate|execute|query) contract failed/g;
        // console.log(`Got error message: ${err.message}`);

        const rgxMatches = errorMessageRgx.exec(err.message);
        if (rgxMatches == null || rgxMatches.length != 2) {
          throw err;
        }

        const errorCipherB64 = rgxMatches[1];

        // console.log(`Got error message: ${errorCipherB64}`);

        const errorCipherBz = Encoding.fromBase64(errorCipherB64);

        const errorPlainBz = await this.restClient.enigmautils.decrypt(errorCipherBz, encryptionNonce);

        err.message = err.message.replace(errorCipherB64, Encoding.fromUtf8(errorPlainBz));
      } catch (decryptionError) {
        throw new Error(
          `Failed to decrypt the following error message: ${err.message}. Decryption error of the error message: ${decryptionError.message}`,
        );
      }

      throw err;
    }
    let data = Uint8Array.from([]);
    if (this.restClient.broadcastMode == BroadcastMode.Block) {
      const dataFields: MsgData[] = decodeTxData(Encoding.fromHex(result.data));

      if (dataFields[0].data) {
        // decryptedData =
        // dataFields[0].data = JSON.parse(decryptedData.toString());
        // @ts-ignore
        data = await this.restClient.decryptDataField(
          Encoding.toHex(Encoding.fromBase64(dataFields[0].data)),
          [encryptionNonce],
        );
      }
    }

    const logs =
      this.restClient.broadcastMode == BroadcastMode.Block
        ? await this.restClient.decryptLogs(result.logs, [encryptionNonce])
        : [];

    return {
      logs,
      transactionHash: result.transactionHash,
      // @ts-ignore
      data,
    };
  }

  public async sendTokens(
    recipientAddress: string,
    transferAmount: readonly Coin[],
    memo = "",
    fee: StdFee = this.fees.send,
  ): Promise<PostTxResult> {
    const sendMsg: MsgSend = {
      type: "cosmos-sdk/MsgSend",
      value: {
        // eslint-disable-next-line @typescript-eslint/camelcase
        from_address: this.senderAddress,
        // eslint-disable-next-line @typescript-eslint/camelcase
        to_address: recipientAddress,
        amount: transferAmount,
      },
    };

    if (!memo) {
      memo = "";
    }

    const { accountNumber, sequence } = await this.getNonce();
    const chainId = await this.getChainId();
    const signedTx = await this.signAdapter([sendMsg], fee, chainId, memo, accountNumber, sequence);

    return this.postTx(signedTx);
  }
}
