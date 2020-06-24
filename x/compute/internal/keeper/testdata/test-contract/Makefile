all:
	cargo wasm
	cp ./target/wasm32-unknown-unknown/release/test_contract.wasm ./contract.wasm

clean:
	cargo clean
	-rm -f contract.wasm
