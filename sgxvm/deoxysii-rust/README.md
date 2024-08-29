# deoxysii-rust - Deoxys-II-256-128 for Rust

[![Build status][github-ci-tests-badge]][github-ci-tests-link]

[github-ci-tests-badge]: https://github.com/oasisprotocol/deoxysii-rust/workflows/ci-tests/badge.svg
[github-ci-tests-link]: https://github.com/oasisprotocol/deoxysii-rust/actions?query=workflow:ci-tests

This crate provides a Rust implementation of [Deoxys-II-256-128 v1.43][0].

The implementation uses Intel SIMD intrinsics (SSSE3 and AES-NI) for
speed and will therefore only run on relatively modern x86-64 processors.

The MSRV is `1.59.0`.

To build everything, run tests and benchmarks, simply run `make`.

If you have the `RUSTFLAGS` environment variable set, it will override Rust
flags set in the repository's `.cargo/config`, so make sure you also add
`-C target-feature=+aes,+ssse3` to your custom flags or the code will fail
to build.

[0]: https://sites.google.com/view/deoxyscipher
