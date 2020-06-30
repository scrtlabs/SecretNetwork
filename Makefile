PACKAGES=$(shell go list ./... | grep -v '/simulation')
VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')
LEDGER_ENABLED ?= true
BINDIR ?= $(GOPATH)/bin
BUILD_PROFILE ?= release

SGX_MODE ?= HW
BRANCH ?= develop
DEBUG ?= 0
DOCKER_TAG ?= latest

ifeq ($(SGX_MODE), HW)
	ext := hw
else ifeq ($(SGX_MODE), SW)
	ext := sw
else
$(error SGX_MODE must be either HW or SW)
endif

SGX_MODE ?= HW
BRANCH ?= develop
DEBUG ?= 0
DOCKER_TAG ?= latest

ifeq ($(SGX_MODE), HW)
	ext := hw
else ifeq ($(SGX_MODE), SW)
	ext := sw
else
$(error SGX_MODE must be either HW or SW)
endif

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

ifeq ($(WITH_CLEVELDB),yes)
  build_tags += gcc
endif
build_tags += $(BUILD_TAGS)
build_tags := $(strip $(build_tags))

whitespace :=
whitespace += $(whitespace)
comma := ,
build_tags_comma_sep := $(subst $(whitespace),$(comma),$(build_tags))

ldflags = -X github.com/enigmampc/cosmos-sdk/version.Name=EnigmaBlockchain \
	-X github.com/enigmampc/cosmos-sdk/version.ServerName=secretd \
	-X github.com/enigmampc/cosmos-sdk/version.ClientName=secretcli \
	-X github.com/enigmampc/cosmos-sdk/version.Version=$(VERSION) \
	-X github.com/enigmampc/cosmos-sdk/version.Commit=$(COMMIT) \
	-X "github.com/enigmampc/cosmos-sdk/version.BuildTags=$(build_tags)"

ifeq ($(WITH_CLEVELDB),yes)
  ldflags += -X github.com/enigmampc/cosmos-sdk/types.DBBackend=cleveldb
endif
ldflags += -s -w
ldflags += $(LDFLAGS)
ldflags := $(strip $(ldflags))

BUILD_FLAGS := -tags "$(build_tags)" -ldflags '$(ldflags)'

all: build_all

vendor:
	cargo vendor third_party/vendor --manifest-path third_party/build/Cargo.toml

go.sum: go.mod
	@echo "--> Ensure dependencies have not been modified"
	GO111MODULE=on go mod verify

xgo_build_secretd: go.sum
	xgo --go latest --targets $(XGO_TARGET) $(BUILD_FLAGS) github.com/enigmampc/SecretNetwork/cmd/secretd

xgo_build_secretcli: go.sum
	xgo --go latest --targets $(XGO_TARGET) $(BUILD_FLAGS) github.com/enigmampc/SecretNetwork/cmd/secretcli

build_local_no_rust:
	cp go-cosmwasm/target/release/libgo_cosmwasm.so go-cosmwasm/api
#   this pulls out ELF symbols, 80% size reduction!
	go build -mod=readonly $(BUILD_FLAGS) ./cmd/secretd
	go build -mod=readonly $(BUILD_FLAGS) ./cmd/secretcli

build-linux: vendor
	BUILD_PROFILE=$(BUILD_PROFILE) $(MAKE) -C go-cosmwasm build-rust
	cp go-cosmwasm/target/$(BUILD_PROFILE)/libgo_cosmwasm.so go-cosmwasm/api
#   this pulls out ELF symbols, 80% size reduction!
	go build -mod=readonly $(BUILD_FLAGS) ./cmd/secretd
	go build -mod=readonly $(BUILD_FLAGS) ./cmd/secretcli

build_windows:
	# CLI only 
	$(MAKE) xgo_build_secretcli XGO_TARGET=windows/amd64

build_macos:
	# CLI only 
	$(MAKE) xgo_build_secretcli XGO_TARGET=darwin/amd64

build_arm_linux:
	# CLI only 
	$(MAKE) xgo_build_secretcli XGO_TARGET=linux/arm64

build_all: build-linux build_windows build_macos build_arm_linux

