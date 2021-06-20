# Encryption

- [Encryption](#encryption)
- [Bootstrap Process](#bootstrap-process)
  - [`consensus_seed`](#consensus_seed)
  - [Key Derivation](#key-derivation)
    - [`consensus_seed_exchange_privkey`](#consensus_seed_exchange_privkey)
    - [`consensus_io_exchange_privkey`](#consensus_io_exchange_privkey)
    - [`consensus_state_ikm`](#consensus_state_ikm)
    - [`consensus_callback_secret`](#consensus_callback_secret)
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
  - [remove_db(field_name)](#remove_dbfield_name)
- [Transaction Encryption](#transaction-encryption)
  - [Input](#input)
    - [On the transaction sender](#on-the-transaction-sender)
    - [On the consensus layer, inside the Enclave of every full node](#on-the-consensus-layer-inside-the-enclave-of-every-full-node-1)
  - [Output](#output)
    - [On the consensus layer, inside the Enclave of every full node](#on-the-consensus-layer-inside-the-enclave-of-every-full-node-2)
    - [Back on the transaction sender](#back-on-the-transaction-sender)
- [Blockchain Upgrades](#blockchain-upgrades)
- [Theoretical Attacks](#theoretical-attacks)
  - [Deanonymizing with ciphertext byte count](#deanonymizing-with-ciphertext-byte-count)
  - [Two contracts with the same `contract_key` could deanonymize each other's states](#two-contracts-with-the-same-contract_key-could-deanonymize-each-others-states)
  - [Tx Replay attacks](#tx-replay-attacks)
  - [More Advanced Tx Replay attacks -- search to decision for Millionaire's problem](#more-advanced-tx-replay-attacks----search-to-decision-for-millionaires-problem)
  - [Partial storage rollback during contract runtime](#partial-storage-rollback-during-contract-runtime)
  - [Tx outputs can leak data](#tx-outputs-can-leak-data)

# Bootstrap Process

Before the genesis of a new chain, there must be a bootstrap node to generate network-wide secrets to fuel all the privacy features of the chain.

## `consensus_seed`

- Create a remote attestation proof that the node's Enclave is genuine.
- Generate inside the Enclave a true random 256 bits seed: `consensus_seed`.
- Seal `consensus_seed` with MRSIGNER to a local file: `$HOME/.sgx_secrets/consensus_seed.sealed`.

```js
// 256 bits
consensus_seed = true_random({ bytes: 32 });

seal({
  key: "MRSIGNER",
  data: consensus_seed,
  to_file: "$HOME/.sgx_secrets/consensus_seed.sealed",
});
```

## Key Derivation

TODO reasoning

- Key Derivation inside the Enclave is done deterministically using HKDF-SHA256 [[1]](https://tools.ietf.org/html/rfc5869#section-2)[[2]](https://en.wikipedia.org/wiki/HKDF).
- The HKDF-SHA256 [salt](https://tools.ietf.org/html/rfc5869#section-3.1) is chosen to be Bitcoin's halving block hash.

```js
hkdf_salt = 0x000000000000000000024bead8df69990852c202db0e0097c1a12ea637d7e96d;
```

- Using HKDF-SHA256, `hkdf_salt` and `consensus_seed`, derive the following keys:

### `consensus_seed_exchange_privkey`

- `consensus_seed_exchange_privkey`: A curve25519 private key. Will be used to derive encryption keys in order to securely share `consensus_seed` with new nodes in the network.
- From `consensus_seed_exchange_privkey` calculate `consensus_seed_exchange_pubkey`.

```js
consensus_seed_exchange_privkey = hkdf({
  salt: hkdf_salt,
  ikm: consensus_seed.append(uint8(1)),
}); // 256 bits

consensus_seed_exchange_pubkey = calculate_curve25519_pubkey(
  consensus_seed_exchange_privkey
);
```

### `consensus_io_exchange_privkey`

- `consensus_io_exchange_privkey`: A curve25519 private key. Will be used to derive encryption keys in order to decrypt transaction inputs and encrypt transaction outputs.
- From `consensus_io_exchange_privkey` calculate `consensus_io_exchange_pubkey`.

```js
consensus_io_exchange_privkey = hkdf({
  salt: hkdf_salt,
  ikm: consensus_seed.append(uint8(2)),
}); // 256 bits

consensus_io_exchange_pubkey = calculate_curve25519_pubkey(
  consensus_io_exchange_privkey
);
```

### `consensus_state_ikm`

- `consensus_state_ikm`: An input keyring material (IKM) for HKDF-SHA256 to derive encryption keys for contracts' state.

```js
consensus_state_ikm = hkdf({
  salt: hkdf_salt,
  ikm: consensus_seed.append(uint8(3)),
}); // 256 bits
```

### `consensus_callback_secret`

- `consensus_callback_secret`: A curve25519 private key. Will be used to create callback signatures when contracts call other contracts.

```js
consensus_state_ikm = hkdf({
  salt: hkdf_salt,
  ikm: consensus_seed.append(uint8(4)),
}); // 256 bits
```

## Bootstrap Process Epilogue

TODO reasoning

- Seal `consensus_seed` to disk at `$HOME/.sgx_secrets/consensus_seed.sealed`.
- Publish to `genesis.json`:
  - The remote attestation proof that the Enclave is genuine.
  - `consensus_seed_exchange_pubkey`
  - `consensus_io_exchange_pubkey`

# Node Startup

When a full node resumes its participation in the network, it reads `consensus_seed` from `$HOME/.sgx_secrets/consensus_seed.sealed` and again does [key derivation](#Key-Derivation) as outlined above.

# New Node Registration

A new node wants to join the network as a full node.

## On the new node

TODO reasoning

- Verify the remote attestation proof of the bootstrap node from `genesis.json`.
- Create a remote attestation proof that the node's Enclave is genuine.
- Generate inside the node's Enclave a true random curve25519 private key: `registration_privkey`.
- From `registration_privkey` calculate `registration_pubkey`.
- Send an `secretcli tx register auth` transaction with the following inputs:
  - The remote attestation proof that the node's Enclave is genuine.
  - `registration_pubkey`
  - 256 bits true random `nonce`

## On the consensus layer, inside the Enclave of every full node

TODO reasoning

- Receive the `secretcli tx register auth` transaction.
- Verify the remote attestation proof that the new node's Enclave is genuine.

### `seed_exchange_key`

TODO reasoning

- `seed_exchange_key`: An [AES-128-SIV](https://tools.ietf.org/html/rfc5297) encryption key. Will be used to send `consensus_seed` to the new node.
- AES-128-SIV was chosen to prevent IV misuse by client libraries.
  - https://tools.ietf.org/html/rfc5297
  - https://github.com/miscreant/meta
  - The input key is 256 bits, but half of it is used to derive the internal IV.
  - AES-SIV does not pad the cipertext, and this leaks information about the plaintext data, specifically its size.
- `seed_exchange_key` is derived the following way:
  - `seed_exchange_ikm` is derived using [ECDH](https://en.wikipedia.org/wiki/Elliptic-curve_Diffie%E2%80%93Hellman) ([x25519](https://tools.ietf.org/html/rfc7748#section-6)) with `consensus_seed_exchange_privkey` and `registration_pubkey`.
  - `seed_exchange_key` is derived using HKDF-SHA256 from `seed_exchange_ikm` and `nonce`.

```js
seed_exchange_ikm = ecdh({
  privkey: consensus_seed_exchange_privkey,
  pubkey: registration_pubkey,
}); // 256 bits

seed_exchange_key = hkdf({
  salt: hkdf_salt,
  ikm: concat(seed_exchange_ikm, nonce),
}); // 256 bits
```

### Sharing `consensus_seed` with the new node

TODO reasoning

- The output of the `secretcli tx register auth` transaction is `consensus_seed` encrypted with AES-128-SIV, `seed_exchange_key` as the encryption key, using the public key of the registering node for the AD.

```js
encrypted_consensus_seed = aes_128_siv_encrypt({
  key: seed_exchange_key,
  data: consensus_seed,
  ad: new_node_public_key,
});

return encrypted_consensus_seed;
```

## Back on the new node, inside its Enclave

- Receive the `secretcli tx register auth` transaction output (`encrypted_consensus_seed`).

### `seed_exchange_key`

TODO reasoning

- `seed_exchange_key`: An AES-128-SIV encryption key. Will be used to decrypt `consensus_seed`.
- `seed_exchange_key` is derived the following way:

  - `seed_exchange_ikm` is derived using [ECDH](https://en.wikipedia.org/wiki/Elliptic-curve_Diffie%E2%80%93Hellman) ([x25519](https://tools.ietf.org/html/rfc7748#section-6)) with `consensus_seed_exchange_pubkey` (public in `genesis.json`) and `registration_privkey` (available only inside the new node's Enclave).

  - `seed_exchange_key` is derived using HKDF-SHA256 with `seed_exchange_ikm` and `nonce`.

```js
seed_exchange_ikm = ecdh({
  privkey: registration_privkey,
  pubkey: consensus_seed_exchange_pubkey, // from genesis.json
}); // 256 bits

seed_exchange_key = hkdf({
  salt: hkdf_salt,
  ikm: concat(seed_exchange_ikm, nonce),
}); // 256 bits
```

### Decrypting `encrypted_consensus_seed`

TODO reasoning

- `encrypted_consensus_seed` is encrypted with AES-128-SIV, `seed_exchange_key` as the encryption key and the public key of the registering node as the `ad` as the decryption additional data.
- The new node now has all of these^ parameters inside its Enclave, so it's able to decrypt `consensus_seed` from `encrypted_consensus_seed`.
- Seal `consensus_seed` to disk at `$HOME/.sgx_secrets/consensus_seed.sealed`.

```js
consensus_seed = aes_128_siv_decrypt({
  key: seed_exchange_key,
  data: encrypted_consensus_seed,
  ad: new_node_public_key,
});

seal({
  key: "MRSIGNER",
  data: consensus_seed,
  to_file: "$HOME/.sgx_secrets/consensus_seed.sealed",
});
```

## New Node Registration Epilogue

TODO reasoning

- The new node can now do [key derivation](#key-derivation) to get all the required network-wide secrets in order participate in blocks execution and validation.
- After a machine/process reboot, the node can go through the [node startup process](#node-startup) again.

# Contracts State Encryption

TODO reasoning

- While executing a function call inside the Enclave as part of a transaction, the contract code can call `write_db(field_name, value)`, `read_db(field_name)` and `remove_db(field_name)`.
- Contracts' state is stored on-chain inside a key-value store, thus the `field_name` must remain constant between calls.
- `encryption_key` is derived using HKDF-SHA256 from:
  - `consensus_state_ikm`
  - `field_name`
  - `contract_key`
- `ad` (Additional Data) is used to prevent leaking information about the same value written to the same key at different times.

## `contract_key`

- `contract_key` is a concatenation of two values: `signer_id || authenticated_contract_key`.
- Its purpose is to make sure that each contract have a unique unforgeable encryption key.
  - Unique: Make sure the state of two contracts with the same code is different.
  - Unforgeable: Make sure a malicious node runner won't be able to locally encrypt transactions with it's own encryption key and then decrypt the resulting state with the fake key.
- When a contract is deployed (i.e., on contract init), `contract_key` is generated inside the Enclave as follows:

```js
signer_id = sha256(concat(msg_sender, block_height));

authentication_key = hkdf({
  salt: hkdf_salt,
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
  salt: hkdf_salt,
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
  salt: hkdf_salt,
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
  // read previous_ad, verify it, calculate new ad
  previous_ad = current_state_ciphertext.slice(0, 32); // first 32 bytes/256 bits
  current_state_ciphertext = current_state_ciphertext.slice(32); // skip first 32 bytes

  aes_128_siv_decrypt({
    key: encryption_key,
    data: current_state_ciphertext,
    ad: previous_ad,
  }); // just to authenticate previous_ad
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
  salt: hkdf_salt,
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

## remove_db(field_name)

Very similar to `read_db`.

```js
encryption_key = hkdf({
  salt: hkdf_salt,
  ikm: concat(consensus_state_ikm, field_name, contract_key),
});

encrypted_field_name = aes_128_siv_encrypt({
  key: encryption_key,
  data: field_name,
});

internal_remove_db(encrypted_field_name);
```

# Transaction Encryption

TODO reasoning

- `tx_encryption_key`: An AES-128-SIV encryption key. Will be used to encrypt tx inputs and decrypt tx outputs.
  - `tx_encryption_ikm` is derived using [ECDH](https://en.wikipedia.org/wiki/Elliptic-curve_Diffie%E2%80%93Hellman) ([x25519](https://tools.ietf.org/html/rfc7748#section-6)) with `consensus_io_exchange_pubkey` and `tx_sender_wallet_privkey` (on the sender's side).
  - `tx_encryption_ikm` is derived using [ECDH](https://en.wikipedia.org/wiki/Elliptic-curve_Diffie%E2%80%93Hellman) ([x25519](https://tools.ietf.org/html/rfc7748#section-6)) with `consensus_io_exchange_privkey` and `tx_sender_wallet_pubkey` (inside the Enclave of every full node).
- `tx_encryption_key` is derived using HKDF-SHA256 with `tx_encryption_ikm` and a random number `nonce`. This is to prevent using the same key for the same tx sender multiple times.
- The input (`msg`) to the contract is always prepended with the sha256 hash of the contract's code.
  - This is meant to prevent replaying an encrypted input of a legitimate contract to a malicious contract and asking the malicious contract to decrypt the input. In this attack example the output will still be encrypted with a `tx_encryption_key` that only the original sender knows, but the malicious contract can be written to save the decrypted input to its state and then via a getter with no access control retrieve the encrypted input.

## Input

### On the transaction sender

```js
tx_encryption_ikm = ecdh({
  privkey: tx_sender_wallet_privkey,
  pubkey: consensus_io_exchange_pubkey, // from genesis.json
}); // 256 bits

nonce = true_random({ bytes: 32 });

tx_encryption_key = hkdf({
  salt: hkdf_salt,
  ikm: concat(tx_encryption_ikm, nonce),
}); // 256 bits

ad = concat(nonce, tx_sender_wallet_pubkey);

codeHash = toHexString(sha256(contract_code));

encrypted_msg = aes_128_siv_encrypt({
  key: tx_encryption_key,
  data: concat(codeHash, msg),
  ad: ad,
});

tx_input = concat(ad, encrypted_msg);
```

### On the consensus layer, inside the Enclave of every full node

```js
nonce = tx_input.slice(0, 32); // 32 bytes
tx_sender_wallet_pubkey = tx_input.slice(32, 32); // 32 bytes, compressed curve25519 public key
encrypted_msg = tx_input.slice(64);

tx_encryption_ikm = ecdh({
  privkey: consensus_io_exchange_privkey,
  pubkey: tx_sender_wallet_pubkey,
}); // 256 bits

tx_encryption_key = hkdf({
  salt: hkdf_salt,
  ikm: concat(tx_encryption_ikm, nonce),
}); // 256 bits

codeHashAndMsg = aes_128_siv_decrypt({
  key: tx_encryption_key,
  data: encrypted_msg,
});

codeHash = codeHashAndMsg.slice(0, 64);
assert(codeHash == toHexString(sha256(contract_code)));

msg = codeHashAndMsg.slice(64);
```

## Output

- The output must be a valid JSON object, as it is passed to multiple mechanisms for final processing:
  - Logs are treated as Tendermint events
  - Messages can be callbacks to another contract call or contract init
  - Messages can also instruct sending funds from the contract's wallet
  - There's a data section which is free-form bytes to be interpreted by the client (or dApp)
  - And there's also an error section
- Therefore the output must be partially encrypted.
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
          "wasm": {
            "execute": {
              "msg": "{\"banana\":1,\"papaya\":2}", // need to encrypt this value
              "contract_addr": "aaa",
              "callback_code_hash": "bbb",
              "send": { "amount": 100, "denom": "uscrt" }
            }
          }
        },
        {
          "wasm": {
            "instantiate": {
              "msg": "{\"water\":1,\"fire\":2}", // need to encrypt this value
              "code_id": "123",
              "callback_code_hash": "ccc",
              "send": { "amount": 0, "denom": "uscrt" }
            }
          }
        }
      ],
      "log": [
        {
          "key": "action", // need to encrypt this value
          "value": "transfer" // need to encrypt this value
        },
        {
          "key": "sender", // need to encrypt this value
          "value": "secret1v9tna8rkemndl7cd4ahru9t7ewa7kdq87c02m2" // need to encrypt this value
        },
        {
          "key": "recipient", // need to encrypt this value
          "value": "secret1f395p0gg67mmfd5zcqvpnp9cxnu0hg6rjep44t" // need to encrypt this value
        }
      ],
      "data": "bla bla" // need to encrypt this value
    }
  }
  ```

- Notice on a `Contract` message, the `msg` value should be the same `msg` as in our `tx_input`, so we need to prepend the `nonce` and `tx_sender_wallet_pubkey` just like we did on the tx sender above.
- On a `Contract` message, we also send a `callback_signature`, so we can later on verify the parameters sent to the enclave:
  ```
  callback_signature = sha256(consensus_callback_secret | calling_contract_addr | encrypted_msg | funds_to_send)
  ```
  For more on that, [read here](../dev/privacy-model-of-secret-contracts.md#verified-values-during-contract-execution).
- For the rest of the encrypted outputs we only need to send the ciphertext, as the tx sender can get `consensus_io_exchange_pubkey` from `genesis.json` and `nonce` from the `tx_input` that is attached to the `tx_output`.
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
  // init or execute
  // external query is the same, but happens mid-run and not as an output
  for (m in output["ok"]["messages"]) {
    if (m["type"] == "Instantiate" || m["type"] == "Execute") {
      encrypted_msg = aes_128_siv_encrypt({
        key: tx_encryption_key,
        data: concat(m["callback_code_hash"], m["msg"]),
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

TODO add more

## Deanonymizing with ciphertext byte count

No encryption padding, so a value of e.g. "yes" or "no" can be deanonymized by its byte count.

## Two contracts with the same `contract_key` could deanonymize each other's states

If an attacker can create a contract with the same `contract_key` as another contract, the state of the original contract can potentially be deanonymized.

For example, an original contract with a permissioned getter, such that only whitelisted addresses can query the getter. In the malicious contract the attacker can set themselves as the owner and ask the malicious contract to decrypt the state of the original contract via that permissioned getter.

## Tx Replay attacks

After a contract runs on the chain, an attacker can sync up a node up to a specific block in the chain, and then call into the enclave with the same authenticated user inputs that were given to the enclave on-chain, but out-of-order, or omit selected messages. A contract that does not anticipate or protect against this might end up de-anonymizing the information provided by users. For example, in a naive voting contract (or other personal data collection algorithm), we can de-anonymize a voter by re-running the vote without the target's request, and analyze the difference in final results.

## More Advanced Tx Replay attacks -- search to decision for Millionaire's problem

This attack provides a specific example of a TX replay attack extracting the full information of a client based on replaying a TX.

Specifically, assume for millionaire's that you have a contract where one person inputs their amount of money, then the other person does, then the contract sends them both a single bit saying who has more -- this is the simplest implementation for Millionaire's problem-solving. As person 2, binary search the interval of possible money amounts person 1 could have -- say you know person 1 has less than N dollars. First, query with N/2 as your value with your node detached from the wider network, get the single bit out (whether the true value is higher or lower), then repeat by re-syncing your node and calling in.
By properties of binary search, in log(n) tries (where n is the size of the interval) you'll have the exact value of person 1's input.

The naive solution to this is requiring the node to successfully broadcast the data of person 1 and person 2 to the network before revealing an answer (which is an implicit heartbeat test, that also ensures the transaction isn't replay-able), but even that's imperfect since you can reload the contract and replay the network state up to that broadcast, restoring the original state of the contract, then perform the attack with repeated rollbacks.

Assaf: You could maybe implement the contract with the help of a 3rd party. I.e. the 2 players send their amounts. When the 3rd party sends an approval tx only then the 2 players can query the result. However, this is not good UX.

## Partial storage rollback during contract runtime

Our current schema can verify that when reading from a field in storage, the value received from the host has been written by the same contract instance to the same field in storage. BUT we can not (yet) verify that the value is the most recent value that was stored there. This means that a malicious host can (offline) run a transaction, and then selectively provide outdated values for some fields of the storage. In the worst case, this can cause a contract to expose old secrets with new permissions, or new secrets with old permissions. The contract can protect against this by either (e.g.) making sure that pieces of information that have to be synced with each other are saved under the same field (so they are never observed as desynchronized) or (e.g.) somehow verify their validity when reading them from two separate fields of storage.

## Tx outputs can leak data

E.g. a dev writes a contract with 2 functions, the first one always outputs 3 events and the second one always outputs 4 events. By counting the number of output events an attacker can know which function was invoked. Also applies with deposits, callbacks and transfers.
