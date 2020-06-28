all:
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os ./target/wasm32-unknown-unknown/release/test_contract.wasm -o ./contract.wasm
