SHELL := /bin/bash
COMPILER = rustc
TARGET = $(shell rustc --version --verbose 2> /dev/null | awk "/host:/ { print \$$2 }")
TARGET_DIR = target/release/
DEFAULT = help
CC=clang
CXX=clang++
SGX_MODE ?= HW
ENCLAVE_HOME ?= $(HOME)/.swisstronik-enclave
PRODUCTION_MODE ?= false
AS_MODE ?= false 
VERSION := v1.0.3
Wrapper_build_feature := ""

Wrapper_lib := libsgx_wrapper_$(VERSION).x86_64.so
ifeq ($(AS_MODE), true)
	Wrapper_lib := libsgx_attestation_wrapper_$(VERSION).x86_64.so
	Wrapper_build_feature += attestation_server
endif

build:
	@ENCLAVE_HOME=$(ENCLAVE_HOME) PRODUCTION_MODE=$(PRODUCTION_MODE) SGX_MODE=$(SGX_MODE) AS_MODE=$(AS_MODE) VERSION=$(VERSION) make build -C ../sgxvm/
	@protoc --rust_out src/protobuf_generated/ proto/node.proto
	@ENCLAVE_HOME=$(ENCLAVE_HOME) cargo build --features "$(Wrapper_build_feature)" --release
	@cp $(CURDIR)/target/release/libsgxvm_wrapper.so $(CURDIR)/internal/api/$(Wrapper_lib)
	@protoc --go_out=types --proto_path=../sgxvm/proto/ ../sgxvm/proto/ffi.proto
	@protoc --go_out=types --proto_path=proto/ proto/node.proto
	@cp types/github.com/SigmaGmbH/librustgo/types/* types/ && rm -rf types/github.com

build_d:
	@ENCLAVE_HOME=$(ENCLAVE_HOME) PRODUCTION_MODE=$(PRODUCTION_MODE) SGX_MODE=$(SGX_MODE) AS_MODE=$(AS_MODE) VERSION=$(VERSION) make build_d -C ../sgxvm/
	@protoc --rust_out src/protobuf_generated/ proto/node.proto
	@ENCLAVE_HOME=$(ENCLAVE_HOME) cargo build --features "$(Wrapper_build_feature)" --release
	@cp $(CURDIR)/target/release/libsgxvm_wrapper.so $(CURDIR)/internal/api/$(Wrapper_lib)
	@protoc --go_out=types --proto_path=../sgxvm/proto/ ../sgxvm/proto/ffi.proto
	@protoc --go_out=types --proto_path=proto/ proto/node.proto
	@cp types/github.com/SigmaGmbH/librustgo/types/* types/ && rm -rf types/github.com

build_AS: build
	go build -tags osusergo,netgo,attestationServer -o ../build/attestationServer ./cmd/attestation

build_AS_d: build_d
	go build -tags osusergo,netgo,attestationServer -o ../build/attestationServer ./cmd/attestation