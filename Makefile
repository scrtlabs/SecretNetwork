PACKAGES=$(shell go list ./... | grep -v '/simulation')
VERSION ?= $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')
DOCKER := $(shell which docker)
DOCKER_BUF := $(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace bufbuild/buf

SPID ?= 00000000000000000000000000000000
API_KEY ?= FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF

LEDGER_ENABLED ?= true
BINDIR ?= $(GOPATH)/bin
BUILD_PROFILE ?= release
DEB_BIN_DIR ?= /usr/local/bin
DEB_LIB_DIR ?= /usr/lib

DB_BACKEND ?= goleveldb

SGX_MODE ?= HW
BRANCH ?= develop
DEBUG ?= 0
DOCKER_TAG ?= latest

TM_SGX ?= true

CW_CONTRACTS_V010_PATH = ./cosmwasm/contracts/v010/
CW_CONTRACTS_V1_PATH = ./cosmwasm/contracts/v1/

TEST_CONTRACT_V010_PATH = ./cosmwasm/contracts/v010/compute-tests
TEST_CONTRACT_V1_PATH = ./cosmwasm/contracts/v1/compute-tests

TEST_COMPUTE_MODULE_PATH = ./x/compute/internal/keeper/testdata/

ENCLAVE_PATH = cosmwasm/enclaves/
EXECUTE_ENCLAVE_PATH = $(ENCLAVE_PATH)/execute/
DOCKER_BUILD_ARGS ?=

DOCKER_BUILDX_CHECK = $(@shell docker build --load test)

ifeq (Building,$(findstring Building,$(DOCKER_BUILDX_CHECK)))
	DOCKER_BUILD_ARGS += "--load"
endif

ifeq ($(SGX_MODE), HW)
	ext := hw
else ifeq ($(SGX_MODE), SW)
	ext := sw
else
$(error SGX_MODE must be either HW or SW)
endif

ifeq ($(DB_BACKEND), rocksdb)
	DB_BACKEND = rocksdb
	DOCKER_CGO_LDFLAGS = "-L/usr/lib/x86_64-linux-gnu/ -lrocksdb -lstdc++ -llz4 -lm -lz -lbz2 -lsnappy"
	DOCKER_CGO_FLAGS = "-I/opt/rocksdb/include"
else ifeq ($(DB_BACKEND), cleveldb)
	DB_BACKEND = cleveldb
else ifeq ($(DB_BACKEND), goleveldb)
	DB_BACKEND = goleveldb
	DOCKER_CGO_LDFLAGS = ""
else
$(error DB_BACKEND must be one of: rocksdb/cleveldb/goleveldb)
endif

CUR_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

build_tags = netgo
ifeq ($(LEDGER_ENABLED),true)
  ifeq ($(OS),Windows_NT)
    GCCEXE = $(shell where gcc.exe 2> NUL)
    ifeq ($(GCCEXE),)
      $(error "gcc.exe not installed for ledger support, please install or set LEDGER_ENABLED=false")
    else
      build_tags += ledger
    endif
  else
    UNAME_S = $(shell uname -s)
    ifeq ($(UNAME_S),OpenBSD)
      $(warning "OpenBSD detected, disabling ledger support (https://github.com/cosmos/cosmos-sdk/issues/1988)")
    else
      GCC = $(shell command -v gcc 2> /dev/null)
      ifeq ($(GCC),)
        $(error "gcc not installed for ledger support, please install or set LEDGER_ENABLED=false")
      else
        build_tags += ledger
      endif
    endif
  endif
endif

IAS_BUILD = sw

ifeq ($(SGX_MODE), HW)
  ifneq (,$(findstring production,$(FEATURES)))
    IAS_BUILD = production
  else
    IAS_BUILD = develop
  endif

  build_tags += hw
  build_tags += sgx
else
  ifeq ($(TM_SGX), true)
    build_tags += sgx
  endif
endif

build_tags += $(IAS_BUILD)

ifeq ($(DB_BACKEND),rocksdb)
  build_tags += gcc
endif
ifeq ($(DB_BACKEND),cleveldb)
  build_tags += gcc
endif
build_tags += $(BUILD_TAGS)
build_tags := $(strip $(build_tags))

whitespace :=
whitespace += $(whitespace)
comma := ,
build_tags_comma_sep := $(subst $(whitespace),$(comma),$(build_tags))

ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=SecretNetwork \
	-X github.com/cosmos/cosmos-sdk/version.AppName=secretd \
	-X github.com/scrtlabs/SecretNetwork/cmd/secretcli/version.ClientName=secretcli \
	-X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
	-X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
	-X "github.com/cosmos/cosmos-sdk/version.BuildTags=$(build_tags)"

ifeq ($(DB_BACKEND),cleveldb)
  ldflags += -X github.com/cosmos/cosmos-sdk/types.DBBackend=cleveldb
endif
ifeq ($(DB_BACKEND),rocksdb)
  CGO_ENABLED=1
  build_tags += rocksdb
  ldflags += -X github.com/cosmos/cosmos-sdk/types.DBBackend=rocksdb
  ldflags += -extldflags "-lrocksdb -llz4"
endif

ldflags += -s -w
ldflags += $(LDFLAGS)
ldflags := $(strip $(ldflags))

GO_TAGS := $(build_tags)
# -ldflags
LD_FLAGS := $(ldflags)

all: build_all

go.sum: go.mod
	@echo "--> Ensure dependencies have not been modified"
	GO111MODULE=on go mod verify

build_cli:
	go build -o secretcli -mod=readonly -tags "$(filter-out sgx, $(GO_TAGS)) secretcli" -ldflags '$(LD_FLAGS)' ./cmd/secretd

xgo_build_secretcli: go.sum
	xgo --targets $(XGO_TARGET) -tags="$(filter-out sgx, $(GO_TAGS)) secretcli" -ldflags '$(LD_FLAGS)' --pkg cmd/secretd .

build_local_no_rust: bin-data-$(IAS_BUILD)
	cp go-cosmwasm/target/$(BUILD_PROFILE)/libgo_cosmwasm.so go-cosmwasm/api
	go build -mod=readonly -tags "$(GO_TAGS)" -ldflags '$(LD_FLAGS)' ./cmd/secretd

build-secret: build-linux

build-linux: _build-linux build_local_no_rust build_cli
_build-linux:
	BUILD_PROFILE=$(BUILD_PROFILE) FEATURES="$(FEATURES)" FEATURES_U="$(FEATURES_U) light-client-validation go-tests" $(MAKE) -C go-cosmwasm build-rust

build-tm-secret-enclave:
	git clone https://github.com/scrtlabs/tm-secret-enclave.git /tmp/tm-secret-enclave || true
	cd /tmp/tm-secret-enclave && git checkout v1.9.3 && git submodule init && git submodule update --remote
	rustup component add rust-src
	SGX_MODE=$(SGX_MODE) $(MAKE) -C /tmp/tm-secret-enclave build

build_windows_cli:
	$(MAKE) xgo_build_secretcli XGO_TARGET=windows/amd64
	sudo mv github.com/scrtlabs/SecretNetwork-windows-* secretcli-windows-amd64.exe

build_macos_cli:
	$(MAKE) xgo_build_secretcli XGO_TARGET=darwin/amd64
	sudo mv github.com/scrtlabs/SecretNetwork-darwin-amd64 secretcli-macos-amd64

build_macos_arm64_cli:
	$(MAKE) xgo_build_secretcli XGO_TARGET=darwin/arm64
	sudo mv github.com/scrtlabs/SecretNetwork-darwin-arm64 secretcli-macos-arm64

build_linux_cli:
	$(MAKE) xgo_build_secretcli XGO_TARGET=linux/amd64
	sudo mv github.com/scrtlabs/SecretNetwork-linux-amd64 secretcli-linux-amd64

build_linux_arm64_cli:
	$(MAKE) xgo_build_secretcli XGO_TARGET=linux/arm64
	sudo mv github.com/scrtlabs/SecretNetwork-linux-arm64 secretcli-linux-arm64

build_all: build-linux build_windows_cli build_macos_cli build_linux_arm64_cli

deb: build-linux deb-no-compile

deb-no-compile:
    ifneq ($(UNAME_S),Linux)
		exit 1
    endif
	rm -rf /tmp/SecretNetwork

	mkdir -p /tmp/SecretNetwork/deb/$(DEB_BIN_DIR)
	cp -f ./secretcli /tmp/SecretNetwork/deb/$(DEB_BIN_DIR)/secretcli
	cp -f ./secretd /tmp/SecretNetwork/deb/$(DEB_BIN_DIR)/secretd
	chmod +x /tmp/SecretNetwork/deb/$(DEB_BIN_DIR)/secretd /tmp/SecretNetwork/deb/$(DEB_BIN_DIR)/secretcli

	mkdir -p /tmp/SecretNetwork/deb/$(DEB_LIB_DIR)
	cp -f ./go-cosmwasm/tendermint_enclave.signed.so ./go-cosmwasm/librandom_api.so ./go-cosmwasm/api/libgo_cosmwasm.so ./go-cosmwasm/librust_cosmwasm_enclave.signed.so /tmp/SecretNetwork/deb/$(DEB_LIB_DIR)/
	chmod +x /tmp/SecretNetwork/deb/$(DEB_LIB_DIR)/lib*.so

	mkdir -p /tmp/SecretNetwork/deb/DEBIAN
	cp ./deployment/deb/control /tmp/SecretNetwork/deb/DEBIAN/control
	printf "Version: " >> /tmp/SecretNetwork/deb/DEBIAN/control
	printf "$(VERSION)" >> /tmp/SecretNetwork/deb/DEBIAN/control
	echo "" >> /tmp/SecretNetwork/deb/DEBIAN/control
	cp ./deployment/deb/postinst /tmp/SecretNetwork/deb/DEBIAN/postinst
	chmod 755 /tmp/SecretNetwork/deb/DEBIAN/postinst
	cp ./deployment/deb/postrm /tmp/SecretNetwork/deb/DEBIAN/postrm
	chmod 755 /tmp/SecretNetwork/deb/DEBIAN/postrm
	cp ./deployment/deb/triggers /tmp/SecretNetwork/deb/DEBIAN/triggers
	chmod 755 /tmp/SecretNetwork/deb/DEBIAN/triggers
	dpkg-deb --build /tmp/SecretNetwork/deb/ .
	-rm -rf /tmp/SecretNetwork

clean:
	-rm -rf /tmp/SecretNetwork
	-rm -f ./secretcli*
	-rm -f ./secretd*
	-find -name '*.so' -not -path './third_party/*' -delete
	-rm -f ./enigma-blockchain*.deb
	-rm -f ./SHA256SUMS*
	-rm -rf ./third_party/vendor/
	-rm -rf ./.sgx_secrets/*
	-rm -rf ./x/compute/internal/keeper/.sgx_secrets/*
	-rm -rf ./*.der
	-rm -rf ./x/compute/internal/keeper/*.der
	-rm -rf ./cmd/secretd/ias_bin*
	$(MAKE) -C go-cosmwasm clean-all
	$(MAKE) -C cosmwasm/enclaves/test clean
	$(MAKE) -C check-hw clean
	$(MAKE) -C $(TEST_CONTRACT_V010_PATH)/test-compute-contract clean
	$(MAKE) -C $(TEST_CONTRACT_V010_PATH)/test-compute-contract-v2 clean
	$(MAKE) -C $(TEST_CONTRACT_V1_PATH)/test-compute-contract clean
	$(MAKE) -C $(TEST_CONTRACT_V1_PATH)/test-compute-contract-v2 clean

localsecret:
	DOCKER_BUILDKIT=1 docker build \
			--build-arg FEATURES="${FEATURES},debug-print,random,light-client-validation" \
			--build-arg FEATURES_U=${FEATURES_U} \
			--secret id=API_KEY,src=.env.local \
			--secret id=SPID,src=.env.local \
			--build-arg SGX_MODE=SW \
			$(DOCKER_BUILD_ARGS) \
 			--build-arg SECRET_NODE_TYPE=BOOTSTRAP \
			--build-arg CHAINID=secretdev-1 \
 			-f deployment/dockerfiles/Dockerfile \
 			--target build-localsecret \
 			-t ghcr.io/scrtlabs/localsecret:${DOCKER_TAG} .

build-ibc-hermes:
	docker build -f deployment/dockerfiles/ibc/hermes.Dockerfile -t hermes:v0.0.0 deployment/dockerfiles/ibc

build-testnet-bootstrap:
	@mkdir build 2>&3 || true
	DOCKER_BUILDKIT=1 docker build --build-arg BUILDKIT_INLINE_CACHE=1 \
				 --secret id=API_KEY,src=api_key.txt \
				 --secret id=SPID,src=spid.txt \
				 --build-arg BUILD_VERSION=${VERSION} \
				 --build-arg SGX_MODE=HW \
				 $(DOCKER_BUILD_ARGS) \
				 --build-arg DB_BACKEND=${DB_BACKEND} \
				 --build-arg SECRET_NODE_TYPE=BOOTSTRAP \
				 --build-arg CGO_LDFLAGS=${DOCKER_CGO_LDFLAGS} \
				 -f deployment/dockerfiles/Dockerfile \
				 -t ghcr.io/scrtlabs/testnet:${DOCKER_TAG} \
				 --target release-image .

build-testnet:
	@mkdir build 2>&3 || true
	DOCKER_BUILDKIT=1 docker build --build-arg BUILDKIT_INLINE_CACHE=1 \
				 --secret id=API_KEY,src=api_key.txt \
				 --secret id=SPID,src=spid.txt \
				 --build-arg BUILD_VERSION=${VERSION} \
				 --build-arg SGX_MODE=HW \
				 --build-arg FEATURES="verify-validator-whitelist,light-client-validation,random,${FEATURES}" \
				 $(DOCKER_BUILD_ARGS) \
				 --build-arg DB_BACKEND=${DB_BACKEND} \
				 --build-arg SECRET_NODE_TYPE=NODE \
				 --build-arg CGO_LDFLAGS=${DOCKER_CGO_LDFLAGS} \
				 -f deployment/dockerfiles/Dockerfile \
				 -t ghcr.io/scrtlabs/secret-network-node-testnet:v$(VERSION) \
				 --target release-image .
	DOCKER_BUILDKIT=1 docker build --build-arg BUILDKIT_INLINE_CACHE=1 \
				 --secret id=API_KEY,src=api_key.txt \
				 --secret id=SPID,src=spid.txt \
				 --build-arg BUILD_VERSION=${VERSION} \
				 --build-arg SGX_MODE=HW \
				 --build-arg FEATURES="verify-validator-whitelist,light-client-validation,random,${FEATURES}" \
				 $(DOCKER_BUILD_ARGS) \
				 --build-arg CGO_LDFLAGS=${DOCKER_CGO_LDFLAGS} \
				 --build-arg DB_BACKEND=${DB_BACKEND} \
				 --cache-from ghcr.io/scrtlabs/secret-network-node-testnet:v$(VERSION) \
				 -f deployment/dockerfiles/Dockerfile \
				 -t deb_build \
				 --target build-deb .
	docker run -e VERSION=${VERSION} -v $(CUR_DIR)/build:/build deb_build

build-mainnet-upgrade:
	@mkdir build 2>&3 || true
	DOCKER_BUILDKIT=1 docker build --build-arg FEATURES="verify-validator-whitelist,light-client-validation,production, ${FEATURES}" \
                 --build-arg FEATURES_U="production, ${FEATURES_U}" \
                 --build-arg BUILDKIT_INLINE_CACHE=1 \
                 --secret id=API_KEY,src=api_key.txt \
                 --secret id=SPID,src=spid.txt \
                 --build-arg SECRET_NODE_TYPE=NODE \
                 --build-arg DB_BACKEND=${DB_BACKEND} \
                 --build-arg BUILD_VERSION=${VERSION} \
                 --build-arg SGX_MODE=HW \
                 -f deployment/dockerfiles/Dockerfile \
                 $(DOCKER_BUILD_ARGS) \
                 -t ghcr.io/scrtlabs/secret-network-node:v$(VERSION) \
                 --target mainnet-release .
	DOCKER_BUILDKIT=1 docker build --build-arg FEATURES="verify-validator-whitelist,light-client-validation,production, ${FEATURES}" \
				 --build-arg FEATURES_U="production, ${FEATURES_U}" \
				 --build-arg BUILDKIT_INLINE_CACHE=1 \
				 --secret id=API_KEY,src=api_key.txt \
				 --secret id=SPID,src=spid.txt \
				 --build-arg DB_BACKEND=${DB_BACKEND} \
				 --build-arg BUILD_VERSION=${VERSION} \
				 --build-arg SGX_MODE=HW \
				 -f deployment/dockerfiles/Dockerfile \
				 -t deb_build \
				 --target build-deb-mainnet .
	docker run -e VERSION=${VERSION} -v $(CUR_DIR)/build:/build deb_build
build-mainnet:
	@mkdir build 2>&3 || true
	DOCKER_BUILDKIT=1 docker build --build-arg FEATURES="verify-validator-whitelist,light-client-validation,production,random, ${FEATURES}" \
                 --build-arg FEATURES_U=${FEATURES_U} \
                 --build-arg BUILDKIT_INLINE_CACHE=1 \
                 --secret id=API_KEY,src=api_key.txt \
                 --secret id=SPID,src=spid.txt \
                 --build-arg SECRET_NODE_TYPE=NODE \
                 --build-arg BUILD_VERSION=${VERSION} \
                 --build-arg SGX_MODE=HW \
                 --build-arg CGO_LDFLAGS=${DOCKER_CGO_LDFLAGS} \
                 --build-arg DB_BACKEND=${DB_BACKEND} \
                 $(DOCKER_BUILD_ARGS) \
                 -f deployment/dockerfiles/Dockerfile \
                 -t ghcr.io/scrtlabs/secret-network-node:v$(VERSION) \
                 --target release-image .
	DOCKER_BUILDKIT=1 docker build --build-arg FEATURES="verify-validator-whitelist,light-client-validation,production,random, ${FEATURES}" \
				 --build-arg FEATURES_U=${FEATURES_U} \
				 --build-arg BUILDKIT_INLINE_CACHE=1 \
				 --secret id=API_KEY,src=api_key.txt \
				 --secret id=SPID,src=spid.txt \
				 --build-arg BUILD_VERSION=${VERSION} \
				 --build-arg DB_BACKEND=${DB_BACKEND} \
				 --build-arg CGO_LDFLAGS=${DOCKER_CGO_LDFLAGS} \
				 --build-arg SGX_MODE=HW \
				 -f deployment/dockerfiles/Dockerfile \
				 -t deb_build \
				 $(DOCKER_BUILD_ARGS) \
				 --target build-deb .
	docker run -e VERSION=${VERSION} -v $(CUR_DIR)/build:/build deb_build

build-check-hw-tool:
	@mkdir build 2>&3 || true
	DOCKER_BUILDKIT=1 docker build --build-arg FEATURES="${FEATURES}" \
                 --build-arg FEATURES_U=${FEATURES_U} \
                 --build-arg BUILDKIT_INLINE_CACHE=1 \
                 --secret id=API_KEY,src=ias_keys/develop/api_key.txt \
				 --secret id=API_KEY_MAINNET,src=ias_keys/production/api_key.txt \
                 --secret id=SPID,src=spid.txt \
                 --build-arg SECRET_NODE_TYPE=NODE \
                 --build-arg BUILD_VERSION=${VERSION} \
                 --build-arg SGX_MODE=HW \
                 --build-arg DB_BACKEND=${DB_BACKEND} \
                 -f deployment/dockerfiles/Dockerfile \
                 -t compile-check-hw-tool \
                 --target compile-check-hw-tool .

# while developing:
build-enclave:
	$(MAKE) -C $(EXECUTE_ENCLAVE_PATH) enclave

# while developing:
check-enclave:
	$(MAKE) -C $(EXECUTE_ENCLAVE_PATH) check

# while developing:
clippy-enclave:
	$(MAKE) -C $(EXECUTE_ENCLAVE_PATH) clippy

# while developing:
clean-enclave:
	$(MAKE) -C $(EXECUTE_ENCLAVE_PATH) clean

# while developing:
clippy: clippy-enclave
	$(MAKE) -C check-hw clippy

sanity-test:
	SGX_MODE=SW $(MAKE) build-linux
	cp ./$(EXECUTE_ENCLAVE_PATH)/librust_cosmwasm_enclave.signed.so .
	SGX_MODE=SW ./cosmwasm/testing/sanity-test.sh

sanity-test-hw:
	$(MAKE) build-linux
	cp ./$(EXECUTE_ENCLAVE_PATH)/librust_cosmwasm_enclave.signed.so .
	./cosmwasm/testing/sanity-test.sh

callback-sanity-test:
	SGX_MODE=SW $(MAKE) build-linux
	cp ./$(EXECUTE_ENCLAVE_PATH)/librust_cosmwasm_enclave.signed.so .
	SGX_MODE=SW ./cosmwasm/testing/callback-test.sh

build-bench-contract:
	$(MAKE) -C $(TEST_CONTRACT_V1_PATH)/bench-contract
	cp $(TEST_CONTRACT_V1_PATH)/bench-contract/*.wasm $(TEST_COMPUTE_MODULE_PATH)/

build-test-contracts:
	# echo "" | sudo add-apt-repository ppa:hnakamur/binaryen
	# sudo apt update
	# sudo apt install -y binaryen
	$(MAKE) -C $(TEST_CONTRACT_V010_PATH)/test-compute-contract
	
	rm -f $(TEST_COMPUTE_MODULE_PATH)/contract.wasm
	cp $(TEST_CONTRACT_V010_PATH)/test-compute-contract/contract.wasm $(TEST_COMPUTE_MODULE_PATH)/contract.wasm

	rm -f $(TEST_COMPUTE_MODULE_PATH)/contract_with_floats.wasm
	cp $(TEST_CONTRACT_V010_PATH)/test-compute-contract/contract_with_floats.wasm $(TEST_COMPUTE_MODULE_PATH)/contract_with_floats.wasm

	rm -f $(TEST_COMPUTE_MODULE_PATH)/static-too-high-initial-memory.wasm
	cp $(TEST_CONTRACT_V010_PATH)/test-compute-contract/static-too-high-initial-memory.wasm $(TEST_COMPUTE_MODULE_PATH)/static-too-high-initial-memory.wasm

	rm -f $(TEST_COMPUTE_MODULE_PATH)/too-high-initial-memory.wasm
	cp $(TEST_CONTRACT_V010_PATH)/test-compute-contract/too-high-initial-memory.wasm $(TEST_COMPUTE_MODULE_PATH)/too-high-initial-memory.wasm

	$(MAKE) -C $(TEST_CONTRACT_V010_PATH)/test-compute-contract-v2
	
	rm -f $(TEST_COMPUTE_MODULE_PATH)/contract-v2.wasm
	cp $(TEST_CONTRACT_V010_PATH)/test-compute-contract-v2/contract-v2.wasm $(TEST_COMPUTE_MODULE_PATH)/contract-v2.wasm

	$(MAKE) -C $(TEST_CONTRACT_V1_PATH)/test-compute-contract
	rm -f $(TEST_COMPUTE_MODULE_PATH)/v1-contract.wasm
	cp $(TEST_CONTRACT_V1_PATH)/test-compute-contract/v1-contract.wasm $(TEST_COMPUTE_MODULE_PATH)/v1-contract.wasm

	$(MAKE) -C $(TEST_CONTRACT_V1_PATH)/test-compute-contract-v2
	rm -f $(TEST_COMPUTE_MODULE_PATH)/v1-contract-v2.wasm
	cp $(TEST_CONTRACT_V1_PATH)/test-compute-contract-v2/v1-contract-v2.wasm $(TEST_COMPUTE_MODULE_PATH)/v1-contract-v2.wasm

	$(MAKE) -C $(TEST_CONTRACT_V1_PATH)/ibc-test-contract
	rm -f $(TEST_COMPUTE_MODULE_PATH)/ibc.wasm
	cp $(TEST_CONTRACT_V1_PATH)/ibc-test-contract/ibc.wasm $(TEST_COMPUTE_MODULE_PATH)/ibc.wasm

	$(MAKE) -C $(TEST_CONTRACT_V1_PATH)/migration/contract-v1
	rm -f $(TEST_COMPUTE_MODULE_PATH)/migrate_contract_v1.wasm
	cp $(TEST_CONTRACT_V1_PATH)/migration/contract-v1/migrate_contract_v1.wasm $(TEST_COMPUTE_MODULE_PATH)/migrate_contract_v1.wasm

	$(MAKE) -C $(TEST_CONTRACT_V1_PATH)/migration/contract-v2
	rm -f $(TEST_COMPUTE_MODULE_PATH)/migrate_contract_v2.wasm
	cp $(TEST_CONTRACT_V1_PATH)/migration/contract-v2/migrate_contract_v2.wasm $(TEST_COMPUTE_MODULE_PATH)/migrate_contract_v2.wasm

	$(MAKE) -C $(TEST_CONTRACT_V1_PATH)/random-test
	rm -f $(TEST_COMPUTE_MODULE_PATH)/v1_random_test.wasm
	cp $(TEST_CONTRACT_V1_PATH)/random-test/v1_random_test.wasm $(TEST_COMPUTE_MODULE_PATH)/v1_random_test.wasm


prep-go-tests: build-test-contracts bin-data-sw
	# empty BUILD_PROFILE means debug mode which compiles faster
	SGX_MODE=SW $(MAKE) build-linux
	cp ./$(EXECUTE_ENCLAVE_PATH)/librust_cosmwasm_enclave.signed.so ./x/compute/internal/keeper
	cp ./$(EXECUTE_ENCLAVE_PATH)/librust_cosmwasm_enclave.signed.so .

go-tests: build-test-contracts bin-data-sw
	# SGX_MODE=SW $(MAKE) build-tm-secret-enclave
	# cp /tmp/tm-secret-enclave/tendermint_enclave.signed.so ./x/compute/internal/keeper
	SGX_MODE=SW $(MAKE) build-linux
	cp ./$(EXECUTE_ENCLAVE_PATH)/librust_cosmwasm_enclave.signed.so ./x/compute/internal/keeper
	GOMAXPROCS=8 SGX_MODE=SW SCRT_SGX_STORAGE='./' SKIP_LIGHT_CLIENT_VALIDATION=TRUE go test -count 1 -failfast -timeout 90m -v ./x/compute/internal/... $(GO_TEST_ARGS)

go-tests-hw: build-test-contracts bin-data
	# empty BUILD_PROFILE means debug mode which compiles faster
	# SGX_MODE=HW $(MAKE) build-tm-secret-enclave
	# cp /tmp/tm-secret-enclave/tendermint_enclave.signed.so ./x/compute/internal/keeper
	SGX_MODE=HW $(MAKE) build-linux
	cp ./$(EXECUTE_ENCLAVE_PATH)/librust_cosmwasm_enclave.signed.so ./x/compute/internal/keeper
	GOMAXPROCS=8 SGX_MODE=HW SCRT_SGX_STORAGE='./' SKIP_LIGHT_CLIENT_VALIDATION=TRUE go test -v ./x/compute/internal/... $(GO_TEST_ARGS)

# When running this more than once, after the first time you'll want to remove the contents of the `ffi-types`
# rule in the Makefile in `enclaves/execute`. This is to speed up the compilation time of tests and speed up the
# test debugging process in general.
.PHONY: enclave-tests
enclave-tests:
	$(MAKE) -C cosmwasm/enclaves/test run

build-all-test-contracts: build-test-contracts
	cd $(CW_CONTRACTS_V010_PATH)/gov && RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os $(CW_CONTRACTS_V010_PATH)/gov/target/wasm32-unknown-unknown/release/gov.wasm -o $(TEST_CONTRACT_PATH)/gov.wasm

	cd $(CW_CONTRACTS_V010_PATH)/dist && RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os $(CW_CONTRACTS_V010_PATH)/dist/target/wasm32-unknown-unknown/release/dist.wasm -o $(TEST_CONTRACT_PATH)/dist.wasm

	cd .$(CW_CONTRACTS_V010_PATH)/mint && RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os .$(CW_CONTRACTS_V010_PATH)/mint/target/wasm32-unknown-unknown/release/mint.wasm -o $(TEST_CONTRACT_PATH)/mint.wasm

	cd .$(CW_CONTRACTS_V010_PATH)/staking && RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os .$(CW_CONTRACTS_V010_PATH)/staking/target/wasm32-unknown-unknown/release/staking.wasm -o $(TEST_CONTRACT_PATH)/staking.wasm

	cd .$(CW_CONTRACTS_V010_PATH)/reflect && RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os .$(CW_CONTRACTS_V010_PATH)/reflect/target/wasm32-unknown-unknown/release/reflect.wasm -o $(TEST_CONTRACT_PATH)/reflect.wasm

	cd .$(CW_CONTRACTS_V010_PATH)/burner && RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os .$(CW_CONTRACTS_V010_PATH)/burner/target/wasm32-unknown-unknown/release/burner.wasm -o $(TEST_CONTRACT_PATH)/burner.wasm

	cd .$(CW_CONTRACTS_V010_PATH)/erc20 && RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os .$(CW_CONTRACTS_V010_PATH)/erc20/target/wasm32-unknown-unknown/release/cw_erc20.wasm -o $(TEST_CONTRACT_PATH)/erc20.wasm

	cd .$(CW_CONTRACTS_V010_PATH)/hackatom && RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os .$(CW_CONTRACTS_V010_PATH)/hackatom/target/wasm32-unknown-unknown/release/hackatom.wasm -o $(TEST_CONTRACT_PATH)/contract.wasm
	cat $(TEST_CONTRACT_PATH)/contract.wasm | gzip > $(TEST_CONTRACT_PATH)/contract.wasm.gzip

build-erc20-contract: build-test-contracts
	cd .$(CW_CONTRACTS_V010_PATH)/erc20 && RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os .$(CW_CONTRACTS_V010_PATH)/erc20/target/wasm32-unknown-unknown/release/cw_erc20.wasm -o ./erc20.wasm

bin-data: bin-data-sw bin-data-develop bin-data-production

bin-data-sw:
	cd ./x/registration/internal/types && go-bindata -o ias_bin_sw.go -pkg types -prefix "../../../../ias_keys/sw_dummy/" -tags "!hw" ../../../../ias_keys/sw_dummy/...

bin-data-develop:
	cd ./x/registration/internal/types && go-bindata -o ias_bin_dev.go -pkg types -prefix "../../../../ias_keys/develop/" -tags "develop,hw" ../../../../ias_keys/develop/...

bin-data-production:
	cd ./x/registration/internal/types && go-bindata -o ias_bin_prod.go -pkg types -prefix "../../../../ias_keys/production/" -tags "production,hw" ../../../../ias_keys/production/...

# Before running this you might need to do:
# 1. sudo docker login -u ABC -p XYZ
# 2. sudo docker buildx create --use
secret-contract-optimizer:
	sudo docker buildx build --platform=linux/amd64,linux/arm64/v8 -f deployment/dockerfiles/base-images/secret-contract-optimizer.Dockerfile -t enigmampc/secret-contract-optimizer:${TAG} --push .
	sudo docker buildx imagetools create -t enigmampc/secret-contract-optimizer:latest enigmampc/secret-contract-optimizer:${TAG}

aesm-image:
	docker build -f deployment/dockerfiles/aesm.Dockerfile -t enigmampc/aesm .

###############################################################################
###                         Swagger & Protobuf                              ###
###############################################################################

.PHONY: update-swagger-openapi-docs statik statik-install proto-swagger-openapi-gen

statik-install:
	@echo "Installing statik..."
	@go install github.com/rakyll/statik@v0.1.6

statik:
	statik -src=client/docs/static/ -dest=client/docs -f -m

proto-swagger-openapi-gen:
	cp go.mod /tmp/go.mod.bak
	cp go.sum /tmp/go.sum.bak
	@./scripts/protoc-swagger-openapi-gen.sh
	cp /tmp/go.mod.bak go.mod
	cp /tmp/go.sum.bak go.sum

# Example `CHAIN_VERSION=v1.4.0 make update-swagger-openapi-docs`
update-swagger-openapi-docs: statik-install proto-swagger-openapi-gen statik

protoVer=v0.2

proto-all: proto-lint proto-gen proto-swagger-openapi-gen

proto-gen:
	cp go.mod /tmp/go.mod.bak
	cp go.sum /tmp/go.sum.bak
	@echo "Generating Protobuf files"
	$(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace tendermintdev/sdk-proto-gen:$(protoVer) sh ./scripts/protocgen.sh
	cp /tmp/go.mod.bak go.mod
	cp /tmp/go.sum.bak go.sum
	go mod tidy

proto-lint:
	@$(DOCKER_BUF) lint --error-format=json

.PHONY: proto-all proto-gen proto-format proto-lint proto-check-breaking

.PHONY: check-hw
check-hw: build-linux
	$(MAKE) -C check-hw
