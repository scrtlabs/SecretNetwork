const miscreant = require("miscreant");
import { sharedKey as x25519, generateKeyPair } from "curve25519-js";
import { Encoding } from "@iov/encoding";
const secureRandom = require("secure-random");
import axios from "axios";
const hkdf = require("js-crypto-hkdf");

const cryptoProvider = new miscreant.PolyfillCryptoProvider();
const { fromBase64, fromUtf8, toBase64, toUtf8, toHex, fromHex } = Encoding;

// const ALICE_PRIV = '77076d0a7318a57d3c16c17251b26645df4c2f87ebc0992ab177fba51db92c2a';
// const BOB_PUB = 'de9edb7d7b7dc1b4d35b61c2ece435373f8343c85b78674dadfc7e146f882b4f';

// const alicePriv = Uint8Array.from(Buffer.from(ALICE_PRIV, 'hex'));

// const bobPub = Uint8Array.from(Buffer.from(BOB_PUB, 'hex'));

// const secret = sharedKey(alicePriv, bobPub);

// console.log('Secret:', Buffer.from(secret).toString('hex'))

//var bytes = secureRandom(10, {type: 'Uint8Array'}) //return a Uint8Array of 10 bytes

if (typeof process === "object") {
  // nodejs
  const LocalStorage = require("node-localstorage").LocalStorage;
  const homedir = require("os").homedir();
  const path = require("path");

  var fs = require("fs");
  var dir = path.join(homedir, ".cosmwasmjs");

  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir);
  }

  var localStorage = new LocalStorage(path.join(dir, "id_tx_io.json"));
}

function getTxSenderKeyPair(): { privkey: Uint8Array; pubkey: Uint8Array } {
  if (!localStorage.getItem("tx_sender_privkey") || !localStorage.getItem("tx_sender_pubkey")) {
    const seedFor25519 = secureRandom(32, { type: "Uint8Array" });
    const { private: privkey, public: pubkey } = generateKeyPair(seedFor25519);

    localStorage.setItem("tx_sender_privkey", toHex(privkey));
    localStorage.setItem("tx_sender_pubkey", toHex(pubkey));
  }

  const privkey = fromHex(localStorage.getItem("tx_sender_privkey"));
  const pubkey = fromHex(localStorage.getItem("tx_sender_pubkey"));

  // TODO verify pubkey

  return { privkey, pubkey };
}

const hkdfSalt = Uint8Array.from([
  0x00,
  0x00,
  0x00,
  0x00,
  0x00,
  0x00,
  0x00,
  0x00,
  0x00,
  0x02,
  0x4b,
  0xea,
  0xd8,
  0xdf,
  0x69,
  0x99,
  0x08,
  0x52,
  0xc2,
  0x02,
  0xdb,
  0x0e,
  0x00,
  0x97,
  0xc1,
  0xa1,
  0x2e,
  0xa6,
  0x37,
  0xd7,
  0xe9,
  0x6d,
]);

async function getTxEncryptionKey(
  consensusIoPubKey: Uint8Array,
  txSenderPrivKey: Uint8Array,
  nonce: Uint8Array,
): Promise<Uint8Array> {
  const txEncryptionIkm = x25519(txSenderPrivKey, consensusIoPubKey);

  return hkdf.compute(Uint8Array.from([...txEncryptionIkm, ...nonce]), "SHA-256", 32, "", hkdfSalt);
}

export async function encrypt(msg: object, consensusIoPubKey: Uint8Array): Promise<string> {
  const { privkey: txSenderPrivKey, pubkey: txSenderPubKey } = getTxSenderKeyPair();

  const nonce = secureRandom(32, {
    type: "Uint8Array",
  });

  const txEncryptionKey = await getTxEncryptionKey(consensusIoPubKey, txSenderPrivKey, nonce);

  const siv = await miscreant.SIV.importKey(txEncryptionKey, "AES-SIV", cryptoProvider);

  const plaintext = toUtf8(JSON.stringify(msg));

  const ciphertext = await siv.seal(plaintext, [new Uint8Array()]);

  // ciphertext = nonce(32) || wallet_pubkey(32) || ciphertext
  return toBase64(Uint8Array.from([...nonce, ...txSenderPubKey, ...ciphertext]));
}

export async function decrypt(consensusIoPubKey: Uint8Array, ciphertext: Uint8Array): Promise<Uint8Array> {
  const key = Uint8Array.from(new Array(32).fill(0x7));
  const siv = await miscreant.SIV.importKey(key, "AES-SIV", cryptoProvider);

  const plaintext = await siv.open(ciphertext, [new Uint8Array()]);
  return plaintext;
}

module.exports = {
  encrypt,
  decrypt,
};
