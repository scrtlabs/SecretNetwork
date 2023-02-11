Rust_Crate_Source := $(shell find -type f | grep -P '(\.rs|\.toml|\.lock)$$' | grep -vP '^\./target')

.PHONY: all
all: contract.wasm contract_with_floats.wasm too-high-initial-memory.wasm static-too-high-initial-memory.wasm

contract.wasm: $(Rust_Crate_Source)
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	cp ./target/wasm32-unknown-unknown/release/test_contract.wasm ./contract.wasm

# Compile with floats
contract_with_floats.wasm: $(Rust_Crate_Source)
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --features with_floats
	cp ./target/wasm32-unknown-unknown/release/test_contract.wasm ./contract_with_floats.wasm

# Create a wasm with more than 192 memory pages (fails in init, handle & query, this is our limit)
# https://github.com/scrtlabs/SecretNetwork/blob/9eef8591b2c04c586ceee12f424b92062598123e/cosmwasm/packages/wasmi-runtime/src/wasm/memory.rs#L39
too-high-initial-memory.wasm: contract.wasm
	wasm2wat ./contract.wasm | perl -pe 's/\(memory \(;0;\) \d+\)/(memory (;0;) 193)/' > /tmp/too-high-initial-memory.wat
	wat2wasm /tmp/too-high-initial-memory.wat -o ./too-high-initial-memory.wasm

# Create a wasm with more than 512 memory pages (fails in store, this is cosmwasm's limit)
# https://github.com/scrtlabs/SecretNetwork/blob/9eef8591b2c04c586ceee12f424b92062598123e/cosmwasm/packages/sgx-vm/src/compatability.rs#L36
static-too-high-initial-memory.wasm: contract.wasm
	wasm2wat ./contract.wasm | perl -pe 's/\(memory \(;0;\) \d+\)/(memory (;0;) 513)/' > /tmp/static-too-high-initial-memory.wat
	wat2wasm /tmp/static-too-high-initial-memory.wat -o ./static-too-high-initial-memory.wasm

setup:
	apt-get install wabt

clean:
	cargo clean
	-rm -f ./contract.wasm
