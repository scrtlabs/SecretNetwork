import { sha256 } from "@noble/hashes/sha256";
import {toHex, toUtf8} from "secretjs";

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

  return (
    "ibc/" +
    toHex(sha256(toUtf8(denom)))
      .toUpperCase()
  );
};