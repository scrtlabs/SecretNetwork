SGX_SDK ?= $(HOME)/.sgxsdk/sgxsdk
CUSTOM_EDL_PATH := ../../../third_party/vendor/sgx_edl/edl

SGX_COMMON_CFLAGS += -fstack-protector
App_Include_Paths := -I./ -I./include -I$(SGX_SDK)/include -I$(CUSTOM_EDL_PATH)
App_C_Flags := $(SGX_COMMON_CFLAGS) -fPIC -Wno-attributes $(App_Include_Paths)

Enclave_Path := ../execute
Enclave_EDL_Products := lib/Enclave_u.c lib/Enclave_u.h

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
build-deps: librust_cosmwasm_enclave.signed.so lib/libEnclave_u.a

librust_cosmwasm_enclave.signed.so: inner-build-enclave
	cp $(Enclave_Path)/$@ ./

.PHONY: inner-build-enclave
inner-build-enclave:
	FEATURES="test" $(MAKE) -C $(Enclave_Path) enclave

# This file will be picked up by the crate's build script and linked into the library.
lib/libEnclave_u.a: $(Enclave_EDL_Products)
	$(CC) $(App_C_Flags) -c lib/Enclave_u.c -o lib/Enclave_u.o
	$(AR) rcsD $@ lib/Enclave_u.o

# We make sure that the enclave is built before we compile the edl,
# because the EDL depends on a header file that is generated in that process.
$(Enclave_EDL_Products): $(Enclave_Path)/Enclave.edl enclave-ffi-types.h
	mkdir -p ./lib
	sgx_edger8r --untrusted $< --search-path $(SGX_SDK)/include --search-path $(CUSTOM_EDL_PATH) --untrusted-dir ./lib

# This file gets generated whenever we build this crate, because enclave-ffi-types is our dependency
# but when running the build for the first time, there's an interdependency between the .edl which requires this
# header, and the crate which needs the objects generated from the .edl file to correctly compile.
# So here we do the minimum required work to generate this file correctly, and copy it to the right location
enclave-ffi-types.h: ../target/headers/enclave-ffi-types.h
	cp $< $@

.PHONY: clean-all
clean-all: clean clean-enclave

.PHONY: clean
clean:
	rm -rf lib Enclave_u.h Enclave_u.o Enclave_u.c enclave-ffi-types.h
	rm -f librust_cosmwasm_enclave.signed.so
	cargo clean

.PHONY: clean-enclave
clean-enclave:
	$(MAKE) -C $(Enclave_Path) clean
