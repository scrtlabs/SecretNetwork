all: src/contract.rs src Cargo.toml Cargo.lock
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	rm -f ./v1_random_test.wasm
	mv -f ./target/wasm32-unknown-unknown/release/v1_random_test.wasm ./v1_random_test.wasm

clean:
	cargo clean
	-rm -f ./v1_random_test.wasm
