import { Bech32, Encoding, isNonNullObject } from "@iov/encoding";
import axios, { AxiosError, AxiosInstance } from "axios";

import EnigmaUtils, { SecretUtils } from "./enigmautils";
import { Attribute, Log } from "./logs";
import { decodeTxData, MsgData } from "./ProtoEncoding";
import {
  Coin,
  CosmosSdkTx,
  JsonObject,
  Model,
  Msg,
  MsgExecuteContract,
  MsgInstantiateContract,
  parseWasmData,
  StdTx,
  WasmData,
} from "./types";
import { sleep } from "@iov/utils";
import {decodeBech32Pubkey} from "./pubkey";

export interface CosmosSdkAccount {
  /** Bech32 account address */
  readonly address: string;
  readonly coins: ReadonlyArray<Coin>;
  /** Bech32 encoded pubkey */
  readonly public_key: string;
  readonly account_number: number;
  readonly sequence: number;
}

export interface NodeInfo {
  readonly protocol_version: {
    readonly p2p: string;
    readonly block: string;
    readonly app: string;
  };
  readonly id: string;
  readonly listen_addr: string;
  readonly network: string;
  readonly version: string;
  readonly channels: string;
  readonly moniker: string;
  readonly other: {
    readonly tx_index: string;
    readonly rpc_address: string;
  };
}

export interface ApplicationVersion {
  readonly name: string;
  readonly server_name: string;
  readonly client_name: string;
  readonly version: string;
  readonly commit: string;
  readonly build_tags: string;
  readonly go: string;
}

export interface NodeInfoResponse {
  readonly node_info: NodeInfo;
  readonly application_version: ApplicationVersion;
}

export interface BlockId {
  readonly hash: string;
  // TODO: here we also have this
  // parts: {
  //   total: '1',
  //   hash: '7AF200C78FBF9236944E1AB270F4045CD60972B7C265E3A9DA42973397572931'
  // }
}

export interface BlockHeader {
  readonly version: {
    readonly block: string;
    readonly app: string;
  };
  readonly height: string;
  readonly chain_id: string;
  /** An RFC 3339 time string like e.g. '2020-02-15T10:39:10.4696305Z' */
  readonly time: string;
  readonly last_commit_hash: string;
  readonly last_block_id: BlockId;
  /** Can be empty */
  readonly data_hash: string;
  readonly validators_hash: string;
  readonly next_validators_hash: string;
  readonly consensus_hash: string;
  readonly app_hash: string;
  /** Can be empty */
  readonly last_results_hash: string;
  /** Can be empty */
  readonly evidence_hash: string;
  readonly proposer_address: string;
}

export interface Block {
  readonly header: BlockHeader;
  readonly data: {
    /** Array of base64 encoded transactions */
    readonly txs: ReadonlyArray<string> | null;
  };
}

export interface BlockResponse {
  readonly block_id: BlockId;
  readonly block: Block;
}

interface AuthAccountsResponse {
  readonly height: string;
  readonly result: {
    readonly type: "cosmos-sdk/Account";
    readonly value: CosmosSdkAccount;
  };
}

interface ContractHashResponse {
  readonly height: string;
  readonly result: string;
}

// Currently all wasm query responses return json-encoded strings...
// later deprecate this and use the specific types for result
// (assuming it is inlined, no second parse needed)
type WasmResponse<T = string> = WasmSuccess<T> | WasmError;

interface WasmSuccess<T = string> {
  readonly height: string;
  readonly result: T;
}

interface WasmError {
  readonly error: string;
}

export interface TxsResponse {
  readonly height: string;
  readonly txhash: string;
  /** 🤷‍♂️ */
  readonly codespace?: string;
  /** Falsy when transaction execution succeeded. Contains error code on error. */
  readonly code?: number;
  raw_log: string;
  data: string;
  logs?: Log[];
  readonly tx: CosmosSdkTx;
  /** The gas limit as set by the user */
  readonly gas_wanted?: string;
  /** The gas used by the execution */
  readonly gas_used?: string;
  readonly timestamp: string;
}

interface SearchTxsResponse {
  readonly total_count: string;
  readonly count: string;
  readonly page_number: string;
  readonly page_total: string;
  readonly limit: string;
  readonly txs: TxsResponse[];
}

export interface PostTxsResponse {
  readonly height: string;
  readonly txhash: string;
  readonly code?: number;
  readonly raw_log?: string;
  data: any;
  /** The same as `raw_log` but deserialized? */
  readonly logs?: object;
  /** The gas limit as set by the user */
  readonly gas_wanted?: string;
  /** The gas used by the execution */
  readonly gas_used?: string;
}

