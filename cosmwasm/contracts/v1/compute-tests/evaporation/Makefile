all: src/contract.rs src src src Cargo.toml Cargo.lock
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	cp ./target/wasm32-unknown-unknown/release/evaporate_contract_example.wasm ./evaporate_contract_example.wasm
	cp ./target/wasm32-unknown-unknown/release/evaporate_contract_example.wasm ../../../../../x/compute/internal/keeper/testdata/evaporate.wasm

clean:
	cargo clean
	-rm -f ./v1-contract.wasm
