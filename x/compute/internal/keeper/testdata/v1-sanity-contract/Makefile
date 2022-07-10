all: src/contract.rs src/lib.rs src/msg.rs src/state.rs Cargo.toml Cargo.lock
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	cp ./target/wasm32-unknown-unknown/release/v1_sanity_contract.wasm ./contract.wasm

clean:
	cargo clean
	-rm -f ./contract.wasm