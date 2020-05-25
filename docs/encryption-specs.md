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
  - [`contract_id`](#contract_id)
  - [write_db(field_name, value)](#write_dbfield_name-value)
  - [read_db(field_name)](#read_dbfield_name)
- [Transaction Encryption](#transaction-encryption)
  - [Input](#input)
    - [On the transaction sender](#on-the-transaction-sender)
    - [On the consensus layer, inside the Enclave of every full node](#on-the-consensus-layer-inside-the-enclave-of-every-full-node-1)
  - [Output](#output)
    - [On the consensus layer, inside the Enclave of every full node](#on-the-consensus-layer-inside-the-enclave-of-every-full-node-2)
    - [On the transaction sender](#on-the-transaction-sender-1)
- [Blockchain Upgrades](#blockchain-upgrades)
- [Theoretical Attacks](#theoretical-attacks)

# Bootstrap Process

Before the genesis of a new chain, there most be a bootstrap node to generate network-wide secrets to fuel all the privacy features of the chain.

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
- Generate inside the node's Enclave a true random secp256k1 curve private key: `new_node_seed_exchange_privkey`.
- From `new_node_seed_exchange_privkey` calculate `new_node_seed_exchange_pubkey`.
- Send an `enigmacli tx register auth` transaction with the following inputs:
  - The remote attestation proof that the node's Enclave is genuine.
  - `new_node_seed_exchange_pubkey`
  - 256 bits true random `nonce`
  - 256 bits true random `iv`

## On the consensus layer, inside the Enclave of every full node

TODO reasoning

- Receive the `enigmacli tx register auth` transaction.
- Verify the remote attestation proof that the new node's Enclave is genuine.

### `seed_exchange_key`

TODO reasoning

- `seed_exchange_key`: An AES-256-GCM encryption key. Will be used to send `consensus_seed` to the new node.
- `seed_exchange_key` is derived the following way:
  - `seed_exchange_ikm` is derived using [ECDH](https://en.wikipedia.org/wiki/Elliptic-curve_Diffie%E2%80%93Hellman) with `consensus_seed_exchange_privkey` and `new_node_seed_exchange_pubkey`.
  - `seed_exchange_key` is derived using HKDF-SHA256 from `seed_exchange_ikm` and `nonce`.

```js
seed_exchange_ikm = ecdh({
  privkey: consensus_seed_exchange_privkey,
  pubkey: new_node_seed_exchange_pubkey,
}); // 256 bits

seed_exchange_key = hkdf({
  salt: hkfd_salt,
  ikm: concat(seed_exchange_ikm, nonce),
}); // 256 bits
```

### Sharing `consensus_seed` with the new node

TODO reasoning

- The output of the `enigmacli tx register auth` transaction is `consensus_seed` encrypted with AES-256-GCM, `seed_exchange_key` as the encryption key and `iv` as the encryption IV.

```js
encrypted_consensus_seed = aes_256_gcm_encrypt({
  iv: iv,
  key: seed_exchange_key,
  data: consensus_seed,
  aad: iv,
});

return encrypted_consensus_seed;
```

## Back on the new node, inside its Enclave

- Receive the `enigmacli tx register auth` transaction output (`encrypted_consensus_seed`).

### `seed_exchange_key`

TODO reasoning

- `seed_exchange_key`: An AES-256-GCM encryption key. Will be used to receive `consensus_seed` from the network.
- `seed_exchange_key` is derived the following way:
  - `seed_exchange_ikm` is derived using [ECDH](https://en.wikipedia.org/wiki/Elliptic-curve_Diffie%E2%80%93Hellman) with `consensus_seed_exchange_pubkey` (public in `genesis.json`) and `new_node_seed_exchange_privkey` (available only inside the new node's Enclave).
  - `seed_exchange_key` is derived using HKDF-SHA256 with `seed_exchange_ikm` and `nonce`.

```js
seed_exchange_ikm = ecdh({
  privkey: new_node_seed_exchange_privkey,
  pubkey: consensus_seed_exchange_pubkey, // from genesis.json
}); // 256 bits

seed_exchange_key = hkdf({
  salt: hkfd_salt,
  ikm: concat(seed_exchange_ikm, nonce),
}); // 256 bits
```

### Decrypting `encrypted_consensus_seed`

TODO reasoning

- `encrypted_consensus_seed` is encrypted with AES-256-GCM, `seed_exchange_key` as the encryption key and `iv` as the encryption IV.
- The new node now has all of these^ parameters inside its Enclave, so it's able to decrypt `consensus_seed` from `encrypted_consensus_seed`.
- Seal `consensus_seed` to disk at `"$HOME/.enigmad/sgx-secrets/consensus_seed.sealed"`.

```js
consensus_seed = aes_256_gcm_decrypt({
  iv: iv,
  key: seed_exchange_key,
  data: encrypted_consensus_seed,
  aad: iv,
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
- Contracts' state is store on-chain inside a key-value store, thus the `field_name` must remain constant between calls.
- Good encryption doesn't use the same `encryption_key` and `iv` together more than once. This means that encrypting the same input twice yields different outputs, and therefore we cannot encrypt the `field_name` because the next time we want to query it we won't know where to look for it.
- `encryption_key` is derived using HKDF-SHA256 from:
  - `consensus_state_ikm`
  - `field_name`
  - `contact_id`
- Ciphertext is prepended with the `iv` so that the next read will be able to decrypt it. `iv` is also authenticated with the AES-256-GCM AAD.
- `iv` is derive from `sha256(consensus_state_iv || value || previous_iv)` in order to prevent tx rollback attacks that can force `iv` and `encryption_key` reuse. This also prevents using the same `iv` in different instances of the same contract. `consensus_state_iv` prevents exposing `value` by comparing `iv` to `previos_iv`.

## `contract_id`

- `contract_id` is a concatenation of two values: `contract_id_payload || encrypted_contract_id_payload`.
- When a contract is deployed (i.e., on contract init), `contract_id` is generated inside of the enclave as follows:

```js
contract_id_payload = sha256(concat(msg_sender, block_height, contract_code));

encryption_key = hkdf({
  salt: hkfd_salt,
  ikm: concat(consensus_state_ikm, contract_id_payload),
});

iv = sha256(concat(consensus_state_iv, contract_id_payload)).slice(0, 12); // truncate because iv is only 96 bits

encrypted_contract_id_payload = aes_256_gcm_encrypt({
  iv: iv,
  key: encryption_key,
  data: null,
  aad: concat(contract_id_payload, code_hash, iv),
});

contract_id = concat(contract_id_payload, encrypted_contract_id_payload, iv);
```

- Every time a contract execution is called, `contract_id` should be sent to the enclave.
- In the enclave, the following verification needs to happen:

```js
contract_id_payload = contract_id.slice(0, 32);
encrypted_contract_id_payload = contract_id.slice(32, 64);
iv = contract_id.slice(64);

encryption_key = hkdf({
  salt: hkfd_salt,
  ikm: concat(consensus_state_ikm,contract_id_payload),
});

(interpreted_payload, interpreted_code_hash)  = aes_256_gcm_decrypt({
  iv: iv,
  key: encryption_key,
  data: encrypted_contract_id_payload
});

assert(interpreted_payload == contract_id_payload);
assert(interpreted_code_hash == sha256(contract_code);
```

## write_db(field_name, value)

```js
current_state_ciphertext = internal_read_db(field_name);

encryption_key = hkdf({
  salt: hkfd_salt,
  ikm: concat(consensus_state_ikm, field_name, contact_id),
});

if (current_state_ciphertext == null) {
  // field_name doesn't yet initialized in state
  iv = sha256(concat(consensus_state_iv, value)).slice(0, 12); // truncate because iv is only 96 bits
} else {
  // read previous_iv, verify it, calculate new iv
  previous_iv = current_state_ciphertext.slice(0, 12); // first 12 bytes
  current_state_ciphertext = current_state_ciphertext.slice(12); // skip first 12 bytes

  aes_256_gcm_decrypt({
    iv: previous_iv,
    key: encryption_key,
    data: current_state_ciphertext,
    aad: previous_iv,
  }); // just to authenticate previous_iv
  iv = sha256(concat(consensus_state_iv, value, previous_iv)).slice(0, 12); // truncate because iv is only 96 bits
}

new_state_ciphertext = aes_256_gcm_encrypt({
  iv: iv,
  key: encryption_key,
  data: value,
  aad: iv,
});

new_state = concat(iv, new_state_ciphertext);

internal_write_db(field_name, new_state);
```

## read_db(field_name)

```js
current_state_ciphertext = internal_read_db(field_name);

encryption_key = hkdf({
  salt: hkfd_salt,
  ikm: concat(consensus_state_ikm, field_name, sha256(contract_wasm_binary)), // TODO diffrentiate between same binaries for different contracts
});

if (current_state_ciphertext == null) {
  // field_name doesn't yet initialized in state
  return null;
}

// read iv, verify it, calculate new iv
iv = current_state_ciphertext.slice(0, 12); // first 12 bytes
current_state_ciphertext = current_state_ciphertext.slice(12); // skip first 12 bytes
current_state_plaintext = aes_256_gcm_decrypt({
  iv: iv,
  key: encryption_key,
  data: current_state_ciphertext,
  aad: iv,
});

return current_state_plaintext;
```

# Transaction Encryption

TODO reasoning

- `tx_encryption_key`: An AES-256-GCM encryption key. Will be used to encrypt tx inputs and decrypt tx outpus.
  - `tx_encryption_ikm` is derived using [ECDH](https://en.wikipedia.org/wiki/Elliptic-curve_Diffie%E2%80%93Hellman) with `consensus_io_exchange_pubkey` and `tx_sender_wallet_privkey` (on the sender's side).
  - `tx_encryption_ikm` is derived using [ECDH](https://en.wikipedia.org/wiki/Elliptic-curve_Diffie%E2%80%93Hellman) with `consensus_io_exchange_privkey` and `tx_sender_wallet_pubkey` (inside the enclave of every full node).
- `tx_encryption_key` is derived using HKDF-SHA256 with `tx_encryption_ikm` and a random number `nonce`. This is to prevent using the same key for the same tx sender multiple times.
- `iv_input` for the input is randomly generated on the client side by the transation sender.
- `iv`s for the output are derived from `iv_input` using HKDF-SHA256.

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

iv_input = true_random({ bytes: 12 });

aad = concat(iv_input, nonce, tx_sender_wallet_pubkey);

encrypted_msg = aes_256_gcm_encrypt({
  iv: iv_input,
  key: tx_encryption_key,
  data: msg,
  aad: aad,
});

tx_input = concat(aad, encrypted_msg);
```

### On the consensus layer, inside the Enclave of every full node

```js
iv_input = tx_input.slice(0, 12); // 12 bytes
nonce = tx_input.slice(12, 44); // 32 bytes
tx_sender_wallet_pubkey = tx_input.slice(44, 77); // 33 bytes, compressed secp256k1 public key
encrypted_msg = tx_input.slice(77);

tx_encryption_ikm = ecdh({
  privkey: consensus_io_exchange_privkey,
  pubkey: tx_sender_wallet_pubkey,
}); // 256 bits

tx_encryption_key = hkdf({
  salt: hkfd_salt,
  ikm: concat(tx_encryption_ikm, nonce),
}); // 256 bits

msg = aes_256_gcm_decrypt({
  iv: iv_input,
  key: tx_encryption_key,
  data: encrypted_msg,
  aad: concat(iv_input, nonce, tx_sender_wallet_pubkey), // or: tx_input.slice(0, 77)
});
```

## Output

- The output must be a valid JSON object, as it is passed to multiple mechanisms for final processing:
  - Logs are treated as Tendermint events
  - Messages can be callbacks to another contract call or to
  - Messages can also instruct to send funds from the contract's wallet
  - There's a data section which is free form bytes to be inerperted by the client (or dApp)
  - And there's also an error section
- Therefore the output must be part-encrypted, so we need to use a new `iv` for each part.
- We'll use HKDF-SHA256 in combination with the `input_iv` and a counter to derive a new `iv` for each part.
- An example output for an execution:
  ```json
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
- Notice on a `Contract` message, the `msg` value is the same as our `tx_input`, so we need to prepend the new `iv_input`, the `nonce` and `tx_sender_wallet_pubkey` just like we did on the tx sender.
- For the rest of the encrypted outputs we ony need to prepend the new `iv` for each encrypted value, as the tx sender can get `consensus_io_exchange_prubkey` from `genesis.json` and nonce from the `tx_input` the is attached to the `tx_output`.
- An example output with an error:
  ```json
  {
    "err": "{\"watermelon\":6,\"coffee\":5}" // need to encrypt this value
  }
  ```
- An example output for a query:
  ```json
  {
    "ok": "{\"answer\":42}" // need to encrypt this value
  }
  ```

### On the consensus layer, inside the Enclave of every full node

```js
// already have from tx_input:
// - tx_encryption_key
// - iv_input
// - nonce

iv_counter = 1;

if (typeof output["err"] == "string") {
  iv = hkdf({
    salt: hkfd_salt,
    ikm: concat(input_iv, [iv_counter]),
  }).slice(0, 12); // 96 bits
  iv_counter += 1;

  encrypted_err = aes_256_gcm_encrypt({
    iv: iv,
    key: tx_encryption_key,
    data: output["err"],
    aad: iv,
  });

  output["err"] = base64_encode(concat(iv, encrypted_err)); // needs to be a string
} else if (typeof output["ok"] == "string") {
  // query
  // same as output["err"]...
} else if (typeof output["ok"] == "object") {
  // execute
  for (m in output["ok"]["messages"]) {
    if (m["type"] == "Contract") {
      iv_input = hkdf({
        salt: hkfd_salt,
        ikm: concat(input_iv, [iv_counter]),
      }).slice(0, 12); // 96 bits
      iv_counter += 1;

      encrypted_msg = aes_256_gcm_encrypt({
        iv: iv,
        key: tx_encryption_key,
        data: m["msg"],
        aad: concat(iv_input, nonce, tx_sender_wallet_pubkey),
      });

      // base64_encode because needs to be a string
      // also turns into a tx_input so we also need to prepend iv_input, nonce and tx_sender_wallet_pubkey
      m["msg"] = base64_encode(
        concat(iv_input, nonce, tx_sender_wallet_pubkey, encrypted_msg)
      );
    }
  }

  for (l in output["ok"]["log"]) {
    // l["key"] is the same as output["err"]...
    // l["value"] is the same as output["err"]...
  }

  // output["ok"]["data"] is the same as output["err"]...
}

return output;
```

### On the transaction sender

TODO ??

# Blockchain Upgrades

TODO reasoning

# Theoretical Attacks
