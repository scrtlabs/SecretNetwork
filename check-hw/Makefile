FEATURES ?=

all: 
	cargo build --release --features "$(FEATURES)"
	cp target/release/check-hw .

clean:
	cargo clean 
	rm -f check-hw check_hw_enclave.so

# Enforce HW because SW doesn't make sense
clippy:
	SGX_MODE=HW cargo clippy 