FROM rust:1.56-slim-bullseye

RUN rustup target add wasm32-unknown-unknown
RUN apt update && apt install -y binaryen clang && rm -rf /var/lib/apt/lists/*

WORKDIR /contract

ENTRYPOINT ["/bin/bash", "-c", "RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked && wasm-opt -Oz ./target/wasm32-unknown-unknown/release/*.wasm -o ./contract.wasm && cat ./contract.wasm | gzip -n -9 > ./contract.wasm.gz && rm -f ./contract.wasm"]