deb: build-linux
    ifneq ($(UNAME_S),Linux)
		exit 1
    endif
	rm -rf /tmp/EnigmaBlockchain

	mkdir -p /tmp/EnigmaBlockchain/deb/usr/local/bin
	mv -f ./secretcli /tmp/EnigmaBlockchain/deb/usr/local/bin/secretcli
	mv -f ./secretd /tmp/EnigmaBlockchain/deb/usr/local/bin/secretd
	chmod +x /tmp/EnigmaBlockchain/deb/usr/local/bin/secretd /tmp/EnigmaBlockchain/deb/usr/local/bin/secretcli

	mkdir -p /tmp/EnigmaBlockchain/deb/usr/local/lib
	cp -f ./go-cosmwasm/api/libgo_cosmwasm.so ./go-cosmwasm/librust_cosmwasm_enclave.signed.so /tmp/EnigmaBlockchain/deb/usr/local/lib/
	chmod +x /tmp/EnigmaBlockchain/deb/usr/local/lib/lib*.so

	mkdir -p /tmp/EnigmaBlockchain/deb/DEBIAN
	cp ./packaging_ubuntu/control /tmp/EnigmaBlockchain/deb/DEBIAN/control
	printf "Version: " >> /tmp/EnigmaBlockchain/deb/DEBIAN/control
	git describe --tags | tr -d v >> /tmp/EnigmaBlockchain/deb/DEBIAN/control
	echo "" >> /tmp/EnigmaBlockchain/deb/DEBIAN/control
	cp ./packaging_ubuntu/postinst /tmp/EnigmaBlockchain/deb/DEBIAN/postinst
	chmod 755 /tmp/EnigmaBlockchain/deb/DEBIAN/postinst
	cp ./packaging_ubuntu/postrm /tmp/EnigmaBlockchain/deb/DEBIAN/postrm
	chmod 755 /tmp/EnigmaBlockchain/deb/DEBIAN/postrm
	dpkg-deb --build /tmp/EnigmaBlockchain/deb/ .
	-rm -rf /tmp/EnigmaBlockchain

rename_for_release:
	-rename "s/windows-4.0-amd64/v${VERSION}-win64/" *.exe
	-rename "s/darwin-10.6-amd64/v${VERSION}-osx64/" *darwin*

sign_for_release: rename_for_release
	sha256sum enigma-blockchain*.deb > SHA256SUMS
	-sha256sum secretd-* secretcli-* >> SHA256SUMS
	gpg -u 91831DE812C6415123AFAA7B420BF1CB005FBCE6 --digest-algo sha256 --clearsign --yes SHA256SUMS
	rm -f SHA256SUMS

release: sign_for_release
	rm -rf ./release/
	mkdir -p ./release/
	cp enigma-blockchain_*.deb ./release/
	cp secretcli-* ./release/
	cp secretd-* ./release/
	cp SHA256SUMS.asc ./release/

