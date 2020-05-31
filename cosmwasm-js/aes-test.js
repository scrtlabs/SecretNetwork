const forge = require("node-forge");

let key = forge.util.createBuffer(new Uint8Array(new Array(32).fill(0x7)).buffer);
let iv = forge.util.createBuffer(new Uint8Array(new Array(12).fill(0x0)).buffer);

const input = forge.util.createBuffer();
input.putString("banana");

var cipher = forge.cipher.createCipher("AES-GCM", key);
cipher.start({ iv: iv });
cipher.update(input);
cipher.finish();
var encrypted = cipher.output;
// outputs encrypted hex
console.log(encrypted.putBuffer(cipher.mode.tag));

key = forge.util.createBuffer(new Uint8Array(new Array(32).fill(0x7)).buffer);
iv = forge.util.createBuffer(new Uint8Array(new Array(12).fill(0x0)).buffer);

var decipher = forge.cipher.createDecipher("AES-GCM", key);
decipher.start({ iv: iv, tag: cipher.mode.tag });
decipher.update(cipher.output);
var result = decipher.finish();
console.log(decipher.output.data);
