######## SGX SDK Settings ########

FEATURES ?=
SGX_SDK ?= $(HOME)/.sgxsdk/sgxsdk
SGX_MODE ?= HW
SGX_ARCH ?= x64
BUILD_PROFILE ?= release
FEATURES += SGX_MODE_$(SGX_MODE)
FEATURES := $(strip $(FEATURES))


TOP_DIR := ../../../third_party/build
include $(TOP_DIR)/buildenv.mk

ifeq ($(shell getconf LONG_BIT), 32)
	SGX_ARCH := x86
else ifeq ($(findstring -m32, $(CXXFLAGS)), -m32)
	SGX_ARCH := x86
endif

ifeq ($(SGX_ARCH), x86)
	SGX_COMMON_CFLAGS := -m32
	SGX_LIBRARY_PATH := $(SGX_SDK)/lib
else
	SGX_COMMON_CFLAGS := -m64
	SGX_LIBRARY_PATH := $(SGX_SDK)/lib64
endif

ifeq ($(SGX_DEBUG), 1)
ifeq ($(SGX_PRERELEASE), 1)
$(error Cannot set SGX_DEBUG and SGX_PRERELEASE at the same time!!)
endif
endif

######## CVE 2020-0551 (INTEL-SA-00334) mitigation flags ########

ifeq ($(MITIGATION-CVE-2020-0551), LOAD)
export MITIGATION_CVE_2020_0551=LOAD
else ifeq ($(MITIGATION-CVE-2020-0551), CF)
export MITIGATION_CVE_2020_0551=CF
endif

ifeq ($(SGX_DEBUG), 1)
	SGX_COMMON_CFLAGS += -O0 -g
else
	SGX_COMMON_CFLAGS += -O2
endif

ifneq (,$(findstring production,$(FEATURES)))
  ENCLAVE_CONFIG = Enclave.config.prod.xml
else
  ENCLAVE_CONFIG = Enclave.config.xml
endif


SGX_COMMON_CFLAGS += -fstack-protector

######## CUSTOM Settings ########

CUSTOM_LIBRARY_PATH := ./lib
CUSTOM_EDL_PATH := ../../../third_party/vendor/sgx_edl/edl
CUSTOM_COMMON_PATH := ../../../third_party/vendor/sgx_edl/common

######## EDL Settings ########

Enclave_EDL_Products := Enclave_t.c Enclave_t.h

######## Enclave Settings ########

ifneq ($(SGX_MODE), HW)
	Trts_Library_Name := sgx_trts_sim
	Service_Library_Name := sgx_tservice_sim
else
	Trts_Library_Name := sgx_trts
	Service_Library_Name := sgx_tservice
endif
Crypto_Library_Name := sgx_tcrypto
KeyExchange_Library_Name := sgx_tkey_exchange
ProtectedFs_Library_Name := sgx_tprotected_fs

RustEnclave_C_Files := $(wildcard ./*.c)
# TODO are we using this?
RustEnclave_C_Objects := $(RustEnclave_C_Files:.c=.o)
RustEnclave_Include_Paths := -I$(CUSTOM_COMMON_PATH)/inc -I$(CUSTOM_EDL_PATH) -I$(SGX_SDK)/include -I$(SGX_SDK)/include/tlibc -I$(SGX_SDK)/include/stlport -I$(SGX_SDK)/include/epid -I./ -I./include

RustEnclave_Link_Libs := -L$(CUSTOM_LIBRARY_PATH) -lenclave
RustEnclave_Compile_Flags := $(SGX_COMMON_CFLAGS) $(ENCLAVE_CFLAGS) $(RustEnclave_Include_Paths)
RustEnclave_Link_Flags := -Wl,--no-undefined -nostdlib -nodefaultlibs -nostartfiles -L$(SGX_LIBRARY_PATH) \
	-Wl,--whole-archive -l$(Trts_Library_Name) -Wl,--no-whole-archive \
	-Wl,--start-group -lsgx_tstdc -lsgx_tcxx -l$(Crypto_Library_Name) -l$(Service_Library_Name) -l$(ProtectedFs_Library_Name) $(RustEnclave_Link_Libs) -Wl,--end-group \
	-Wl,--version-script=Enclave.lds \
	$(ENCLAVE_LDFLAGS)


Rust_Crate_Source := $(shell find -type f | grep -vP '(\.so|\.o|\.a)$$' | grep -vP '^\./target' | grep -v Enclave_t | grep -v Makefile)
Rust_Target_Path := $(CURDIR)/../xargo

ifeq ($(SGX_MODE), SW)
	Rust_Flags := "-Z force-unstable-if-unmarked -C target-feature=+aes,+ssse3"
else
	Rust_Flags := "-Z force-unstable-if-unmarked -C target-cpu=skylake"
endif

.PHONY: all
all: enclave

.PHONY: enclave
enclave: enclave-ffi-types.h librust_cosmwasm_query_enclave.signed.so

FFI_TYPES_SOURCES := $(shell find "../ffi-types" -type f)
../target/headers/enclave-ffi-types.h: $(FFI_TYPES_SOURCES)
	cd ../ffi-types && cargo check --features "build_headers"

enclave-ffi-types.h: ../target/headers/enclave-ffi-types.h
	cp ../target/headers/enclave-ffi-types.h ./

.PHONY: build-protobuf
build-protobuf:
	cd ../shared/cosmos-proto && cargo check --features='build-protobuf'

librust_cosmwasm_query_enclave.signed.so: librust_cosmwasm_query_enclave.so $(ENCLAVE_CONFIG)
	sgx_sign sign -key Enclave_private.pem -enclave $< -out $@ -config $(ENCLAVE_CONFIG)

librust_cosmwasm_query_enclave.so: $(CUSTOM_LIBRARY_PATH)/libenclave.a Enclave_t.o
	$(CXX) Enclave_t.o -o $@ $(RustEnclave_Link_Flags)

$(CUSTOM_LIBRARY_PATH)/libenclave.a: $(CUSTOM_LIBRARY_PATH) $(Rust_Crate_Source)
	RUST_TARGET_PATH=$(Rust_Target_Path) RUSTFLAGS=$(Rust_Flags) xargo build --features "$(FEATURES)" --target x86_64-unknown-linux-sgx -Z unstable-options --profile $(BUILD_PROFILE) --locked
	cp ../target/x86_64-unknown-linux-sgx/$(BUILD_PROFILE)/libsecret_query_enclave.a $(CUSTOM_LIBRARY_PATH)/libenclave.a

$(CUSTOM_LIBRARY_PATH):
	mkdir $(CUSTOM_LIBRARY_PATH)

Enclave_t.o: $(Enclave_EDL_Products)
	$(CC) $(RustEnclave_Compile_Flags) -c Enclave_t.c -o $@

$(Enclave_EDL_Products): Enclave.edl
	sgx_edger8r --trusted $^ --search-path $(SGX_SDK)/include --search-path $(CUSTOM_EDL_PATH) --trusted-dir ./

check:
	RUST_TARGET_PATH=$(Rust_Target_Path) RUSTFLAGS=$(Rust_Flags) xargo check --features "$(FEATURES)" --target x86_64-unknown-linux-sgx

clippy:
	RUST_TARGET_PATH=$(Rust_Target_Path) RUSTFLAGS=$(Rust_Flags) xargo clippy --features "$(FEATURES)" --target x86_64-unknown-linux-sgx -- -D warnings

clean:
	rm -rf $(CUSTOM_LIBRARY_PATH) $(Enclave_EDL_Products) *.o *.so *.h
	xargo clean
