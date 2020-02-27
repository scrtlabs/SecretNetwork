#!/bin/bash

cargo build --release
cp target/release/deps/libgo_cosmwasm.so api
# FIXME: re-enable stripped so when we approach a production release, symbols are nice for debugging
# strip api/libgo_cosmwasm.so
