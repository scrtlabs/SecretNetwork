first: src/contract.rs src src src Cargo.toml Cargo.lock
	mv src/contract2.rs src/contract2.md
	mv src/contract3.rs src/contract3.md
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	cp ./target/wasm32-unknown-unknown/release/v1_sanity_contract.wasm ./v1-contract.wasm
	mv src/contract2.md src/contract2.rs
	mv src/contract3.md src/contract3.rs

second: src/contract2.rs src src src Cargo.toml Cargo.lock
	mv src/lib.rs src/lib.md
	mv src/lib2.md src/lib.rs
	mv src/contract.rs src/contract.md
	mv src/contract3.rs src/contract3.md
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	cp ./target/wasm32-unknown-unknown/release/v1_sanity_contract.wasm ./v1-contract-2.wasm
	mv src/contract.md src/contract.rs
	mv src/contract3.md src/contract3.rs
	mv src/lib.rs src/lib2.md

third: src/contract3.rs src src src Cargo.toml Cargo.lock
	mv src/lib3.md src/lib.rs
	mv src/contract.rs src/contract.md
	mv src/contract2.rs src/contract2.md
	rustup target add wasm32-unknown-unknown
	RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown
	cp ./target/wasm32-unknown-unknown/release/v1_sanity_contract.wasm ./v1-contract-3.wasm
	mv src/contract.md src/contract.rs
	mv src/contract2.md src/contract2.rs
	mv src/lib.rs src/lib3.md
	mv src/lib.md src/lib.rs

all:
	mv src/contract.md src/contract.rs || true
	mv src/contract2.md src/contract2.rs || true
	mv src/contract3.md src/contract3.rs || true
	mv src/lib.md src/lib.rs || true
	mv src/lib2.rs src/lib2.md || true
	mv src/lib3.md src/lib3.md || true
	make first second third

clean:
	cargo clean
	-rm -f ./v1-contract.wasm
