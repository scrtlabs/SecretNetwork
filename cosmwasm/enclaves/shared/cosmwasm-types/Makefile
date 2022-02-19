SGX_MODE ?= HW

ifeq ($(SGX_MODE), SW)
	Rust_Flags := "-Z force-unstable-if-unmarked -C target-feature=+aes,+ssse3"
else
	Rust_Flags := "-Z force-unstable-if-unmarked -C target-cpu=skylake"
endif

Rust_Target_Path = "$(CURDIR)/../../xargo"

.PHONY: check
check:
	RUST_TARGET_PATH=$(Rust_Target_Path) RUSTFLAGS=$(Rust_Flags) xargo check --features "$(FEATURES)" --target x86_64-unknown-linux-sgx

.PHONY: clean
clean:
	xargo clean