interface EncodeTxResponse {
  // base64-encoded amino-binary encoded representation
  readonly tx: string;
}

export interface CodeInfo {
  readonly id: number;
  /** Bech32 account address */
  readonly creator: string;
  /** Hex-encoded sha256 hash of the code stored here */
  readonly data_hash: string;
  // TODO: these are not supported in current wasmd
  readonly source?: string;
  readonly builder?: string;
}

export interface CodeDetails extends CodeInfo {
  /** Base64 encoded raw wasm data */
  readonly data: any;
}

// This is list view, without contract info
export interface ContractInfo {
  readonly address: string;
  readonly code_id: number;
  /** Bech32 account address */
  readonly creator: string;
  readonly label: string;
}

export interface ContractDetails extends ContractInfo {
  /** Argument passed on initialization of the contract */
  readonly init_msg: object;
}

interface SmartQueryResponse {
  // base64 encoded response
  readonly data: string;
}

type RestClientResponse =
  | NodeInfoResponse
  | BlockResponse
  | AuthAccountsResponse
  | TxsResponse
  | SearchTxsResponse
  | PostTxsResponse
  | EncodeTxResponse
  | WasmResponse<string>
  | WasmResponse<CodeInfo[]>
  | WasmResponse<CodeDetails>
  | WasmResponse<ContractInfo[] | null>
  | WasmResponse<ContractDetails | null>
  | WasmResponse<ContractHashResponse | null>;

/** Unfortunately, Cosmos SDK encodes empty arrays as null */
type CosmosSdkArray<T> = ReadonlyArray<T> | null;

function normalizeArray<T>(backend: CosmosSdkArray<T>): ReadonlyArray<T> {
  return backend || [];
}

/**
 * The mode used to send transaction
 *
 * @see https://cosmos.network/rpc/#/Transactions/post_txs
 */
export enum BroadcastMode {
  /** Return after tx commit */
  Block = "block",
  /** Return afer CheckTx */
  Sync = "sync",
  /** Return right away */
  Async = "async",
}

function isWasmError<T>(resp: WasmResponse<T>): resp is WasmError {
  return (resp as WasmError).error !== undefined;
}

function unwrapWasmResponse<T>(response: WasmResponse<T>): T {
  if (isWasmError(response)) {
    throw new Error(response.error);
  }
  return response.result;
}

// We want to get message data from 500 errors
// https://stackoverflow.com/questions/56577124/how-to-handle-500-error-message-with-axios
// this should be chained to catch one error and throw a more informative one
function parseAxiosError(err: AxiosError): never {
  // use the error message sent from server, not default 500 msg
  if (err.response?.data) {
    let errorText: string;
    const data = err.response.data;
    // expect { error: string }, but otherwise dump
    if (data.error && typeof data.error === "string") {
      errorText = data.error;
    } else if (typeof data === "string") {
      errorText = data;
    } else {
      errorText = JSON.stringify(data);
    }
    throw new Error(`${errorText} (HTTP ${err.response.status})`);
  } else {
    throw err;
  }
}

export class RestClient {
  private readonly client: AxiosInstance;
  public readonly broadcastMode: BroadcastMode;
  public enigmautils: SecretUtils;

  public codeHashCache: Map<string | number, string>;

  /**
   * Creates a new client to interact with a Cosmos SDK light client daemon.
   * This class tries to be a direct mapping onto the API. Some basic decoding and normalizatin is done
   * but things like caching are done at a higher level.
   *
   * When building apps, you should not need to use this class directly. If you do, this indicates a missing feature
   * in higher level components. Feel free to raise an issue in this case.
   *
   * @param apiUrl The URL of a Cosmos SDK light client daemon API (sometimes called REST server or REST API)
   * @param broadcastMode Defines at which point of the transaction processing the postTx method (i.e. transaction broadcasting) returns
   * @param seed - The seed used to generate sender TX encryption key. If empty will generate random new one
   */
  public constructor(apiUrl: string, broadcastMode = BroadcastMode.Block, seed?: Uint8Array) {
    const headers = {
      post: { "Content-Type": "application/json" },
    };
    this.client = axios.create({
      baseURL: apiUrl,
      headers: headers,
    });
    this.broadcastMode = broadcastMode;
    this.enigmautils = new EnigmaUtils(apiUrl, seed);
    this.codeHashCache = new Map<any, string>();
  }

  public async get(path: string): Promise<RestClientResponse> {
    const { data } = await this.client.get(path).catch(parseAxiosError);
    if (data === null) {
      throw new Error("Received null response from server");
    }
    return data;
  }

