all:
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os ./target/wasm32-unknown-unknown/release/test_contract.wasm -o ./contract.wasm
	wasm2wat ./contract.wasm | perl -pe 's/\(module/(module (import "env" "memory" (memory 1))/' | grep -vF '(memory (;0;) 17)' > /tmp/import-memory.wat
	wat2wasm /tmp/import-memory.wat -o ./import-memory.wasm

clean:
	cargo clean
	-rm -f ./contract.wasm