FROM rust:1.69.0-slim-bullseye

RUN rustup target add wasm32-unknown-unknown
RUN apt update && apt install -y binaryen clang && rm -rf /var/lib/apt/lists/*

WORKDIR /contract

ENTRYPOINT ["/bin/bash", "-c", "\
    RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked && \
    (mkdir -p ./optimized-wasm/ && rm -f ./optimized-wasm/* && cp ./target/wasm32-unknown-unknown/release/*.wasm ./optimized-wasm/) && \
    for w in ./optimized-wasm/*.wasm; do \
        wasm-opt -Oz $w -o $w ; \
    done && \
    (cd ./optimized-wasm && gzip -n -9 -f * && rm *.wasm) \
"]
