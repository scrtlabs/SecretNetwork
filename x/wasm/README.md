# Wasm Module

This should be a brief overview of the functionality

## Configuration

You can add the following section to `config/app.toml`. Below is shown with defaults:

```toml
[wasm]
# This is the maximum sdk gas (wasm and storage) that we allow for any x/wasm "smart" queries
query_gas_limit = 300000
# This is the number of wasm vm instances we keep cached in memory for speed-up
# Warning: this is currently unstable and may lead to crashes, best to keep for 0 unless testing locally
lru_size = 0
```

## Messages

TODO

## CLI

TODO

## Rest

TODO
