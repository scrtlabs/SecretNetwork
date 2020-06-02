const miscreant = require("miscreant");
const cryptoProvider = new miscreant.PolyfillCryptoProvider();

if (typeof process === "object") {
  // we're in nodejs
  const util = require("util");
  var TextEncoder = util.TextEncoder;
  var TextDecoder = util.TextDecoder;

  var btoa = function (u8: Uint8Array): string {
    return Buffer.from(u8).toString("base64");
  };

  var atob = function (str: string): Uint8Array {
    return Uint8Array.from(new Buffer(str, "base64"));
  };

  // var atob = function (str: String): Uint8Array {};
}

export async function encrypt(msg: object): Promise<string> {
  const key = Uint8Array.from(new Array(32).fill(0x7));
  const siv = await miscreant.SIV.importKey(key, "AES-SIV", cryptoProvider);

  const msgAsStr = JSON.stringify(msg);
  const plaintext = new TextEncoder("utf-8").encode(msgAsStr);

  const ciphertext = await siv.seal(plaintext, [new Uint8Array()]);

  // ad = nonce(32)|wallet_pubkey(33) = 65 bytes
  const ad = Uint8Array.from(new Array(65).fill(0x0));

  return btoa(Uint8Array.from([...ad, ...ciphertext]));
}

export async function decrypt(ciphertext: Uint8Array): Promise<Uint8Array> {
  const key = Uint8Array.from(new Array(32).fill(0x7));
  const siv = await miscreant.SIV.importKey(key, "AES-SIV", cryptoProvider);

  const plaintext = await siv.open(ciphertext, [new Uint8Array()]);
  return plaintext;
}

module.exports = {
  encrypt,
  decrypt,
};
