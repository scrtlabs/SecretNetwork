# Implementing the Secret in Contracts

:warning: This is a very advanced WIP.

## Bootstrap Process

- Before the genesis of a new chain, the bootstrap node generates inside its Enclave a true random 256 bits seed: `consensus_seed`.
- The bootstrap node then seals `consensus_seed` with MRENCLAVE to a local file: `$HOME/.enigmad/sgx-secrets/consensus_seed.sealed`.
- Then using HKDF [[1]](https://tools.ietf.org/html/rfc5869#section-2)[[2]](https://en.wikipedia.org/wiki/HKDF) with Bitcoin's halving block as [salt](https://tools.ietf.org/html/rfc5869#section-3.1), we're going to derive the following values:
  - `consensus_seed_exchange_privkey`: A secp256k1 curve private key, will be used to derive encryption keys in order to securely share `consensus_seed` with new nodes in the network.
  - `consensus_io_exchange_privkey`: A secp256k1 curve private key, will be used to derive encryption keys in order to decrypt transaction inputs and encrypt transaction outputs.
  - `consensus_base_state_ikm`: An input keying material (IKM) for HKDF to derive encryption keys for contracts' state.
- From `consensus_seed_exchange_privkey` calculate `consensus_seed_exchange_pubkey`.
- From `consensus_io_exchange_privkey` calculate `consensus_io_exchange_pubkey`.
- Publish `consensus_seed_exchange_pubkey` and `consensus_io_exchange_pubkey` to `genesis.json`.

### consensus_seed

```js
// 256 bits
consensus_seed = true_random({ bytes: 32 });
seal({
  key: "MRENCLAVE",
  data: consensus_seed,
  to_file: "$HOME/.enigmad/sgx-secrets/consensus_seed.sealed",
});
```

### consensus_seed_exchange_privkey

```js
// 256 bits
consensus_seed_exchange_privkey = hkdf({
  salt: "0x000000000000000000024bead8df69990852c202db0e0097c1a12ea637d7e96d",
  data: consensus_seed.append(1),
});
consensus_seed_exchange_pubkey = calculate_secp256k1_pubkey(
  consensus_seed_exchange_privkey
);
```

### consensus_io_exchange_privkey

```js
// 256 bits
consensus_io_exchange_privkey = hkdf({
  salt: "0x000000000000000000024bead8df69990852c202db0e0097c1a12ea637d7e96d",
  data: consensus_seed.append(2),
});
consensus_io_exchange_pubkey = calculate_secp256k1_pubkey(
  consensus_io_exchange_privkey
);
```

### consensus_base_state_ikm

```js
// 256 bits
consensus_base_state_ikm = hkdf({
  salt: "0x000000000000000000024bead8df69990852c202db0e0097c1a12ea637d7e96d",
  data: consensus_seed.append(3),
});
```

## New Node Registration

## Contracts State Encryption

## Transaction Encryption
