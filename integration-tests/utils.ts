import { sha256 } from "@noble/hashes/sha256";
import { SecretNetworkClient, toHex, toUtf8 } from "secretjs";

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
export async function waitForBlocks() {
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
export async function waitForIBC(chainId: string, grpcWebUrl: string) {
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
