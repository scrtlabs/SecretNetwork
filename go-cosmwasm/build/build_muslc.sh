#!/bin/sh

cargo build --release --features backtraces --example muslc
cp /code/target/release/examples/libmuslc.a /code/api/libgo_cosmwasm_muslc.a
