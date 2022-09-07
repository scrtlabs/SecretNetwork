import { sha256 } from "@noble/hashes/sha256";
import {
  SecretNetworkClient,
  toHex,
  toUtf8,
  Wallet,
  MsgStoreCode,
  MsgInstantiateContract,
  Tx,
  TxResultCode,
} from "secretjs";
import { State as ConnectionState } from "secretjs/dist/protobuf_stuff/ibc/core/connection/v1/connection";
import { State as ChannelState } from "secretjs/dist/protobuf_stuff/ibc/core/channel/v1/channel";

export class Contract {
  address: string;
  codeId: number;
  ibcPortId: string;
  codeHash: string;
  version: string;

  constructor(version) {
    this.version = version;
  }
}

interface BytesObj {
  [key: string]: number;
}

const bytesToKv = (input: BytesObj) => {
  let output = "";
  for (const v of Object.values(input)) {
    output += String.fromCharCode(v);
  }

  return output;
};

const objToKv = (input) => {
  // console.log("got object:", input);
  const output = {};
  const key = bytesToKv(input.key);
  output[key] = bytesToKv(input.value);
  return output;
};

export const cleanBytes = (tx) => {
  // console.log("input:", JSON.stringify(testInput, null, 2), "\n\n");

  const events = tx.events.map((e) => {
    return {
      ...e,
      attributes: e.attributes.map((i) => objToKv(i)),
    };
  });

  const output = {
    ...tx,
    events,
  };

  // these fields clutter the output too much
  output.txBytes = "redacted";
  output.tx.authInfo = "redacted";
  output.tx.body.messages.forEach((m) => (m.value.wasmByteCode = "redacted"));

  // console.log("output:", JSON.stringify(output, null, 2));
  return output;
};

export const ibcDenom = (
  paths: {
    portId: string;
    channelId: string;
  }[],
  coinMinimalDenom: string
): string => {
  const prefixes = [];
  for (const path of paths) {
    prefixes.push(`${path.portId}/${path.channelId}`);
  }

  const prefix = prefixes.join("/");
  const denom = `${prefix}/${coinMinimalDenom}`;

  return "ibc/" + toHex(sha256(toUtf8(denom))).toUpperCase();
};

export async function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export async function waitForBlocks(chainId: string) {
  const secretjs = await SecretNetworkClient.create({
    grpcWebUrl: "http://localhost:9091",
    chainId,
  });

  console.log(`Waiting for blocks on ${chainId}...`);
  while (true) {
    try {
      const { block } = await secretjs.query.tendermint.getLatestBlock({});

      if (Number(block?.header?.height) >= 1) {
        console.log(`Current block on ${chainId}: ${block!.header!.height}`);
        break;
      }
    } catch (e) {
      // console.error("block error:", e);
    }
    await sleep(100);
  }
}

export async function waitForIBCConnection(
  chainId: string,
  grpcWebUrl: string
) {
  const secretjs = await SecretNetworkClient.create({
    grpcWebUrl,
    chainId,
  });

  console.log("Waiting for open connections on", chainId + "...");
  while (true) {
    try {
      const { connections } = await secretjs.query.ibc_connection.connections(
        {}
      );

      if (
        connections.length >= 1 &&
        connections[0].state === ConnectionState.STATE_OPEN
      ) {
        console.log("Found an open connection on", chainId);
        break;
      }
    } catch (e) {
      // console.error("IBC error:", e, "on chain", chainId);
    }
    await sleep(100);
  }
}

export async function waitForIBCChannel(
  chainId: string,
  grpcWebUrl: string,
  channelId: string
) {
  const secretjs = await SecretNetworkClient.create({
    grpcWebUrl,
    chainId,
  });

  console.log(`Waiting for ${channelId} on ${chainId}...`);
  outter: while (true) {
    try {
      const { channels } = await secretjs.query.ibc_channel.channels({});

      for (const c of channels) {
        if (c.channelId === channelId && c.state == ChannelState.STATE_OPEN) {
          console.log(`${channelId} is open on ${chainId}`);
          break outter;
        }
      }
    } catch (e) {
      // console.error("IBC error:", e, "on chain", chainId);
    }
    await sleep(100);
  }
}

export async function storeContracts(
  account: SecretNetworkClient,
  wasms: Uint8Array[]
) {
  const tx: Tx = await account.tx.broadcast(
    [
      new MsgStoreCode({
        sender: account.address,
        wasmByteCode: wasms[0],
        source: "",
        builder: "",
      }),
      new MsgStoreCode({
        sender: account.address,
        wasmByteCode: wasms[1],
        source: "",
        builder: "",
      }),
    ],
    { gasLimit: 5_000_000 }
  );

  if (tx.code !== TxResultCode.Success) {
    console.error(tx.rawLog);
  }
  expect(tx.code).toBe(TxResultCode.Success);

  return tx;
}

export async function instantiateContracts(
  account: SecretNetworkClient,
  contracts: Contract[]
) {
  const tx: Tx = await account.tx.broadcast(
    [
      new MsgInstantiateContract({
        sender: account.address,
        codeId: contracts[0].codeId,
        codeHash: contracts[0].codeHash,
        initMsg: { nop: {} },
        label: `v1-${Date.now()}`,
      }),
      new MsgInstantiateContract({
        sender: account.address,
        codeId: contracts[1].codeId,
        codeHash: contracts[1].codeHash,
        initMsg: { nop: {} },
        label: `v010-${Date.now()}`,
      }),
    ],
    { gasLimit: 300_000 }
  );
  if (tx.code !== TxResultCode.Success) {
    console.error(tx.rawLog);
  }
  expect(tx.code).toBe(TxResultCode.Success);

  return tx;
}
