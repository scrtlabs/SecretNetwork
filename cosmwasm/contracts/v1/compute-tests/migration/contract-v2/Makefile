all: src/contract.rs src Cargo.toml Cargo.lock
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	rm -f ./migrate_contract_v2.wasm
	mv ./target/wasm32-unknown-unknown/release/migrate_contract_v2.wasm ./migrate_contract_v2.wasm

clean:
	cargo clean
	-rm -f ./*.wasm
