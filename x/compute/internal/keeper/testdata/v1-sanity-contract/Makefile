all:
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	cp ./target/wasm32-unknown-unknown/release/v1_sanity_contract.wasm ./v1-sanity-contract.wasm

clean:
	cargo clean
	-rm -f ./v1-sanity-contract.wasm