  async get_raw(path: string): Promise<RestClientResponse> {
    const { data } = await this.client.get(path);
    if (data === null) {
      throw new Error("Received null response from server");
    }
    return data;
  }

  public async post(path: string, params: any): Promise<RestClientResponse> {
    if (!isNonNullObject(params)) throw new Error("Got unexpected type of params. Expected object.");
    const { data } = await this.client.post(path, params).catch(parseAxiosError);
    if (data === null) {
      throw new Error("Received null response from server");
    }
    return data;
  }

  // The /auth endpoints
  public async authAccounts(address: string): Promise<AuthAccountsResponse> {
    const [authResp, bankResp]: [
      {
        height: string;
        result: {
          type: string;
          value: {
            address: string;
            public_key: {
              type: string;
              value: string;
            };
            account_number: string;
            sequence: string;
          };
        };
      },
      { height: string; result: Coin[] },
    ] = (await Promise.all([
      this.get(`/auth/accounts/${address}`),
      this.get(`/bank/balances/${address}`),
    ])) as any;

    const result = {
      height: bankResp.height,
      result: {
        type: "cosmos-sdk/Account",
        value: {
          address: authResp.result.value.address,
          coins: bankResp.result,
          public_key: JSON.stringify(authResp.result.value.public_key),
          account_number: Number(authResp.result.value.account_number || 0),
          sequence: Number(authResp.result.value.sequence || 0),
        },
      },
    };

    return result as AuthAccountsResponse;
  }

  // The /blocks endpoints
  public async blocksLatest(): Promise<BlockResponse> {
    const responseData = await this.get("/blocks/latest");
    if (!(responseData as any).block) {
      throw new Error("Unexpected response data format");
    }
    return responseData as BlockResponse;
  }

  public async blocks(height: number): Promise<BlockResponse> {
    const responseData = await this.get(`/blocks/${height}`);
    if (!(responseData as any).block) {
      throw new Error("Unexpected response data format");
    }
    return responseData as BlockResponse;
  }

  // The /node_info endpoint
  public async nodeInfo(): Promise<NodeInfoResponse> {
    const responseData = await this.get("/node_info");
    if (!(responseData as any).node_info) {
      throw new Error("Unexpected response data format");
    }
    return responseData as NodeInfoResponse;
  }

  // The /txs endpoints
  public async txById(id: string, tryToDecrypt = true): Promise<TxsResponse> {
    const responseData = await this.get(`/txs/${id}`);
    if (!(responseData as any).tx) {
      throw new Error("Unexpected response data format");
    }

    if (tryToDecrypt) {
      return this.decryptTxsResponse(responseData as TxsResponse);
    } else {
      return responseData as TxsResponse;
    }
  }

  public async txsQuery(query: string, tryToDecrypt = true): Promise<SearchTxsResponse> {
    const responseData = await this.get(`/txs?${query}`);
    if (!(responseData as any).txs) {
      throw new Error("Unexpected response data format");
    }

    const resp = responseData as SearchTxsResponse;

    if (tryToDecrypt) {
      for (let i = 0; i < resp.txs.length; i++) {
        resp.txs[i] = await this.decryptTxsResponse(resp.txs[i]);
      }
    }

    return resp;
  }

  /** returns the amino-encoding of the transaction performed by the server */
  public async encodeTx(tx: CosmosSdkTx): Promise<Uint8Array> {
    const responseData = await this.post("/txs/encode", tx);
    if (!(responseData as any).tx) {
      throw new Error("Unexpected response data format");
    }
    return Encoding.fromBase64((responseData as EncodeTxResponse).tx);
  }

  /**
   * Broadcasts a signed transaction to into the transaction pool.
   * Depending on the RestClient's broadcast mode, this might or might
   * wait for checkTx or deliverTx to be executed before returning.
   *
   * @param tx a signed transaction as StdTx (i.e. not wrapped in type/value container)
   */
  public async postTx(tx: StdTx): Promise<PostTxsResponse> {
    const params = {
      tx: tx,
      mode: this.broadcastMode,
    };
    const responseData = await this.post("/txs", params);
    if (!(responseData as any).txhash) {
      throw new Error("Unexpected response data format");
    }
    return responseData as PostTxsResponse;
  }

  // The /wasm endpoints

  // wasm rest queries are listed here: https://github.com/cosmwasm/wasmd/blob/master/x/wasm/client/rest/query.go#L19-L27
  public async listCodeInfo(): Promise<readonly CodeInfo[]> {
    const path = `/wasm/code`;
    const responseData = (await this.get(path)) as WasmResponse<CosmosSdkArray<CodeInfo>>;
    return normalizeArray(await unwrapWasmResponse(responseData));
  }

