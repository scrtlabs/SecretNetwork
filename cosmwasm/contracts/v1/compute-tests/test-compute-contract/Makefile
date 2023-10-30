all: src/contract.rs src src src Cargo.toml Cargo.lock
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	rm -f ./v1-contract.wasm
	mv -f ./target/wasm32-unknown-unknown/release/v1_sanity_contract.wasm ./v1-contract.wasm

clean:
	cargo clean
	-rm -f ./v1-contract.wasm
