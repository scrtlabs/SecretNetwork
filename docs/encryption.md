# Implementing the Secret in Contracts

:warning: This is a very advanced WIP.

- [Implementing the Secret in Contracts](#implementing-the-secret-in-contracts)
  - [Bootstrap Process](#bootstrap-process)
    - [`consensus_seed`](#consensusseed)
    - [Key Derivation](#key-derivation)
      - [`consensus_seed_exchange_privkey`](#consensusseedexchangeprivkey)
      - [`consensus_io_exchange_privkey`](#consensusioexchangeprivkey)
      - [`consensus_base_state_ikm`](#consensusbasestateikm)
    - [Bootstrap process epilogue](#bootstrap-process-epilogue)
  - [Node Startup](#node-startup)
  - [New Node Registration](#new-node-registration)
  - [Contracts State Encryption](#contracts-state-encryption)
  - [Transaction Encryption](#transaction-encryption)

## Bootstrap Process

Before the genesis of a new chain.

### `consensus_seed`

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

### Key Derivation

- Key Derivation inside the Enclave is done deterministically using HKDF [[1]](https://tools.ietf.org/html/rfc5869#section-2)[[2]](https://en.wikipedia.org/wiki/HKDF).
- The HKDF [salt](https://tools.ietf.org/html/rfc5869#section-3.1) is chosen to be Bitcoin's halving block hash.

```js
hkfd_salt = uint256(
  0x000000000000000000024bead8df69990852c202db0e0097c1a12ea637d7e96d
);
```

- Using HKDF, `hkfd_salt` and `consensus_seed`, derive the following keys:

#### `consensus_seed_exchange_privkey`

- `consensus_seed_exchange_privkey`: A secp256k1 curve private key. Will be used to derive encryption keys in order to securely share `consensus_seed` with new nodes in the network.
- From `consensus_seed_exchange_privkey` calculate `consensus_seed_exchange_pubkey`.

```js
// 256 bits
consensus_seed_exchange_privkey = hkdf({
  salt: hkfd_salt,
  data: uint256(consensus_seed) + uint256(1),
});
consensus_seed_exchange_pubkey = calculate_secp256k1_pubkey(
  consensus_seed_exchange_privkey
);
```

#### `consensus_io_exchange_privkey`

- `consensus_io_exchange_privkey`: A secp256k1 curve private key. Will be used to derive encryption keys in order to decrypt transaction inputs and encrypt transaction outputs.
- From `consensus_io_exchange_privkey` calculate `consensus_io_exchange_pubkey`.

```js
// 256 bits
consensus_io_exchange_privkey = hkdf({
  salt: hkfd_salt,
  data: uint256(consensus_seed) + uint256(2),
});
consensus_io_exchange_pubkey = calculate_secp256k1_pubkey(
  consensus_io_exchange_privkey
);
```

#### `consensus_base_state_ikm`

- `consensus_base_state_ikm`: An input keying material (IKM) for HKDF to derive encryption keys for contracts' state.

```js
// 256 bits
consensus_base_state_ikm = hkdf({
  salt: hkfd_salt,
  data: uint256(consensus_seed) + uint256(3),
});
```

### Bootstrap process epilogue

- Seal to disk (`"$HOME/.enigmad/sgx-secrets/consensus_seed.sealed"`):
  - `consensus_seed`
- Publish to `genesis.json`:
  - `consensus_seed_exchange_pubkey`
  - `consensus_io_exchange_pubkey`

## Node Startup

When a full node resumes its participation in the network, it reads `consensus_seed` from `"$HOME/.enigmad/sgx-secrets/consensus_seed.sealed"` and again does [key derivation](#Key-Derivation) as outlined above.

## New Node Registration

A new node wants to join the network as a full node.

- Generate inside the node's Enclave a true random secp256k1 curve private key: `new_node_seed_exchange_privkey`.
- From `new_node_seed_exchange_privkey` calculate `new_node_seed_exchange_pubkey`.
- Create a remote attestation proof that the Enclave is genuine.
-

## Contracts State Encryption

## Transaction Encryption
