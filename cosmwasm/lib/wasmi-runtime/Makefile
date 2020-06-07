######## SGX SDK Settings ########

SGX_SDK ?= $(HOME)/.sgxsdk/sgxsdk
SGX_MODE ?= HW
SGX_ARCH ?= x64

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
RustEnclave_Compile_Flags := $(SGX_COMMON_CFLAGS) -nostdinc -fvisibility=hidden -fpie -fstack-protector $(RustEnclave_Include_Paths)
RustEnclave_Link_Flags := $(SGX_COMMON_CFLAGS) -Wl,--no-undefined -nostdlib -nodefaultlibs -nostartfiles -L$(SGX_LIBRARY_PATH) \
	-Wl,--whole-archive -l$(Trts_Library_Name) -Wl,--no-whole-archive \
	-Wl,--start-group -lsgx_tstdc -lsgx_tcxx -l$(Crypto_Library_Name) -l$(Service_Library_Name) -l$(ProtectedFs_Library_Name) $(RustEnclave_Link_Libs) -Wl,--end-group \
	-Wl,--version-script=Enclave.lds \
	$(ENCLAVE_LDFLAGS)


Rust_Crate_Source := $(wildcard src/*.rs)
Rust_Target_Path := $(CURDIR)/xargo

.PHONY: all
all: enclave

.PHONY: enclave
enclave: ffi-types librust_cosmwasm_enclave.signed.so

ffi-types:
	mkdir -p target/headers/
	cd ../enclave-ffi-types && \
	RUST_TARGET_PATH=$(Rust_Target_Path) xargo build --features "build_headers" --target x86_64-unknown-linux-sgx --release && \
	cp $(Rust_Target_Path)/../../../target/headers/enclave-ffi-types.h $(Rust_Target_Path)/../target/headers/

librust_cosmwasm_enclave.signed.so: librust_cosmwasm_enclave.so
	sgx_sign sign -key Enclave_private.pem -enclave $^ -out $@ -config Enclave.config.xml

librust_cosmwasm_enclave.so: $(CUSTOM_LIBRARY_PATH)/libenclave.a Enclave_t.o
	$(CXX) Enclave_t.o -o $@ $(RustEnclave_Link_Flags)

$(CUSTOM_LIBRARY_PATH)/libenclave.a: $(CUSTOM_LIBRARY_PATH) $(Rust_Crate_Source)
	RUST_TARGET_PATH=$(Rust_Target_Path) RUSTFLAGS="-Z force-unstable-if-unmarked -Ctarget-feature=+aes,+ssse3" xargo build --features "SGX_MODE_$(SGX_MODE)" --target x86_64-unknown-linux-sgx --release
	cp target/x86_64-unknown-linux-sgx/release/libwasmi_runtime_enclave.a $(CUSTOM_LIBRARY_PATH)/libenclave.a

$(CUSTOM_LIBRARY_PATH):
	mkdir $(CUSTOM_LIBRARY_PATH)

Enclave_t.o: $(Enclave_EDL_Products)
	$(CC) $(RustEnclave_Compile_Flags) -c Enclave_t.c -o $@

$(Enclave_EDL_Products): Enclave.edl
	sgx_edger8r --trusted $^ --search-path $(SGX_SDK)/include --search-path $(CUSTOM_EDL_PATH) --trusted-dir ./

check:
	RUST_TARGET_PATH=$(Rust_Target_Path) RUSTFLAGS="-Z force-unstable-if-unmarked" xargo check --features "SGX_MODE_$(SGX_MODE)" --target x86_64-unknown-linux-sgx

clean:
	rm -rf $(CUSTOM_LIBRARY_PATH) $(Enclave_EDL_Products) *.o *.so
	xargo clean
