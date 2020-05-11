.PHONY: all build build-rust build-go test docker-image docker-build docker-image-centos7

TOP_DIR := ../third_party/build
include $(TOP_DIR)/buildenv.mk

DOCKER_TAG := 0.6.3
USER_ID := $(shell id -u)
USER_GROUP = $(shell id -g)

DLL_EXT = ""
ifeq ($(OS),Windows_NT)
	DLL_EXT = dll
else
	UNAME_S := $(shell uname -s)
	ifeq ($(UNAME_S),Linux)
		DLL_EXT = so
	endif
	ifeq ($(UNAME_S),Darwin)
		DLL_EXT = dylib
	endif
endif

SGX_SDK ?= $(HOME)/.sgxsdk/sgxsdk

ifeq ($(SGX_ARCH), x86)
	SGX_COMMON_CFLAGS := -m32
else
	SGX_COMMON_CFLAGS := -m64
endif

ifeq ($(SGX_DEBUG), 1)
	SGX_COMMON_CFLAGS += -O0 -g
else
	SGX_COMMON_CFLAGS += -O2
endif

SGX_COMMON_CFLAGS += -fstack-protector

CUSTOM_EDL_PATH := ../third_party/vendor/sgx_edl/edl
App_Rust_Flags := --release
App_SRC_Files := $(shell find ../cosmwasm/lib/sgx-vm/ -type f -name '*.rs') \
    $(shell find ../cosmwasm/lib/sgx-vm/ -type f -name 'Cargo.toml') \
    $(shell find ./ -type f -name '*.rs') \
    $(shell find ./ -type f -name 'Cargo.toml')
App_Include_Paths := -I./ -I./include -I$(SGX_SDK)/include -I$(CUSTOM_EDL_PATH)
App_C_Flags := $(SGX_COMMON_CFLAGS) -fPIC -Wno-attributes $(App_Include_Paths)

Enclave_Path := ../cosmwasm/lib/wasmi-runtime
Enclave_EDL_Products := Enclave_u.c Enclave_u.h

all: build test

build: build-rust build-go

# don't strip for now, for better error reporting
# build-rust: build-rust-release strip
build-rust: build-rust-release

# use debug build for quick testing
build-rust-debug: librust_cosmwasm_enclave.signed.so lib/libEnclave_u.a
	cargo build --features backtraces
	cp target/debug/libgo_cosmwasm.$(DLL_EXT) api

# use release build to actually ship - smaller and much faster
build-rust-release: librust_cosmwasm_enclave.signed.so lib/libEnclave_u.a
	cargo build --release --features backtraces
	cp target/release/libgo_cosmwasm.$(DLL_EXT) api
	@ #this pulls out ELF symbols, 80% size reduction!

librust_cosmwasm_enclave.signed.so: build-enclave
	cp ../cosmwasm/lib/wasmi-runtime/librust_cosmwasm_enclave.signed.so ./

# This file will be picked up by the crates build script and linked into the library.
# We make sure that the enclave is built before we compile the edl,
# because the EDL depends on a header file that is generated in that process.
lib/libEnclave_u.a: $(Enclave_Path)/Enclave.edl target/headers/enclave-ffi-types.h build-enclave
	sgx_edger8r --untrusted $< --search-path $(SGX_SDK)/include --search-path $(CUSTOM_EDL_PATH) --untrusted-dir ./
	$(CC) $(App_C_Flags) -c Enclave_u.c -o Enclave_u.o
	rm Enclave_u.c
	mkdir -p lib
	$(AR) rcsD $@ Enclave_u.o

# This file gets generated whenever we build this crate, because enclave-ffi-types is our dependency
# but when running the build for the first time, there's an interdependency between the .edl which requires this
# header, and the crate which needs the objects generated from the .edl file to correctly compile.
# So here we do the minimum required work to generate this file correctly, and copy it to the right location
target/headers/enclave-ffi-types.h: build-enclave
	mkdir -p $(dir $@)
	cp ../cosmwasm/lib/wasmi-runtime/$(@) $@

.PHONY: build-enclave
build-enclave:
	$(MAKE) -C $(Enclave_Path) enclave

# implement stripping based on os
ifeq ($(DLL_EXT),so)
strip:
	strip api/libgo_cosmwasm.so
else
# TODO: add for windows and osx
strip:
endif

build-go:
	go build ./...

test:
	RUST_BACKTRACE=1 go test -v ./api .

docker-image-centos7:
	docker build . -t go-cosmwasm:$(DOCKER_TAG)-centos7 -f ./Dockerfile.centos7

docker-image-cross:
	docker build . -t go-cosmwasm:$(DOCKER_TAG)-cross -f ./Dockerfile.cross

release: docker-image-cross docker-image-centos7
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code go-cosmwasm:$(DOCKER_TAG)-cross
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code go-cosmwasm:$(DOCKER_TAG)-centos7

.PHONY: clean
clean:
	rm -rf lib $(Enclave_EDL_Products) *.o *.so
	cargo clean

.PHONY: clean-all
clean-all: clean
	$(MAKE) -C ../cosmwasm/lib/wasmi-runtime clean
