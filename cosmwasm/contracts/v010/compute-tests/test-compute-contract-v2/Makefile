Rust_Crate_Source := $(shell find -type f | grep -P '(\.rs|\.toml|\.lock)$$' | grep -vP '^\./target')

.PHONY: all
all: contract.wasm

contract.wasm: $(Rust_Crate_Source)
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	rm -f ./contract-v2.wasm
	mv -f ./target/wasm32-unknown-unknown/release/test_contract_v2.wasm ./contract-v2.wasm

setup:
	apt-get install wabt

clean:
	cargo clean
	-rm -f ./*.wasm
