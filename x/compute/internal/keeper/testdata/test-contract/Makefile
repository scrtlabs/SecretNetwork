all:
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os ./target/wasm32-unknown-unknown/release/test_contract.wasm -o ./contract.wasm

	# Compile with floats
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked --features with_floats
	wasm-opt -Os ./target/wasm32-unknown-unknown/release/test_contract.wasm -o ./contract_with_floats.wasm

	# Create a wasm with more than 192 memory pages (fails in init, handle & query, this is our limit)
	# https://github.com/enigmampc/SecretNetwork/blob/9eef8591b2c04c586ceee12f424b92062598123e/cosmwasm/packages/wasmi-runtime/src/wasm/memory.rs#L39
	wasm2wat ./contract.wasm | perl -pe 's/\(memory \(;0;\) 17\)/(memory (;0;) 193)/' > /tmp/too-high-initial-memory.wat
	wat2wasm /tmp/too-high-initial-memory.wat -o ./too-high-initial-memory.wasm

	# Create a wasm with more than 512 memory pages (fails in store, this is cosmwasm's limit)
	# https://github.com/enigmampc/SecretNetwork/blob/9eef8591b2c04c586ceee12f424b92062598123e/cosmwasm/packages/sgx-vm/src/compatability.rs#L36
	wasm2wat ./contract.wasm | perl -pe 's/\(memory \(;0;\) 17\)/(memory (;0;) 513)/' > /tmp/static-too-high-initial-memory.wat
	wat2wasm /tmp/static-too-high-initial-memory.wat -o ./static-too-high-initial-memory.wasm

clean:
	cargo clean
	-rm -f ./contract.wasm