  // this will download the original wasm bytecode by code id
  // throws error if no code with this id
  public async getCode(id: number): Promise<CodeDetails> {
    const path = `/wasm/code/${id}`;
    const responseData = (await this.get(path)) as WasmResponse<CodeDetails>;
    return await unwrapWasmResponse(responseData);
  }

  public async listContractsByCodeId(id: number): Promise<readonly ContractInfo[]> {
    const path = `/wasm/code/${id}/contracts`;
    const responseData = (await this.get(path)) as WasmResponse<CosmosSdkArray<ContractInfo>>;
    return normalizeArray(await unwrapWasmResponse(responseData));
  }

  public async getCodeHashByCodeId(id: number): Promise<string> {
    const codeHashFromCache = this.codeHashCache.get(id);
    if (typeof codeHashFromCache === "string") {
      return codeHashFromCache;
    }

    const path = `/wasm/code/${id}/hash`;
    const responseData = (await this.get(path)) as ContractHashResponse;

    this.codeHashCache.set(id, responseData.result);
    return responseData.result;
  }

  public async getCodeHashByContractAddr(addr: string): Promise<string> {
    const codeHashFromCache = this.codeHashCache.get(addr);
    if (typeof codeHashFromCache === "string") {
      return codeHashFromCache;
    }

    const path = `/wasm/contract/${addr}/code-hash`;
    const responseData = (await this.get(path)) as ContractHashResponse;

    this.codeHashCache.set(addr, responseData.result);
    return responseData.result;
  }

  /**
   * Returns null when contract was not found at this address.
   */
  public async getContractInfo(address: string): Promise<ContractDetails | null> {
    const path = `/wasm/contract/${address}`;
    const response = (await this.get(path)) as WasmResponse<ContractDetails | null>;
    return await unwrapWasmResponse(response);
  }

