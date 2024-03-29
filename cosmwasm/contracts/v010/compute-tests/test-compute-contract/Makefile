Rust_Crate_Source := $(shell find -type f | grep -P '(\.rs|\.toml|\.lock)$$' | grep -vP '^\./target')

.PHONY: all
all: contract.wasm contract_with_floats.wasm too-high-initial-memory.wasm static-too-high-initial-memory.wasm

contract.wasm: $(Rust_Crate_Source)
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	rm -f ./contract.wasm
	mv -f ./target/wasm32-unknown-unknown/release/test_contract.wasm ./contract.wasm

# Compile with floats
contract_with_floats.wasm: $(Rust_Crate_Source)
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --features with_floats
	rm -f ./contract_with_floats.wasm
	mv -f ./target/wasm32-unknown-unknown/release/test_contract.wasm ./contract_with_floats.wasm

# Older versions of wabt had the --enable-sign-extension flag, while on newer 
# versions it's turned on by default and the flag is replaced with --disable-sign-extension
WASM2WAT_OPTS := $(shell wasm2wat --help | grep -Po '\-\-enable-sign-extension')
WAT2WAT_OPTS := $(shell wat2wasm --help | grep -Po '\-\-enable-sign-extension')

# Create a wasm with more than 192 memory pages (fails in init, handle & query, this is our limit)
# https://github.com/scrtlabs/SecretNetwork/blob/9eef8591b2c04c586ceee12f424b92062598123e/cosmwasm/packages/wasmi-runtime/src/wasm/memory.rs#L39
too-high-initial-memory.wasm: contract.wasm
	wasm2wat $(WASM2WAT_OPTS) ./contract.wasm | perl -pe 's/\(memory \(;0;\) \d+\)/(memory (;0;) 193)/' > /tmp/too-high-initial-memory.wat
	wat2wasm $(WAT2WAT_OPTS) /tmp/too-high-initial-memory.wat -o ./too-high-initial-memory.wasm

# Create a wasm with more than 512 memory pages (fails in store, this is cosmwasm's limit)
# https://github.com/scrtlabs/SecretNetwork/blob/9eef8591b2c04c586ceee12f424b92062598123e/cosmwasm/packages/sgx-vm/src/compatability.rs#L36
static-too-high-initial-memory.wasm: contract.wasm
	wasm2wat $(WASM2WAT_OPTS) ./contract.wasm | perl -pe 's/\(memory \(;0;\) \d+\)/(memory (;0;) 513)/' > /tmp/static-too-high-initial-memory.wat
	wat2wasm $(WAT2WAT_OPTS) /tmp/static-too-high-initial-memory.wat -o ./static-too-high-initial-memory.wasm

setup:
	apt-get install wabt

clean:
	cargo clean
	-rm -f ./*.wasm
