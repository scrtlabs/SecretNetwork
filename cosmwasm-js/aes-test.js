(async () => {
  const miscreant = require("miscreant");
  const softProvider = new miscreant.PolyfillCryptoProvider();
  const key = await miscreant.SIV.importKey(new Uint8Array(new Array(32).fill(0x7)), "AES-SIV", softProvider);

  const plaintext = new TextEncoder("utf-8").encode("banana");
  const ciphertext = await siv.seal(plaintext);

  const decrypted = await key.open(ciphertext);
  console.log(decrypted);
})();
