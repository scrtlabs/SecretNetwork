all: src/contract.rs src/lib.rs src/msg.rs src/state.rs Cargo.toml Cargo.lock
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	cp ./target/wasm32-unknown-unknown/release/ibc.wasm ./

clean:
	cargo clean
	-rm -f ./ibc.wasm