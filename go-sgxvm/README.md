# go-sgxvm

This is a wrapper around the [Sputnik VM](https://github.com/rust-blockchain/evm) inside Intel SGX Enclave.
It allows you to execute EVM transactions from Go applications.

It was forked from https://github.com/CosmWasm/wasmvm 

## Build SGX-EVM & SGX-Wrapper

Ensure that SGX SDK was installed to `/opt/intel/` directory

Then run:
`source /opt/intel/sgxsdk/environment`

Now you are ready to build enclave with SGXVM and wrapper around it. To do it, run:
`make build`

## Structure

This repo contains both Rust and Go code. The rust code is compiled into a dll/so
to be linked via cgo and wrapped with a pleasant Go API. The full build step
involves compiling rust -> C library, and linking that library to the Go code.
For ergonomics of the user, we will include pre-compiled libraries to easily
link with, and Go developers should just be able to import this directly.

## Design

To understand how Cosmos SDK and EVM Keeper interacts with this library, you can refer to diagram below:

![plot](./spec/sgxsequence.png)

## Development

There are two halfs to this code - go and rust. The first step is to ensure that there is
a proper dll built for your platform. This should be `internal/api/libsgx_wrapper.X`, where X is:

- `so` for Linux systems
- `dylib` for MacOS
- `dll` for Windows - Not currently supported due to upstream dependency

## Toolchain

For development you should be able to use any reasonably up-to-date Rust stable.