clean:
	-rm -rf /tmp/EnigmaBlockchain
	-rm -f ./secretcli*
	-rm -f ./secretd*
	-rm -f ./librust_cosmwasm_enclave.signed.so 
	-rm -f ./x/compute/internal/keeper/librust_cosmwasm_enclave.signed.so 
	-rm -f ./go-cosmwasm/api/libgo_cosmwasm.so
	-rm -f ./enigma-blockchain*.deb
	-rm -f ./SHA256SUMS*
	-rm -rf ./third_party/vendor/
	-rm -rf ./.sgx_secrets/*
	-rm -rf ./x/compute/internal/keeper/.sgx_secrets/*
	-rm -rf ./x/compute/internal/keeper/*.der
	-rm -rf ./*.der
	-rm -rf ./x/compute/internal/keeper/*.so
	$(MAKE) -C go-cosmwasm clean-all
	$(MAKE) -C cosmwasm/packages/wasmi-runtime clean
# docker build --build-arg SGX_MODE=HW --build-arg SECRET_NODE_TYPE=NODE -f Dockerfile.testnet -t cashmaney/secret-network-node:azuretestnet .
build-azure:
	docker build -f Dockerfile.azure -t cashmaney/secret-network-node:azuretestnet .

build-testnet:
	docker build --build-arg SGX_MODE=HW --build-arg SECRET_NODE_TYPE=BOOTSTRAP -f Dockerfile.testnet -t cashmaney/secret-network-bootstrap:testnet  .
	docker build --build-arg SGX_MODE=HW --build-arg SECRET_NODE_TYPE=NODE -f Dockerfile.testnet -t cashmaney/secret-network-node:testnet .

docker_bootstrap:
	docker build --build-arg SGX_MODE=${SGX_MODE} --build-arg SECRET_NODE_TYPE=BOOTSTRAP -t cashmaney/secret-network-bootstrap-${ext}:${DOCKER_TAG} .

docker_node:
	docker build --build-arg SECRET_NODE_TYPE=NODE -t enigmampc/secret_node .

	docker build --build-arg SGX_MODE=${SGX_MODE} --build-arg SECRET_NODE_TYPE=NODE -t cashmaney/secret-network-node-${ext}:${DOCKER_TAG} .
# while developing:
build-enclave:
	$(MAKE) -C cosmwasm/packages/wasmi-runtime

# while developing:
check-enclave:
	$(MAKE) -C cosmwasm/packages/wasmi-runtime check

# while developing:
clean-enclave:
	$(MAKE) -C cosmwasm/packages/wasmi-runtime clean

sanity-test:
	SGX_MODE=SW $(MAKE) build-linux
	cp ./cosmwasm/packages/wasmi-runtime/librust_cosmwasm_enclave.signed.so .
	SGX_MODE=SW ./cosmwasm/testing/sanity-test.sh

sanity-test-hw:
	$(MAKE) build-linux
	cp ./cosmwasm/packages/wasmi-runtime/librust_cosmwasm_enclave.signed.so .
	./cosmwasm/testing/sanity-test.sh

callback-sanity-test:
	SGX_MODE=SW $(MAKE) build-linux
	cp ./cosmwasm/packages/wasmi-runtime/librust_cosmwasm_enclave.signed.so .
	SGX_MODE=SW ./cosmwasm/testing/callback-test.sh

build-test-contract:
	# echo "" | sudo add-apt-repository ppa:hnakamur/binaryen
	# sudo apt update
	# sudo apt install -y binaryen
	$(MAKE) -C ./x/compute/internal/keeper/testdata/test-contract

go-tests: build-test-contract
	# empty BUILD_PROFILE means debug mode which compiles faster
	SGX_MODE=SW $(MAKE) build-linux
	cp ./cosmwasm/packages/wasmi-runtime/librust_cosmwasm_enclave.signed.so ./x/compute/internal/keeper
	mkdir -p ./x/compute/internal/keeper/.sgx_secrets
	SGX_MODE=SW go test -p 1 -v ./x/compute/internal/...

build-cosmwasm-test-contracts:
	# echo "" | sudo add-apt-repository ppa:hnakamur/binaryen
	# sudo apt update
	# sudo apt install -y binaryen
	cd ./cosmwasm/contracts/staking && RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os ./cosmwasm/contracts/staking/target/wasm32-unknown-unknown/release/staking.wasm -o ./x/compute/internal/keeper/testdata/staking.wasm

	cd ./cosmwasm/contracts/reflect && RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os ./cosmwasm/contracts/reflect/target/wasm32-unknown-unknown/release/reflect.wasm -o ./x/compute/internal/keeper/testdata/reflect.wasm

	cd ./cosmwasm/contracts/burner && RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os ./cosmwasm/contracts/burner/target/wasm32-unknown-unknown/release/burner.wasm -o ./x/compute/internal/keeper/testdata/burner.wasm

	cd ./cosmwasm/contracts/erc20 && RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os ./cosmwasm/contracts/erc20/target/wasm32-unknown-unknown/release/erc20.wasm -o ./x/compute/internal/keeper/testdata/erc20.wasm

	cd ./cosmwasm/contracts/hackatom && RUSTFLAGS='-C link-arg=-s' cargo build --release --target wasm32-unknown-unknown --locked
	wasm-opt -Os ./cosmwasm/contracts/hackatom/target/wasm32-unknown-unknown/release/hackatom.wasm -o ./x/compute/internal/keeper/testdata/contract.wasm
	cat ./x/compute/internal/keeper/testdata/contract.wasm | gzip > ./x/compute/internal/keeper/testdata/contract.wasm.gzip