# Implementing the Secret in Contracts

:warning: This is a very advanced WIP.

- [Implementing the Secret in Contracts](#implementing-the-secret-in-contracts)
- [Bootstrap Process](#bootstrap-process)
  - [`consensus_seed`](#consensusseed)
  - [Key Derivation](#key-derivation)
    - [`consensus_seed_exchange_privkey`](#consensusseedexchangeprivkey)
    - [`consensus_io_exchange_privkey`](#consensusioexchangeprivkey)
    - [`consensus_state_ikm`](#consensusstateikm)
  - [Bootstrap Process Epilogue](#bootstrap-process-epilogue)
- [Node Startup](#node-startup)
- [New Node Registration](#new-node-registration)
  - [On the new node](#on-the-new-node)
  - [On the consensus layer, inside the Enclave of every full node](#on-the-consensus-layer-inside-the-enclave-of-every-full-node)
    - [`seed_exchange_key`](#seedexchangekey)
    - [Sharing `consensus_seed` with the new node](#sharing-consensusseed-with-the-new-node)
  - [Back on the new node, inside its Enclave](#back-on-the-new-node-inside-its-enclave)
    - [`seed_exchange_key`](#seedexchangekey-1)
    - [Decrypting `encrypted_consensus_seed`](#decrypting-encryptedconsensusseed)
  - [New Node Registration Epilogue](#new-node-registration-epilogue)
- [Contracts State Encryption](#contracts-state-encryption)
- [Transaction Encryption](#transaction-encryption)
  - [Input](#input)
  - [Output](#output)
- [Chain Upgrades](#chain-upgrades)
- [Theoretical Attacks](#theoretical-attacks)

# Bootstrap Process

Before the genesis of a new chain, there most be a bootstrap node to generate network-wide secrets to fuel all the privacy features of the chain.

## `consensus_seed`

- Create a remote attestation proof that the nodw's Enclave is genuine.
- Generate inside the Enclave a true random 256 bits seed: `consensus_seed`.
- Seal `consensus_seed` with MRENCLAVE to a local file: `$HOME/.enigmad/sgx-secrets/consensus_seed.sealed`.

```js
// array of 32 bytes
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
- The HKDF [salt](https://tools.ietf.org/html/rfc5869#section-3.1) is chosen to be Bitcoin's halving block hash.

```js
hkfd_salt = sha256(
  0x000000000000000000024bead8df69990852c202db0e0097c1a12ea637d7e96d
);
```

- Using HKDF, `hkfd_salt` and `consensus_seed`, derive the following keys:

### `consensus_seed_exchange_privkey`

- `consensus_seed_exchange_privkey`: A secp256k1 curve private key. Will be used to derive encryption keys in order to securely share `consensus_seed` with new nodes in the network.
- From `consensus_seed_exchange_privkey` calculate `consensus_seed_exchange_pubkey`.

```js
consensus_seed_exchange_privkey = hkdf({
  salt: hkfd_salt,
  ikm: consensus_seed.append(uint8(1)),
}); // array of 32 bytes

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
}); // array of 32 bytes

consensus_io_exchange_pubkey = calculate_secp256k1_pubkey(
  consensus_io_exchange_privkey
);
```

### `consensus_state_ikm`

- `consensus_state_ikm`: An input keying material (IKM) for HKDF to derive encryption keys for contracts' state.

```js
consensus_state_ikm = hkdf({
  salt: hkfd_salt,
  ikm: consensus_seed.append(uint8(3)),
}); // array of 32 bytes
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
  - 256 bits true random `challenge`
  - 256 bits true random `nonce`

## On the consensus layer, inside the Enclave of every full node

TODO reasoning

- Receive the `enigmacli tx register auth` transaction.
- Verify the remote attestation proof that the new node's Enclave is genuine.

### `seed_exchange_key`

TODO reasoning

- `seed_exchange_key`: An AES-256-GCM encryption key. Will be used to send `consensus_seed` to the new node.
- `seed_exchange_key` is derived the following way:
  - `seed_exchange_ikm` is derived using [ECDH](https://en.wikipedia.org/wiki/Elliptic-curve_Diffie%E2%80%93Hellman) with `consensus_seed_exchange_privkey` and `new_node_seed_exchange_pubkey`.
  - `seed_exchange_key` is derived using HKDF from `seed_exchange_ikm` and `challenge`.

```js
seed_exchange_ikm = ecdh({
  privkey: consensus_seed_exchange_privkey,
  pubkey: new_node_seed_exchange_pubkey,
}); // array of 32 bytes

seed_exchange_key = hkdf({
  salt: hkfd_salt,
  ikm: seed_exchange_ikm.append(uint8(challenge)),
}); // array of 32 bytes
```

### Sharing `consensus_seed` with the new node

TODO reasoning

- The output of the `enigmacli tx register auth` transaction is `consensus_seed` encrypted with AES-256-GCM, `seed_exchange_key` as the encryption key and `nonce` as the encryption IV.

```js
encrypted_consensus_seed = aes_256_gcm_encrypt({
  iv: nonce,
  key: seed_exchange_key,
  data: consensus_seed,
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
  - `seed_exchange_key` is derived using HKDF with `seed_exchange_ikm` and `challenge`.

```js
seed_exchange_ikm = ecdh({
  privkey: new_node_seed_exchange_privkey,
  pubkey: consensus_seed_exchange_pubkey,
}); // array of 32 bytes

seed_exchange_key = hkdf({
  salt: hkfd_salt,
  ikm: seed_exchange_ikm.append(uint8(challenge)),
}); // array of 32 bytes
```

### Decrypting `encrypted_consensus_seed`

TODO reasoning

- `encrypted_consensus_seed` is encrypted with AES-256-GCM, `seed_exchange_key` as the encryption key and `nonce` as the encryption IV.
- The new node now has all of these^ parameters inside its Enclave, so it's able to decrypt `consensus_seed` from `encrypted_consensus_seed`.
- Seal `consensus_seed` to disk at `"$HOME/.enigmad/sgx-secrets/consensus_seed.sealed"`.

```js
consensus_seed = aes_256_gcm_decrypt({
  iv: nonce,
  key: seed_exchange_key,
  data: encrypted_consensus_seed,
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

# Transaction Encryption

## Input

TODO reasoning

## Output

TODO reasoning

# Chain Upgrades

TODO reasoning

# Theoretical Attacks
