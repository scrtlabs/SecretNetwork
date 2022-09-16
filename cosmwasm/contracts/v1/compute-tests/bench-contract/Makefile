all: src/contract.rs src src src Cargo.toml Cargo.lock
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	cp ./target/wasm32-unknown-unknown/release/bench_contract.wasm ./bench_contract.wasm

clean:
	cargo clean
	-rm -f ./v1-contract.wasm