  /**
   * Makes a smart query on the contract and parses the reponse as JSON.
   * Throws error if no such contract exists, the query format is invalid or the response is invalid.
   */
  public async queryContractSmart(
    contractAddress: string,
    query: object,
    addedParams?: object,
    contractCodeHash?: string,
  ): Promise<JsonObject> {
    if (!contractCodeHash) {
      contractCodeHash = await this.getCodeHashByContractAddr(contractAddress);
    } else {
      this.codeHashCache.set(contractAddress, contractCodeHash);
    }

    const encrypted = await this.enigmautils.encrypt(contractCodeHash, query);
    const nonce = encrypted.slice(0, 32);

    const encoded = Encoding.toBase64(encrypted).replace(/\+/g, "-").replace(/\//g, "_");

    // @ts-ignore
    const paramString = new URLSearchParams(addedParams).toString();

    const encodedContractAddress = Encoding.toBase64(Bech32.decode(contractAddress).data);

    const path = `/compute/v1beta1/contract/${encodedContractAddress}/smart?query_data=${encoded}&${paramString}`;

    let responseData;
    try {
      responseData = (await this.get(path)) as SmartQueryResponse;
    } catch (err) {
      const errorMessageRgx = /encrypted: (.+?): (?:instantiate|execute|query) contract failed/g;
      const rgxMatches = errorMessageRgx.exec(err.message);
      if (rgxMatches == null || rgxMatches?.length != 2) {
        throw err;
      }

      try {
        const errorCipherB64 = rgxMatches[1];
        const errorCipherBz = Encoding.fromBase64(errorCipherB64);

        const errorPlainBz = await this.enigmautils.decrypt(errorCipherBz, nonce);

        err.message = err.message.replace(errorCipherB64, Encoding.fromUtf8(errorPlainBz));
      } catch (decryptionError) {
        throw new Error(`Failed to decrypt the following error message: ${err.message}.`);
      }

      throw err;
    }

    // By convention, smart queries must return a valid JSON document (see https://github.com/CosmWasm/cosmwasm/issues/144)
    return JSON.parse(
      Encoding.fromUtf8(
        Encoding.fromBase64(
          Encoding.fromUtf8(
            await this.enigmautils.decrypt(Encoding.fromBase64(responseData.data), nonce),
          ),
        ),
      ),
    );
  }

  /**
   * Get the consensus keypair for IO encryption
   */
  public async getMasterCerts(address: string, query: object): Promise<any> {
    return this.get("/register/master-cert");
  }

  public async decryptDataField(dataField = "", nonces: Array<Uint8Array>): Promise<Uint8Array> {
    const wasmOutputDataCipherBz = Encoding.fromHex(dataField);

    let error;
    for (const nonce of nonces) {
      try {
        const data = Encoding.fromBase64(
          Encoding.fromUtf8(await this.enigmautils.decrypt(wasmOutputDataCipherBz, nonce)),
        );

        return data;
      } catch (e) {
        error = e;
      }
    }

    throw error;
  }

  public async decryptLogs(logs: readonly Log[], nonces: Array<Uint8Array>): Promise<readonly Log[]> {
    for (const l of logs) {
      for (const e of l.events) {
        if (e.type === "wasm") {
          for (const nonce of nonces) {
            let nonceOk = false;
            for (const a of e.attributes) {
              try {
                a.key = Encoding.fromUtf8(await this.enigmautils.decrypt(Encoding.fromBase64(a.key), nonce));
                nonceOk = true;
              } catch (e) {}
              try {
                a.value = Encoding.fromUtf8(
                  await this.enigmautils.decrypt(Encoding.fromBase64(a.value), nonce),
                );
                nonceOk = true;
              } catch (e) {}
            }
            if (nonceOk) {
              continue;
            }
          }
        }
      }
    }

    return logs;
  }

  public async decryptTxsResponse(txsResponse: TxsResponse): Promise<TxsResponse> {
    let dataFields;
    let data = Uint8Array.from([]);
    if (txsResponse.data) {
      dataFields = decodeTxData(Encoding.fromHex(txsResponse.data));
    }

    let logs: Log[] | undefined = txsResponse.logs;

    if (logs) {
      logs[0].msg_index = 0;
    }

    for (let i = 0; i < txsResponse.tx.value.msg?.length; i++) {
      const msg: Msg = txsResponse.tx.value.msg[i];

      let inputMsgEncrypted: Uint8Array;
      if (msg.type === "wasm/MsgExecuteContract") {
        inputMsgEncrypted = Encoding.fromBase64((msg as MsgExecuteContract).value.msg);
      } else if (msg.type === "wasm/MsgInstantiateContract") {
        inputMsgEncrypted = Encoding.fromBase64((msg as MsgInstantiateContract).value.init_msg);
      } else {
        continue;
      }

      const inputMsgPubkey = inputMsgEncrypted.slice(32, 64);
      if (Encoding.toBase64(await this.enigmautils.getPubkey()) === Encoding.toBase64(inputMsgPubkey)) {
        // my pubkey, can decrypt
        const nonce = inputMsgEncrypted.slice(0, 32);

        // decrypt input
        const inputMsg = Encoding.fromUtf8(
          await this.enigmautils.decrypt(inputMsgEncrypted.slice(64), nonce),
        );

        if (msg.type === "wasm/MsgExecuteContract") {
          // decrypt input
          (txsResponse.tx.value.msg[i] as MsgExecuteContract).value.msg = inputMsg;

          // decrypt output data
          // stupid workaround because only 1st message data is returned
          if (dataFields && i == 0 && dataFields[0].data) {
            data = await this.decryptDataField(Encoding.toHex(Encoding.fromBase64(dataFields[0].data)), [
              nonce,
            ]);
          }
        } else if (msg.type === "wasm/MsgInstantiateContract") {
          // decrypt input
          (txsResponse.tx.value.msg[i] as MsgInstantiateContract).value.init_msg = inputMsg;
        }

        // decrypt output logs
        if (txsResponse.logs && logs) {
          if (!txsResponse.logs[i]?.log) {
            logs[i].log = "";
          }
          logs[i] = (await this.decryptLogs([txsResponse.logs[i]], [nonce]))[0];
        }
        // failed to execute message; message index: 0: encrypted: (.+?): (?:instantiate|execute|query) contract failed
        // decrypt error
        const errorMessageRgx = new RegExp(
          `failed to execute message; message index: ${i}: encrypted: (.+?): (?:instantiate|execute|query) contract failed`,
          "g",
        );

        const rgxMatches = errorMessageRgx.exec(txsResponse.raw_log);

        if (Array.isArray(rgxMatches) && rgxMatches?.length === 2) {
          const errorCipherB64 = rgxMatches[1];
          const errorCipherBz = Encoding.fromBase64(errorCipherB64);

          const errorPlainBz = await this.enigmautils.decrypt(errorCipherBz, nonce);

          txsResponse.raw_log = txsResponse.raw_log.replace(errorCipherB64, Encoding.fromUtf8(errorPlainBz));
        }
      }
    }

    txsResponse = Object.assign({}, txsResponse, { logs: logs });
    // @ts-ignore
    txsResponse.data = data;

    return txsResponse;
  }
}
