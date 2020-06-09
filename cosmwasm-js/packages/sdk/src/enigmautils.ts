const miscreant = require("miscreant");
import { sharedKey as x25519, generateKeyPair } from "curve25519-js";
import { Encoding } from "@iov/encoding";
const secureRandom = require("secure-random");
import axios from "axios";
const hkdf = require("js-crypto-hkdf");

const cryptoProvider = new miscreant.PolyfillCryptoProvider();

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

const hkdfSalt: Uint8Array = Uint8Array.from([
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

export default class EnigmaUtils {
  private readonly apiUrl: string;
  private consensusIoPubKey: Uint8Array = new Uint8Array(); // cache

  public constructor(apiUrl: string) {
    this.apiUrl = apiUrl;
  }

  private getTxSenderKeyPair(): { privkey: Uint8Array; pubkey: Uint8Array } {
    if (!localStorage.getItem("tx_sender_privkey") || !localStorage.getItem("tx_sender_pubkey")) {
      const seedFor25519 = secureRandom(32, { type: "Uint8Array" });
      const { private: privkey, public: pubkey } = generateKeyPair(seedFor25519);

      localStorage.setItem("tx_sender_privkey", Encoding.toHex(privkey));
      localStorage.setItem("tx_sender_pubkey", Encoding.toHex(pubkey));
    }

    const privkey = Encoding.fromHex(localStorage.getItem("tx_sender_privkey"));
    const pubkey = Encoding.fromHex(localStorage.getItem("tx_sender_pubkey"));

    // TODO verify pubkey

    return { privkey, pubkey };
  }

  private async getConsensusIoPubKey(): Promise<Uint8Array> {
    if (this.consensusIoPubKey.length === 32) {
      return this.consensusIoPubKey;
    }

    const {
      data: {
        result: { ioExchPubkey },
      },
    } = await axios.get(this.apiUrl + "/reg/consensus-io-exch-pubkey", {
      headers: { "Content-Type": "application/json" },
    });

    this.consensusIoPubKey = Encoding.fromBase64(ioExchPubkey);
    return this.consensusIoPubKey;
  }

  private async getTxEncryptionKey(txSenderPrivKey: Uint8Array, nonce: Uint8Array): Promise<Uint8Array> {
    const consensusIoPubKey = await this.getConsensusIoPubKey();

    const txEncryptionIkm = x25519(txSenderPrivKey, consensusIoPubKey);
    const { key: txEncryptionKey } = await hkdf.compute(
      Uint8Array.from([...txEncryptionIkm, ...nonce]),
      "SHA-256",
      32,
      "",
      hkdfSalt,
    );
    return txEncryptionKey;
  }

  public async encrypt(msg: object): Promise<Uint8Array> {
    const { privkey: txSenderPrivKey, pubkey: txSenderPubKey } = this.getTxSenderKeyPair();

    const nonce = secureRandom(32, {
      type: "Uint8Array",
    });

    const txEncryptionKey = await this.getTxEncryptionKey(txSenderPrivKey, nonce);

    const siv = await miscreant.SIV.importKey(txEncryptionKey, "AES-SIV", cryptoProvider);

    const plaintext = Encoding.toUtf8(JSON.stringify(msg));

    const ciphertext = await siv.seal(plaintext, [new Uint8Array()]);

    // ciphertext = nonce(32) || wallet_pubkey(32) || ciphertext
    return Uint8Array.from([...nonce, ...txSenderPubKey, ...ciphertext]);
  }

  public async decrypt(ciphertext: Uint8Array, nonce: Uint8Array): Promise<Uint8Array> {
    const { privkey: txSenderPrivKey } = this.getTxSenderKeyPair();
    const txEncryptionKey = await this.getTxEncryptionKey(txSenderPrivKey, nonce);

    const siv = await miscreant.SIV.importKey(txEncryptionKey, "AES-SIV", cryptoProvider);

    const plaintext = await siv.open(ciphertext, [new Uint8Array()]);
    return plaintext;
  }

  public getMyPubkey(): Uint8Array {
    const { pubkey } = this.getTxSenderKeyPair();
    return pubkey;
  }
}

module.exports = EnigmaUtils;
