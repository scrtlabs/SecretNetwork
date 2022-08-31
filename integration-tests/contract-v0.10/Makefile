all: src/contract.rs src/lib.rs Cargo.toml Cargo.lock
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	cp ./target/wasm32-unknown-unknown/release/contract_v010.wasm ./contract.wasm

clean:
	cargo clean
	-rm -f ./contract.wasm