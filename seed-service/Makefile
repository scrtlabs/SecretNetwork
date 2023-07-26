BUILD_PROFILE ?= release
FEATURES ?=
FEATURES_U += $(FEATURES)
FEATURES_U += backtraces
FEATURES_U := $(strip $(FEATURES_U))

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


ARCH_LIBDIR ?= /lib/$(shell $(CC) -dumpmachine)

SELF_EXE = target/release/singularity_seed_service

.PHONY: all
# all: vendor $(SELF_EXE) singularity_seed_service.manifest
all: $(SELF_EXE) singularity_seed_service.manifest
ifeq ($(SGX),1)
all: singularity_seed_service.manifest.sgx singularity_seed_service.sig singularity_seed_service.token
endif

ifeq ($(DEBUG),1)
GRAMINE_LOG_LEVEL = debug
else
GRAMINE_LOG_LEVEL = error
endif

# move the secretNetwork parts to another Makefile. but call vendor here. 
# then make SGX=1 and see if good. 
# then add what's needed in ./Cargo.toml (regarding the enclave-ffi-types and sgx_types. (do i need xargo for sgx_types?? - check sgx-vm (i think not, but maybe it's dependant on compiling first 'enclaves/execute')))
vendor:
	cargo vendor third_party/vendor --manifest-path third_party/build/Cargo.toml
	# $(MAKE) -C ./src all

# Note that we're compiling in release mode regardless of the DEBUG setting passed
# to Make, as compiling in debug mode results in an order of magnitude's difference in
# performance that makes testing by running a benchmark with ab painful. The primary goal
# of the DEBUG setting is to control Gramine's loglevel.
-include $(SELF_EXE).d # See also: .cargo/config.toml
$(SELF_EXE): Cargo.toml src/main.rs src/db.rs
	cargo build --release


singularity_seed_service.manifest: singularity_seed_service.manifest.template
	gramine-manifest \
		-Dlog_level=$(GRAMINE_LOG_LEVEL) \
		-Darch_libdir=$(ARCH_LIBDIR) \
		-Dself_exe=$(SELF_EXE) \
		$< $@

# Make on Ubuntu <= 20.04 doesn't support "Rules with Grouped Targets" (`&:`),
# see the helloworld example for details on this workaround.
singularity_seed_service.manifest.sgx singularity_seed_service.sig: sgx_sign
	@:

.INTERMEDIATE: sgx_sign
sgx_sign: singularity_seed_service.manifest $(SELF_EXE)
	gramine-sgx-sign \
		--manifest $< \
		--output $<.sgx

singularity_seed_service.token: singularity_seed_service.sig
	gramine-sgx-get-token \
		--output $@ --sig $<

ifeq ($(SGX),)
GRAMINE = gramine-direct
else
GRAMINE = gramine-sgx
endif

.PHONY: start-gramine-server
start-gramine-server: all
	$(GRAMINE) singularity_seed_service

.PHONY: clean
clean:
	$(RM) -rf *.token *.sig *.manifest.sgx *.manifest result-* OUTPUT
	-rm -rf /tmp/SecretNetwork
	-rm -f ./secretcli*
	-rm -f ./secretd*
	-find -name '*.so' -delete
	-rm -f ./enigma-blockchain*.deb
	-rm -f ./SHA256SUMS*
	-rm -rf ./third_party/vendor/
	-rm -rf ./.sgx_secrets/*
	-rm -rf ./x/compute/internal/keeper/.sgx_secrets/*
	-rm -rf ./*.der
	-rm -rf ./x/compute/internal/keeper/*.der
	-rm -rf ./cmd/secretd/ias_bin*
	-rm ./Cargo.lock
	-rm -rf ./target
	#-rm ./src/enclaves/Cargo.lock
	#$(MAKE) -C ./src clean-all

.PHONY: distclean
distclean: clean
	$(RM) -rf target/ Cargo.lock
