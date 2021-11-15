const miscreant = require("miscreant");
import { Encoding } from "@iov/encoding";
import { generateKeyPair, sharedKey as x25519 } from "curve25519-js";
const secureRandom = require("secure-random");
import axios from "axios";
const hkdf = require("js-crypto-hkdf");

const cryptoProvider = new miscreant.PolyfillCryptoProvider();

export interface SecretUtils {
  getPubkey: () => Promise<Uint8Array>;
  decrypt: (ciphertext: Uint8Array, nonce: Uint8Array) => Promise<Uint8Array>;
  encrypt: (contractCodeHash: string, msg: object) => Promise<Uint8Array>;
  getTxEncryptionKey: (nonce: Uint8Array) => Promise<Uint8Array>;
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

export default class EnigmaUtils implements SecretUtils {
  private readonly apiUrl: string;
  public readonly seed: Uint8Array;
  private readonly privkey: Uint8Array;
  public readonly pubkey: Uint8Array;
  private consensusIoPubKey: Uint8Array = new Uint8Array(); // cache

  public constructor(apiUrl: string, seed?: Uint8Array) {
    this.apiUrl = apiUrl;
    if (!seed) {
      this.seed = EnigmaUtils.GenerateNewSeed();
    } else {
      this.seed = seed;
    }
    const { privkey, pubkey } = EnigmaUtils.GenerateNewKeyPairFromSeed(this.seed);
    this.privkey = privkey;
    this.pubkey = pubkey;
  }

  public static GenerateNewKeyPair(): { privkey: Uint8Array; pubkey: Uint8Array } {
    return EnigmaUtils.GenerateNewKeyPairFromSeed(EnigmaUtils.GenerateNewSeed());
  }

  public static GenerateNewSeed(): Uint8Array {
    return secureRandom(32, { type: "Uint8Array" });
  }

  public static GenerateNewKeyPairFromSeed(seed: Uint8Array): { privkey: Uint8Array; pubkey: Uint8Array } {
    const { private: privkey, public: pubkey } = generateKeyPair(seed);
    return { privkey, pubkey };
  }

  private async getConsensusIoPubKey(): Promise<Uint8Array> {
    if (this.consensusIoPubKey.length === 32) {
      return this.consensusIoPubKey;
    }

    const {
      data: {
        result: { TxKey },
      },
    } = await axios.get(this.apiUrl + "/reg/tx-key", {
      headers: { "Content-Type": "application/json" },
    });

    this.consensusIoPubKey = Encoding.fromBase64(TxKey);
    return this.consensusIoPubKey;
  }

  public async getTxEncryptionKey(nonce: Uint8Array): Promise<Uint8Array> {
    const consensusIoPubKey = await this.getConsensusIoPubKey();

    const txEncryptionIkm = x25519(this.privkey, consensusIoPubKey);
    const { key: txEncryptionKey } = await hkdf.compute(
      Uint8Array.from([...txEncryptionIkm, ...nonce]),
      "SHA-256",
      32,
      "",
      hkdfSalt,
    );
    return txEncryptionKey;
  }

  public async encrypt(contractCodeHash: string, msg: object): Promise<Uint8Array> {
    const nonce = secureRandom(32, {
      type: "Uint8Array",
    });

    const txEncryptionKey = await this.getTxEncryptionKey(nonce);

    const siv = await miscreant.SIV.importKey(txEncryptionKey, "AES-SIV", cryptoProvider);

    const plaintext = Encoding.toUtf8(contractCodeHash + JSON.stringify(msg));

    const ciphertext = await siv.seal(plaintext, [new Uint8Array()]);

    // ciphertext = nonce(32) || wallet_pubkey(32) || ciphertext
    return Uint8Array.from([...nonce, ...this.pubkey, ...ciphertext]);
  }

  public async decrypt(ciphertext: Uint8Array, nonce: Uint8Array): Promise<Uint8Array> {
    if (!ciphertext?.length) {
      return new Uint8Array();
    }

    const txEncryptionKey = await this.getTxEncryptionKey(nonce);

    //console.log(`decrypt tx encryption key: ${Encoding.toHex(txEncryptionKey)}`);

    const siv = await miscreant.SIV.importKey(txEncryptionKey, "AES-SIV", cryptoProvider);

    const plaintext = await siv.open(ciphertext, [new Uint8Array()]);
    return plaintext;
  }

  getPubkey(): Promise<Uint8Array> {
    return Promise.resolve(this.pubkey);
  }
}

module.exports = EnigmaUtils;
