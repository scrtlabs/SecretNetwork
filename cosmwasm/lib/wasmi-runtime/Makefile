######## SGX SDK Settings ########

SGX_SDK ?= $(HOME)/.sgxsdk/sgxsdk
SGX_MODE ?= HW
SGX_ARCH ?= x64

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


ifeq ($(SGX_DEBUG), 1)
	SGX_COMMON_CFLAGS += -O0 -g
else
	SGX_COMMON_CFLAGS += -O2
endif

######## CUSTOM Settings ########

CUSTOM_LIBRARY_PATH := ./lib
CUSTOM_EDL_PATH := ../incubator-teaclave-sgx-sdk/edl
CUSTOM_COMMON_PATH := ../incubator-teaclave-sgx-sdk/common

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

RustEnclave_Link_Libs := -L$(CUSTOM_LIBRARY_PATH) -lcompiler-rt-patch -lenclave
RustEnclave_Compile_Flags := $(SGX_COMMON_CFLAGS) -nostdinc -fvisibility=hidden -fpie -fstack-protector $(RustEnclave_Include_Paths)
RustEnclave_Link_Flags := $(SGX_COMMON_CFLAGS) -Wl,--no-undefined -nostdlib -nodefaultlibs -nostartfiles -L$(SGX_LIBRARY_PATH) \
	-Wl,--whole-archive -l$(Trts_Library_Name) -Wl,--no-whole-archive \
	-Wl,--start-group -lsgx_tstdc -l$(Service_Library_Name) -l$(Crypto_Library_Name) $(RustEnclave_Link_Libs) -Wl,--end-group \
	-Wl,-Bstatic -Wl,-Bsymbolic -Wl,--no-undefined \
	-Wl,-pie,-eenclave_entry -Wl,--export-dynamic  \
	-Wl,--defsym,__ImageBase=0 \
	-Wl,--gc-sections \
	-Wl,--version-script=Enclave.lds

Rust_Crate_Source := $(wildcard src/*.rs)
Rust_Target_Path := $(CURDIR)/../incubator-teaclave-sgx-sdk/xargo

.PHONY: all
all: enclave

.PHONY: enclave
enclave: librust_cosmwasm_enclave.signed.so

librust_cosmwasm_enclave.signed.so: librust_cosmwasm_enclave.so
	sgx_sign sign -key Enclave_private.pem -enclave $^ -out $@ -config Enclave.config.xml

librust_cosmwasm_enclave.so: $(CUSTOM_LIBRARY_PATH)/libenclave.a Enclave_t.o compiler-rt
	$(CXX) Enclave_t.o -o $@ $(RustEnclave_Link_Flags)

.PHONY: compiler-rt
compiler-rt: $(CUSTOM_LIBRARY_PATH)
	$(MAKE) -C ../incubator-teaclave-sgx-sdk/compiler-rt/ 2> /dev/null
	cp ../incubator-teaclave-sgx-sdk/compiler-rt/libcompiler-rt-patch.a $(CUSTOM_LIBRARY_PATH)/libcompiler-rt-patch.a

$(CUSTOM_LIBRARY_PATH)/libenclave.a: $(CUSTOM_LIBRARY_PATH) $(Rust_Crate_Source)
	RUST_TARGET_PATH=$(Rust_Target_Path) RUSTFLAGS="-Z force-unstable-if-unmarked" xargo build --target x86_64-unknown-linux-sgx --release
	cp target/x86_64-unknown-linux-sgx/release/libwasmi_runtime_enclave.a $(CUSTOM_LIBRARY_PATH)/libenclave.a

$(CUSTOM_LIBRARY_PATH):
	mkdir $(CUSTOM_LIBRARY_PATH)

Enclave_t.o: $(Enclave_EDL_Products)
	$(CC) $(RustEnclave_Compile_Flags) -c Enclave_t.c -o $@

$(Enclave_EDL_Products): Enclave.edl
	sgx_edger8r --trusted $^ --search-path $(SGX_SDK)/include --search-path $(CUSTOM_EDL_PATH) --trusted-dir ./

clean:
	rm -rf $(CUSTOM_LIBRARY_PATH) $(Enclave_EDL_Products) *.o *.so
	xargo clean
