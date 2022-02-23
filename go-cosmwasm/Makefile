BUILD_PROFILE ?= release

TOP_DIR := ../third_party/build
include $(TOP_DIR)/buildenv.mk

FEATURES ?=
FEATURES_U += $(FEATURES)
FEATURES_U += backtraces
FEATURES_U := $(strip $(FEATURES_U))

DOCKER_TAG := 0.8.2
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
App_SRC_Files := $(shell find ../cosmwasm/packages/sgx-vm/ -type f -name '*.rs') \
    $(shell find ../cosmwasm/packages/sgx-vm/ -type f -name 'Cargo.toml') \
    $(shell find ./ -type f -name '*.rs') \
    $(shell find ./ -type f -name 'Cargo.toml')
App_Include_Paths := -I./ -I./include -I$(SGX_SDK)/include -I$(CUSTOM_EDL_PATH)
App_C_Flags := $(SGX_COMMON_CFLAGS) -fPIC -Wno-attributes $(App_Include_Paths)

Enclave_Path := ../cosmwasm/enclaves/execute
Enclave_EDL_Products := lib/enclave/Enclave_u.c lib/enclave/Enclave_u.h

Query_Enclave_Path := ../cosmwasm/enclaves/query
Query_Enclave_EDL_Products := lib/query-enclave/Enclave_u.c lib/query-enclave/Enclave_u.h

.PHONY: all
all: build test

.PHONY: build
build: build-rust build-go

.PHONY: build-rust
build-rust: build-enclave build-query-enclave
	cargo build -Z unstable-options --profile $(BUILD_PROFILE) --features "$(FEATURES_U)"
	cp target/$(BUILD_PROFILE)/libgo_cosmwasm.$(DLL_EXT) api
	@ #this pulls out ELF symbols, 80% size reduction!

.PHONY: build-enclave
build-enclave: librust_cosmwasm_enclave.signed.so lib/libEnclave_u.a

.PHONY: build-query-enclave
build-query-enclave: librust_cosmwasm_query_enclave.signed.so lib/libQueryEnclave_u.a

librust_cosmwasm_enclave.signed.so: inner-build-enclave
	cp $(Enclave_Path)/$@ ./

librust_cosmwasm_query_enclave.signed.so: inner-build-query-enclave
	cp $(Query_Enclave_Path)/$@ ./

.PHONY: inner-build-enclave
inner-build-enclave:
	FEATURES="$(FEATURES)" $(MAKE) -C $(Enclave_Path) enclave

.PHONY: inner-build-query-enclave
inner-build-query-enclave:
	FEATURES="$(FEATURES)" $(MAKE) -C $(Query_Enclave_Path) enclave

# This file will be picked up by the crate's build script and linked into the library.
lib/libEnclave_u.a: $(Enclave_EDL_Products)
	$(CC) $(App_C_Flags) -c lib/enclave/Enclave_u.c -o lib/enclave/Enclave_u.o
	$(AR) rcsD $@ lib/enclave/Enclave_u.o

# This file will be picked up by the crate's build script and linked into the library.
lib/libQueryEnclave_u.a: $(Query_Enclave_EDL_Products)
	$(CC) $(App_C_Flags) -c lib/query-enclave/Enclave_u.c -o lib/query-enclave/Enclave_u.o
	$(AR) rcsD $@ lib/query-enclave/Enclave_u.o

# We make sure that the enclave is built before we compile the edl,
# because the EDL depends on a header file that is generated in that process.
$(Enclave_EDL_Products): $(Enclave_Path)/Enclave.edl enclave-ffi-types.h
	mkdir -p "lib/enclave"
	sgx_edger8r --untrusted $< --search-path $(SGX_SDK)/include --search-path $(CUSTOM_EDL_PATH) --untrusted-dir ./lib/enclave

# We make sure that the enclave is built before we compile the edl,
# because the EDL depends on a header file that is generated in that process.
$(Query_Enclave_EDL_Products): $(Query_Enclave_Path)/Enclave.edl enclave-ffi-types.h
	mkdir -p "lib/query-enclave"
	sgx_edger8r --untrusted $< --search-path $(SGX_SDK)/include --search-path $(CUSTOM_EDL_PATH) --untrusted-dir ./lib/query-enclave

# This file gets generated whenever we build this crate, because enclave-ffi-types is our dependency
# but when running the build for the first time, there's an interdependency between the .edl which requires this
# header, and the crate which needs the objects generated from the .edl file to correctly compile.
# So here we do the minimum required work to generate this file correctly, and copy it to the right location
enclave-ffi-types.h: ../cosmwasm/enclaves/target/headers/enclave-ffi-types.h
	cp $< $@

# implement stripping based on os
.PHONY: strip
ifeq ($(DLL_EXT),so)
strip:
	strip api/libgo_cosmwasm.so
else
# TODO: add for windows and osx
strip:
endif

.PHONY: build-go
build-go:
	go build ./...

.PHONY: test
test:
	RUST_BACKTRACE=1 go test -v ./api ./types .

.PHONY: test-safety
test-safety:
	GODEBUG=cgocheck=2 go test -race -v -count 1 ./api

# we should build all the docker images locally ONCE and publish them
.PHONY: docker-image-centos7
docker-image-centos7:
	docker build . -t cosmwasm/go-ext-builder:$(DOCKER_TAG)-centos7 -f ./Dockerfile.centos7

.PHONY: docker-image-cross
docker-image-cross:
	docker build . -t cosmwasm/go-ext-builder:$(DOCKER_TAG)-cross -f ./Dockerfile.cross

.PHONY: docker-image-alpine
docker-image-alpine:
	docker build . -t cosmwasm/go-ext-builder:$(DOCKER_TAG)-alpine -f ./Dockerfile.alpine

.PHONY: docker-images
docker-images: docker-image-centos7 docker-image-cross docker-image-alpine

.PHONY: docker-publish
docker-publish: docker-images
	docker push cosmwasm/go-ext-builder:$(DOCKER_TAG)-cross
	docker push cosmwasm/go-ext-builder:$(DOCKER_TAG)-centos7
	docker push cosmwasm/go-ext-builder:$(DOCKER_TAG)-alpine

# and use them to compile release builds
.PHONY: release
release:
	rm -rf target/release
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code cosmwasm/go-ext-builder:$(DOCKER_TAG)-cross
	rm -rf target/release
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code cosmwasm/go-ext-builder:$(DOCKER_TAG)-centos7

.PHONY: test-alpine
test-alpine:
	# build the muslc *.a file
	rm -rf target/release/examples
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code cosmwasm/go-ext-builder:$(DOCKER_TAG)-alpine
	# try running go tests using this lib with muslc
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code -w /code cosmwasm/go-ext-builder:$(DOCKER_TAG)-alpine go build -tags muslc .
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code -w /code cosmwasm/go-ext-builder:$(DOCKER_TAG)-alpine go test -tags muslc ./api ./types
	# build a go binary
	docker run --rm -u $(USER_ID):$(USER_GROUP) -v $(shell pwd):/code -w /code cosmwasm/go-ext-builder:$(DOCKER_TAG)-alpine go build -tags muslc -o muslc.exe ./cmd
	# run static binary locally (not dlls)
	./muslc.exe ./api/testdata/hackatom.wasm

.PHONY: clean
clean:
	rm -rf lib $(Enclave_EDL_Products) $(Query_Enclave_EDL_Products) *.o *.so *.h
	cargo clean

.PHONY: clean-all
clean-all: clean
	$(MAKE) -C $(Enclave_Path) clean
	$(MAKE) -C $(Query_Enclave_Path) clean
