# Implementing the Secret in Contracts

:warning: This is a very advanced WIP.

- [Implementing the Secret in Contracts](#implementing-the-secret-in-contracts)
- [Bootstrap Process](#bootstrap-process)
  - [`consensus_seed`](#consensus_seed)
  - [Key Derivation](#key-derivation)
    - [`consensus_seed_exchange_privkey`](#consensus_seed_exchange_privkey)
    - [`consensus_io_exchange_privkey`](#consensus_io_exchange_privkey)
    - [`consensus_state_ikm`](#consensus_state_ikm)
    - [`consensus_state_iv`](#consensus_state_iv)
  - [Bootstrap Process Epilogue](#bootstrap-process-epilogue)
- [Node Startup](#node-startup)
- [New Node Registration](#new-node-registration)
  - [On the new node](#on-the-new-node)
  - [On the consensus layer, inside the Enclave of every full node](#on-the-consensus-layer-inside-the-enclave-of-every-full-node)
    - [`seed_exchange_key`](#seed_exchange_key)
    - [Sharing `consensus_seed` with the new node](#sharing-consensus_seed-with-the-new-node)
  - [Back on the new node, inside its Enclave](#back-on-the-new-node-inside-its-enclave)
    - [`seed_exchange_key`](#seed_exchange_key-1)
    - [Decrypting `encrypted_consensus_seed`](#decrypting-encrypted_consensus_seed)
  - [New Node Registration Epilogue](#new-node-registration-epilogue)
- [Contracts State Encryption](#contracts-state-encryption)
  - [`contract_key`](#contract_key)
  - [write_db(field_name, value)](#write_dbfield_name-value)
  - [read_db(field_name)](#read_dbfield_name)
- [Transaction Encryption](#transaction-encryption)
  - [Input](#input)
    - [On the transaction sender](#on-the-transaction-sender)
    - [On the consensus layer, inside the Enclave of every full node](#on-the-consensus-layer-inside-the-enclave-of-every-full-node-1)
  - [Output](#output)
    - [On the consensus layer, inside the Enclave of every full node](#on-the-consensus-layer-inside-the-enclave-of-every-full-node-2)
    - [Back on the transaction sender](#back-on-the-transaction-sender)
- [Blockchain Upgrades](#blockchain-upgrades)
- [Theoretical Attacks](#theoretical-attacks)

# Bootstrap Process

Before the genesis of a new chain, there must be a bootstrap node to generate network-wide secrets to fuel all the privacy features of the chain.

## `consensus_seed`

- Create a remote attestation proof that the node's Enclave is genuine.
- Generate inside the Enclave a true random 256 bits seed: `consensus_seed`.
- Seal `consensus_seed` with MRENCLAVE to a local file: `$HOME/.enigmad/sgx-secrets/consensus_seed.sealed`.

```js
// 256 bits
consensus_seed = true_random({ bytes: 32 });

seal({
  key: "MRENCLAVE",
  data: consensus_seed,
  to_file: "$HOME/.enigmad/sgx-secrets/consensus_seed.sealed",
});
```

## Key Derivation

TODO reasoning

- Key Derivation inside the Enclave is done deterministically using HKDF-SHA256 [[1]](https://tools.ietf.org/html/rfc5869#section-2)[[2]](https://en.wikipedia.org/wiki/HKDF).
- The HKDF-SHA256 [salt](https://tools.ietf.org/html/rfc5869#section-3.1) is chosen to be Bitcoin's halving block hash.

```js
hkfd_salt = sha256(
  0x000000000000000000024bead8df69990852c202db0e0097c1a12ea637d7e96d
);
```

- Using HKDF-SHA256, `hkfd_salt` and `consensus_seed`, derive the following keys:

### `consensus_seed_exchange_privkey`

- `consensus_seed_exchange_privkey`: A secp256k1 curve private key. Will be used to derive encryption keys in order to securely share `consensus_seed` with new nodes in the network.
- From `consensus_seed_exchange_privkey` calculate `consensus_seed_exchange_pubkey`.

```js
consensus_seed_exchange_privkey = hkdf({
  salt: hkfd_salt,
  ikm: consensus_seed.append(uint8(1)),
}); // 256 bits

consensus_seed_exchange_pubkey = calculate_secp256k1_pubkey(
  consensus_seed_exchange_privkey
);
```

### `consensus_io_exchange_privkey`

- `consensus_io_exchange_privkey`: A secp256k1 curve private key. Will be used to derive encryption keys in order to decrypt transaction inputs and encrypt transaction outputs.
- From `consensus_io_exchange_privkey` calculate `consensus_io_exchange_pubkey`.

```js
consensus_io_exchange_privkey = hkdf({
  salt: hkfd_salt,
  ikm: consensus_seed.append(uint8(2)),
}); // 256 bits

consensus_io_exchange_pubkey = calculate_secp256k1_pubkey(
  consensus_io_exchange_privkey
);
```

### `consensus_state_ikm`

- `consensus_state_ikm`: An input keying material (IKM) for HKDF-SHA256 to derive encryption keys for contracts' state.

```js
consensus_state_ikm = hkdf({
  salt: hkfd_salt,
  ikm: consensus_seed.append(uint8(3)),
}); // 256 bits
```

### `consensus_state_iv`

TODO reasoning

- `consensus_state_iv`: An input secret IV to prevent IV manipulation while encrypting contracts' state.

```js
consensus_state_iv = hkdf({
  salt: hkfd_salt,
  ikm: consensus_seed.append(uint8(4)),
}); // 256 bits
```

## Bootstrap Process Epilogue

TODO reasoning

- Seal `consensus_seed` to disk at `"$HOME/.enigmad/sgx-secrets/consensus_seed.sealed"`.
- Publish to `genesis.json`:
  - The remote attestation proof that the Enclave is genuine.
  - `consensus_seed_exchange_pubkey`
  - `consensus_io_exchange_pubkey`

# Node Startup

When a full node resumes its participation in the network, it reads `consensus_seed` from `"$HOME/.enigmad/sgx-secrets/consensus_seed.sealed"` and again does [key derivation](#Key-Derivation) as outlined above.

# New Node Registration

A new node wants to join the network as a full node.

## On the new node

TODO reasoning

- Verify the remote attestation proof of the bootstrap node from `genesis.json`.
- Create a remote attestation proof that the node's Enclave is genuine.
- Generate inside the node's Enclave a true random secp256k1 curve private key: `registration_privkey`.
- From `registration_privkey` calculate `registration_pubkey`.
- Send an `enigmacli tx register auth` transaction with the following inputs:
  - The remote attestation proof that the node's Enclave is genuine.
  - `registration_pubkey`
  - 256 bits true random `nonce`
  - 256 bits true random `iv`

## On the consensus layer, inside the Enclave of every full node

TODO reasoning

- Receive the `enigmacli tx register auth` transaction.
- Verify the remote attestation proof that the new node's Enclave is genuine.

### `seed_exchange_key`

TODO reasoning

- `seed_exchange_key`: An [AES-128-SIV](https://tools.ietf.org/html/rfc5297) encryption key. Will be used to send `consensus_seed` to the new node.
- AES-128-SIV was chosen to prevent IV misuse by client libraries.
  - https://tools.ietf.org/html/rfc5297
  - https://github.com/miscreant/meta
  - The input key is 256 bits, but half of it is used to derive the internal IV.
- `seed_exchange_key` is derived the following way:
  - `seed_exchange_ikm` is derived using [ECDH](https://en.wikipedia.org/wiki/Elliptic-curve_Diffie%E2%80%93Hellman) with `consensus_seed_exchange_privkey` and `registration_pubkey`.
  - `seed_exchange_key` is derived using HKDF-SHA256 from `seed_exchange_ikm` and `nonce`.

```js
seed_exchange_ikm = ecdh({
  privkey: consensus_seed_exchange_privkey,
  pubkey: registration_pubkey,
}); // 256 bits

seed_exchange_key = hkdf({
  salt: hkfd_salt,
  ikm: concat(seed_exchange_ikm, nonce),
}); // 256 bits
```

### Sharing `consensus_seed` with the new node

TODO reasoning

- The output of the `enigmacli tx register auth` transaction is `consensus_seed` encrypted with AES-128-SIV, `seed_exchange_key` as the encryption key, using the public key of the registering node for the AD.

```js
encrypted_consensus_seed = aes_128_siv_encrypt({
  key: seed_exchange_key,
  data: consensus_seed,
  ad: new_node_public_key,
});

return encrypted_consensus_seed;
```

## Back on the new node, inside its Enclave

- Receive the `enigmacli tx register auth` transaction output (`encrypted_consensus_seed`).

### `seed_exchange_key`

TODO reasoning

- `seed_exchange_key`: An AES-128-SIV encryption key. Will be used to decrypt `consensus_seed`.
- `seed_exchange_key` is derived the following way:

  - `seed_exchange_ikm` is derived using [ECDH](https://en.wikipedia.org/wiki/Elliptic-curve_Diffie%E2%80%93Hellman) with `consensus_seed_exchange_pubkey` (public in `genesis.json`) and `registration_privkey` (available only inside the new node's Enclave).

  - `seed_exchange_key` is derived using HKDF-SHA256 with `seed_exchange_ikm` and `nonce`.

```js
seed_exchange_ikm = ecdh({
  privkey: registration_privkey,
  pubkey: consensus_seed_exchange_pubkey, // from genesis.json
}); // 256 bits

seed_exchange_key = hkdf({
  salt: hkfd_salt,
  ikm: concat(seed_exchange_ikm, nonce),
}); // 256 bits
```

### Decrypting `encrypted_consensus_seed`

TODO reasoning

- `encrypted_consensus_seed` is encrypted with AES-128-SIV, `seed_exchange_key` as the encryption key and the public key of the registering node as the `ad` as the decryption additional data.
- The new node now has all of these^ parameters inside its Enclave, so it's able to decrypt `consensus_seed` from `encrypted_consensus_seed`.
- Seal `consensus_seed` to disk at `"$HOME/.enigmad/sgx-secrets/consensus_seed.sealed"`.

```js
consensus_seed = aes_128_siv_decrypt({
  key: seed_exchange_key,
  data: encrypted_consensus_seed,
  ad: new_node_public_key,
});

seal({
  key: "MRENCLAVE",
  data: consensus_seed,
  to_file: "$HOME/.enigmad/sgx-secrets/consensus_seed.sealed",
});
```

## New Node Registration Epilogue

TODO reasoning

- The new node can now do [key derivation](#key-derivation) to get all the required network-wide secrets in order participate in blocks execution and validation.
- After a machine/process reboot, the node can go through the [node startup process](#node-startup) again.

# Contracts State Encryption

TODO reasoning

- While executing a function call inside the Enclave as part of a transaction, the contract code can call `write_db(field_name, value)` and `read_db(field_name)`.
- Contracts' state is stored on-chain inside a key-value store, thus the `field_name` must remain constant between calls.
- `encryption_key` is derived using HKDF-SHA256 from:
  - `consensus_state_ikm`
  - `field_name`
  - `contact_key`

## `contract_key`

- `contract_key` is a concatenation of two values: `signer_id || authenticated_contract_key`.
- When a contract is deployed (i.e., on contract init), `contract_key` is generated inside of the Enclave as follows:

```js
signer_id = sha256(concat(msg_sender, block_height));

authentication_key = hkdf({
  salt: hkfd_salt,
  info: "contract_key",
  ikm: concat(consensus_state_ikm, signer_id),
});

authenticated_contract_key = hmac_sha256({
  key: authentication_key,
  data: code_hash,
});

contract_key = concat(signer_id, authenticated_contract_key);
```

- Every time a contract execution is called, `contract_key` should be sent to the Enclave.
- In the Enclave, the following verification needs to happen:

```js
signer_id = contract_key.slice(0, 32);
expected_contract_key = contract_key.slice(32, 64);

authentication_key = hkdf({
  salt: hkfd_salt,
  info: "contract_key",
  ikm: concat(consensus_state_ikm, signer_id),
});

calculated_contract_key = hmac_sha256({
  key: authentication_key,
  data: code_hash,
});

assert(calculated_contract_key == expected_contract_key);
```

## write_db(field_name, value)

```js
encryption_key = hkdf({
  salt: hkfd_salt,
  ikm: concat(consensus_state_ikm, field_name, contract_key),
});

encrypted_field_name = aes_128_siv_encrypt({
  key: encryption_key,
  data: field_name,
});

current_state_ciphertext = internal_read_db(encrypted_field_name);

if (current_state_ciphertext == null) {
  // field_name doesn't yet initialized in state
  ad = sha256(encrypted_field_name);
} else {
  // read previous_ad, verify it, calculate new iv
  previous_ad = current_state_ciphertext.slice(0, 32); // first 32 bytes/256 bits
  current_state_ciphertext = current_state_ciphertext.slice(32); // skip first 32 bytes

  aes_128_siv_decrypt({
    key: encryption_key,
    data: current_state_ciphertext,
    ad: previous_ad,
  }); // just to authenticate previous_iv
  ad = sha256(previous_ad);
}

new_state_ciphertext = aes_128_siv_encrypt({
  key: encryption_key,
  data: value,
  ad: ad,
});

new_state = concat(ad, new_state_ciphertext);

internal_write_db(encrypted_field_name, new_state);
```

## read_db(field_name)

```js
encryption_key = hkdf({
  salt: hkfd_salt,
  ikm: concat(consensus_state_ikm, field_name, contract_key),
});

encrypted_field_name = aes_128_siv_encrypt({
  key: encryption_key,
  data: field_name,
});

current_state_ciphertext = internal_read_db(encrypted_field_name);

if (current_state_ciphertext == null) {
  // field_name doesn't yet initialized in state
  return null;
}

// read ad, verify it
ad = current_state_ciphertext.slice(0, 32); // first 32 bytes/256 bits
current_state_ciphertext = current_state_ciphertext.slice(32); // skip first 32 bytes
current_state_plaintext = aes_128_siv_decrypt({
  key: encryption_key,
  data: current_state_ciphertext,
  ad: ad,
});

return current_state_plaintext;
```

# Transaction Encryption

TODO reasoning

- `tx_encryption_key`: An AES-128-SIV encryption key. Will be used to encrypt tx inputs and decrypt tx outpus.
  - `tx_encryption_ikm` is derived using [ECDH](https://en.wikipedia.org/wiki/Elliptic-curve_Diffie%E2%80%93Hellman) with `consensus_io_exchange_pubkey` and `tx_sender_wallet_privkey` (on the sender's side).
  - `tx_encryption_ikm` is derived using [ECDH](https://en.wikipedia.org/wiki/Elliptic-curve_Diffie%E2%80%93Hellman) with `consensus_io_exchange_privkey` and `tx_sender_wallet_pubkey` (inside the Enclave of every full node).
- `tx_encryption_key` is derived using HKDF-SHA256 with `tx_encryption_ikm` and a random number `nonce`. This is to prevent using the same key for the same tx sender multiple times.

## Input

### On the transaction sender

```js
tx_encryption_ikm = ecdh({
  privkey: tx_sender_wallet_privkey,
  pubkey: consensus_io_exchange_pubkey, // from genesis.json
}); // 256 bits

nonce = true_random({ bytes: 32 });

tx_encryption_key = hkdf({
  salt: hkfd_salt,
  ikm: concat(tx_encryption_ikm, nonce),
}); // 256 bits

ad = concat(nonce, tx_sender_wallet_pubkey);

encrypted_msg = aes_128_siv_encrypt({
  key: tx_encryption_key,
  data: msg,
  ad: ad,
});

tx_input = concat(ad, encrypted_msg);
```

### On the consensus layer, inside the Enclave of every full node

```js
nonce = tx_input.slice(0, 32); // 32 bytes
tx_sender_wallet_pubkey = tx_input.slice(32, 65); // 33 bytes, compressed secp256k1 public key
encrypted_msg = tx_input.slice(65);

tx_encryption_ikm = ecdh({
  privkey: consensus_io_exchange_privkey,
  pubkey: tx_sender_wallet_pubkey,
}); // 256 bits

tx_encryption_key = hkdf({
  salt: hkfd_salt,
  ikm: concat(tx_encryption_ikm, nonce),
}); // 256 bits

msg = aes_128_siv_decrypt({
  key: tx_encryption_key,
  data: encrypted_msg,
  ad: concat(nonce, tx_sender_wallet_pubkey), // or: tx_input.slice(0, 65)
});
```

## Output

- The output must be a valid JSON object, as it is passed to multiple mechanisms for final processing:
  - Logs are treated as Tendermint events
  - Messages can be callbacks to another contract call
  - Messages can also instruct to send funds from the contract's wallet
  - There's a data section which is free form bytes to be inerperted by the client (or dApp)
  - And there's also an error section
- Therefore the output must be partialy-encrypted.
- An example output for an execution:
  ```js
  {
    "ok": {
      "messages": [
        {
          "type": "Send",
          "to": "...",
          "amount": "..."
        },
        {
          "type": "Contract",
          "msg": "{\"banana\":1,\"papaya\":2}", // need to encrypt this value
          "contract_addr": "aaa",
          "send": { "amount": 100, "denom": "uscrt" }
        },
        {
          "type": "Contract",
          "msg": "{\"water\":1,\"fire\":2}", // need to encrypt this value
          "contract_addr": "bbb",
          "send": { "amount": 0, "denom": "uscrt" }
        }
      ],
      "log": [
        {
          "key": "action", // need to encrypt this value
          "value": "transfer" // need to encrypt this value
        },
        {
          "key": "sender", // need to encrypt this value
          "value": "enigma1v9tna8rkemndl7cd4ahru9t7ewa7kdq8d4zlr5" // need to encrypt this value
        },
        {
          "key": "recipient", // need to encrypt this value
          "value": "enigma1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rp5vqd4" // need to encrypt this value
        }
      ],
      "data": "bla bla" // need to encrypt this value
    }
  }
  ```
- Notice on a `Contract` message, the `msg` value should be the same `msg` as in our `tx_input`, so we need to prepend the `nonce` and `tx_sender_wallet_pubkey` just like we did on the tx sender above.
- For the rest of the encrypted outputs we only need to send the ciphertext, as the tx sender can get `consensus_io_exchange_prubkey` from `genesis.json` and `nonce` from the `tx_input` that is attached to the `tx_output`.
- An example output with an error:
  ```js
  {
    "err": "{\"watermelon\":6,\"coffee\":5}" // need to encrypt this value
  }
  ```
- An example output for a query:
  ```js
  {
    "ok": "{\"answer\":42}" // need to encrypt this value
  }
  ```

### On the consensus layer, inside the Enclave of every full node

```js
// already have from tx_input:
// - tx_encryption_key
// - nonce

if (typeof output["err"] == "string") {
  encrypted_err = aes_128_siv_encrypt({
    key: tx_encryption_key,
    data: output["err"],
  });
  output["err"] = base64_encode(encrypted_err); // needs to be a JSON string
} else if (typeof output["ok"] == "string") {
  // query
  // output["ok"] is handled the same way as output["err"]...
  encrypted_query_result = aes_128_siv_encrypt({
    key: tx_encryption_key,
    data: output["ok"],
  });
  output["ok"] = base64_encode(encrypted_query_result); // needs to be a JSON string
} else if (typeof output["ok"] == "object") {
  // execute
  for (m in output["ok"]["messages"]) {
    if (m["type"] == "Contract") {
      encrypted_msg = aes_128_siv_encrypt({
        key: tx_encryption_key,
        data: m["msg"],
        ad: concat(nonce, tx_sender_wallet_pubkey),
      });

      // base64_encode because needs to be a string
      // also turns into a tx_input so we also need to prepend nonce and tx_sender_wallet_pubkey
      m["msg"] = base64_encode(
        concat(nonce, tx_sender_wallet_pubkey, encrypted_msg)
      );
    }
  }

  for (l in output["ok"]["log"]) {
    // l["key"] is handled the same way as output["err"]...
    encrypted_log_key_name = aes_128_siv_encrypt({
      key: tx_encryption_key,
      data: l["key"],
    });
    l["key"] = base64_encode(encrypted_log_key_name); // needs to be a JSON string

    // l["value"] is handled the same way as output["err"]...
    encrypted_log_value = aes_128_siv_encrypt({
      key: tx_encryption_key,
      data: l["value"],
    });
    l["value"] = base64_encode(encrypted_log_value); // needs to be a JSON string
  }

  // output["ok"]["data"] is handled the same way as output["err"]...
  encrypted_output_data = aes_128_siv_encrypt({
    key: tx_encryption_key,
    data: output["ok"]["data"],
  });
  output["ok"]["data"] = base64_encode(encrypted_output_data); // needs to be a JSON string
}

return output;
```

### Back on the transaction sender

- The transaction output is written to the chain
- Only the wallet with the right `tx_sender_wallet_privkey` can derive `tx_encryption_key`, so for everyone else it will just be encrypted.
- Every encrypted value can be decrypted the following way:

```js
// output["err"]
// output["ok"]["data"]
// output["ok"]["log"][i]["key"]
// output["ok"]["log"][i]["value"]
// output["ok"] if input is a query

encrypted_bytes = base64_encode(encrypted_output);

aes_128_siv_decrypt({
  key: tx_encryption_key,
  data: encrypted_bytes,
});
```

- For `output["ok"]["messages"][i]["type"] == "Contract"`, `output["ok"]["messages"][i]["msg"]` will be decrypted in [this](#on-the-consensus-layer-inside-the-enclave-of-every-full-node-1) manner by the consensus layer when it handles the contract callback.

# Blockchain Upgrades

TODO

# Theoretical Attacks

TODO
