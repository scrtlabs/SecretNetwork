#!/bin/bash

# ref: https://wapl.es/rust/2019/02/17/rust-cross-compile-linux-to-macos.html
export PATH="/opt/osxcross/target/bin:$PATH"
export LIBZ_SYS_STATIC=1
export CC=o64-clang
export CXX=o64-clang++

cargo build --release --target x86_64-apple-darwin 
cp target/x86_64-apple-darwin/release/deps/libgo_cosmwasm.dylib api
