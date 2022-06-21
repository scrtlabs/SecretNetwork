SGX_SIGNER_KEY ?= ./Enclave_private.pem
SGX ?= 1

ARCH_LIBDIR ?= /lib/$(shell $(CC) -dumpmachine)

CFLAGS = -Wall -Wextra

GRAMINE_LOG_LEVEL = error

USE_MUSL ?= 0

BIN_NAME = secret-query-enclave

ifeq ($(USE_MUSL),1)
BIN_FILE = ../target/x86_64-unknown-linux-musl/release/${BIN_NAME}
CARGO_ARGS = --target x86_64-unknown-linux-musl
else
BIN_FILE = ../target/release/${BIN_NAME}
CARGO_ARGS =
endif

ifeq ($(V),1)
CARGO_ARGS += -vv
endif

PREFIX ?= ../bin
SEAL_DIR ?= data

.PHONY: all
all: ${BIN_NAME} ${BIN_NAME}.manifest
ifeq ($(SGX),1)
all: ${BIN_NAME}.manifest.sgx ${BIN_NAME}.sig ${BIN_NAME}.token
endif

.PHONY: ${BIN_FILE}
${BIN_FILE}:
	cargo build --release ${CARGO_ARGS}

${BIN_NAME}: ${BIN_FILE}
	cp ${BIN_FILE} ${BIN_NAME}

${BIN_NAME}.manifest: ${BIN_NAME}.manifest.template
	mkdir -p data
	gramine-manifest \
		-Dinstall_dir=$(INSTALL_DIR) \
		-Dlog_level=$(GRAMINE_LOG_LEVEL) \
		-Dra_client_spid=${IAS_SPID} \
		-Dseal_dir=${SEAL_DIR} \
		$< $@

${BIN_NAME}.manifest.sgx: ${BIN_NAME}.manifest ${BIN_FILE}
	@test -s $(SGX_SIGNER_KEY) || \
	    { echo "SGX signer private key was not found, please specify SGX_SIGNER_KEY!"; exit 1; }
	gramine-sgx-sign \
		--key $(SGX_SIGNER_KEY) \
		--manifest $< \
		--output $@

${BIN_NAME}.sig: ${BIN_NAME}.manifest.sgx

${BIN_NAME}.token: ${BIN_NAME}.sig
	gramine-sgx-get-token \
		--output $@ --sig $<

.PHONY: clean
clean:
	$(RM) *.token *.sig *.manifest.sgx *.manifest ${BIN_NAME}.o ${BIN_NAME} OUTPUT
	$(RM) -rf ../target

.PHONY: distclean
distclean: clean

.PHONY: dist
dist: all
	mkdir -p ${PREFIX}/data
	cp ${BIN_NAME} ${PREFIX}/
	cp Rocket.toml ${PREFIX}/
ifeq ($(SGX),1)
	cp ${BIN_NAME}.manifest.sgx ${PREFIX}/
	cp ${BIN_NAME}.sig ${PREFIX}/
	cp ${BIN_NAME}.token ${PREFIX}/
endif
	cp ${BIN_NAME}.manifest ${PREFIX}/

run: all
ifeq ($(SGX),1)
	rm -rf data
	mkdir -p data
	gramine-sgx secret-query-enclave
else
	mkdir -p data
	gramine-direct secret-query-enclave
endif
