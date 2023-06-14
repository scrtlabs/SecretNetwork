all: src/contract.rs src Cargo.toml Cargo.lock
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	cp ./target/wasm32-unknown-unknown/release/migrate_contract_v1.wasm ./migrate_contract_v1.wasm
	cp ./target/wasm32-unknown-unknown/release/migrate_contract_v1.wasm ../../../../../../x/compute/internal/keeper/testdata/migrate_contract_v1.wasm

clean:
	cargo clean
	-rm -f ./*.wasm
