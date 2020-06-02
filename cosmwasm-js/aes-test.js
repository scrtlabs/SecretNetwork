(async () => {
  const miscreant = require("miscreant");
  const softProvider = new miscreant.PolyfillCryptoProvider();

  const key = new Uint8Array(new Array(32).fill(0x7));
  const siv = await miscreant.SIV.importKey(key, "AES-SIV", softProvider);

  const plaintext = new Uint8Array([1, 2, 3, 4, 5, 6]);
  const sealed = await siv.seal(plaintext, []);
  console.log(sealed);

  const unsealed = await siv.open(sealed, []);
  console.log(unsealed);
})();
