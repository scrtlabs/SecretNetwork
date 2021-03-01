# Protobuf Files directory
## What's in this directory

Files in this directory are copied from other projects, and are used
to build the [Protobuf](https://developers.google.com/protocol-buffers)
parsers used in the enclave to parse Cosmos SDK messages.

The file in the `gogoproto` directory is taken from
[regen-network/protobuf @ v1.3.3-alpha.regen.1](https://github.com/regen-network/protobuf/blob/v1.3.3-alpha.regen.1/gogoproto/gogo.proto)
which is a fork of
[gogo/protobuf @ v1.3.2](https://github.com/gogo/protobuf/blob/v1.3.2/gogoproto/gogo.proto)
([PR](https://github.com/gogo/protobuf/pull/658)
| [cosmos-sdk go.mod](https://github.com/cosmos/cosmos-sdk/blob/v0.40.1/go.mod#L60))
and is used by the Cosmos SDK protobuf files.

The `cosmos` directory contains copies of protobuf files from
[cosmos/cosmos-sdk @ v0.40.1](https://github.com/cosmos/cosmos-sdk/tree/v0.40.1/proto/cosmos).

CosmWasm protobuf files are taken straight from `x/compute/internal/types` in this repo.

## How to build the rust parsers to Cosmos protobuf messages

1. Download a version of `protobuf` compatible with Protobuf 3 from
   [protocolbuffers/protobuf](https://github.com/protocolbuffers/protobuf/releases)
   and install the binary to `~/bin` put the `include` directory in `~/.local`.
   ( Make sure `~/bin` is in your `$PATH`, or just install the binary to a directory that
   _is_ in your `$PATH`)

2. Run `make build-protobuf` from the `wasmi-runtime` directory. This should recreate
   most of the files under `src/proto`.

If you add new files that you want to create parsers for, add them to
the `directories` list in `fn build_protobuf_parsers()` inside `build.rs`.
Then, update any `mod.rs` files that may be missing or incomplete.

## How to get syntax highlighting for the `*.proto` files in JetBrains IDEs

1. install the "Protocol Buffer Editor" plugin to your JetBrains IDE

2. Add these paths to the "Languages & Frameworks/Protocol Buffers" configuration
   in your JetBrains IDE so it can see gogoproto:
   * `~/.local/include`
   * `cosmwasm/packages/wasmi-runtime/proto`
