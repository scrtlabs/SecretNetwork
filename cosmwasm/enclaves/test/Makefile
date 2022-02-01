SGX_SDK ?= $(HOME)/.sgxsdk/sgxsdk
CUSTOM_EDL_PATH := ../../../third_party/vendor/sgx_edl/edl

SGX_COMMON_CFLAGS += -fstack-protector
App_Include_Paths := -I./ -I./include -I$(SGX_SDK)/include -I$(CUSTOM_EDL_PATH)
App_C_Flags := $(SGX_COMMON_CFLAGS) -fPIC -Wno-attributes $(App_Include_Paths)

Enclave_Path := ../wasmi-runtime

.PHONY: check
check:
	FEATURES="test" $(MAKE) -C $(Enclave_Path) check
	cargo check

.PHONY: clippy
clippy:
	FEATURES="test" $(MAKE) -C $(Enclave_Path) clippy
	cargo clippy

.PHONY: run
run: build-deps
	SGX_MODE=$(SGX_MODE) cargo run

.PHONY: build-deps
build-deps: lib/libEnclave_u.a build-enclave

# This file will be picked up by the crates build script and linked into the library.
# We make sure that the enclave is built before we compile the edl,
# because the EDL depends on a header file that is generated in that process.
lib/libEnclave_u.a: $(Enclave_Path)/Enclave.edl target/headers/enclave-ffi-types.h build-enclave
	sgx_edger8r --untrusted $< --search-path $(SGX_SDK)/include --search-path $(CUSTOM_EDL_PATH) --untrusted-dir ./
	$(CC) $(App_C_Flags) -c Enclave_u.c -o Enclave_u.o
	mkdir -p lib
	$(AR) rcsD $@ Enclave_u.o

target/headers/enclave-ffi-types.h: build-enclave
	mkdir -p $(dir $@)
	cp $(Enclave_Path)/$(@) $@

.PHONY: build-enclave
build-enclave:
	FEATURES="test" $(MAKE) -C $(Enclave_Path) enclave
	cp $(Enclave_Path)/librust_cosmwasm_enclave.signed.so ./

.PHONY: clean-all
clean-all: clean clean-enclave

.PHONY: clean
clean:
	rm -rf lib Enclave_u.h Enclave_u.o Enclave_u.c
	rm -f librust_cosmwasm_enclave.signed.so
	cargo clean

.PHONY: clean-enclave
clean-enclave:
	$(MAKE) -C $(Enclave_Path) clean
