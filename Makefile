PACKAGES=$(shell go list ./... | grep -v '/simulation')
VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')
LEDGER_ENABLED ?= true
BINDIR ?= $(GOPATH)/bin

build_tags = 
ifeq ($(LEDGER_ENABLED),true)
  ifeq ($(OS),Windows_NT)
    GCCEXE = $(shell where gcc.exe 2> NUL)
    ifeq ($(GCCEXE),)
      $(error gcc.exe not installed for ledger support, please install or set LEDGER_ENABLED=false)
    else
      build_tags += ledger
    endif
  else
    UNAME_S = $(shell uname -s)
    ifeq ($(UNAME_S),OpenBSD)
      $(warning OpenBSD detected, disabling ledger support (https://github.com/cosmos/cosmos-sdk/issues/1988))
    else
      GCC = $(shell command -v gcc 2> /dev/null)
      ifeq ($(GCC),)
        $(error gcc not installed for ledger support, please install or set LEDGER_ENABLED=false)
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

ldflags = -X github.com/enigmampc/cosmos-sdk/version.Name=SecretNetwork \
	-X github.com/enigmampc/cosmos-sdk/version.ServerName=secretd \
	-X github.com/enigmampc/cosmos-sdk/version.ClientName=secretcli \
	-X github.com/enigmampc/cosmos-sdk/version.Version=$(VERSION) \
	-X github.com/enigmampc/cosmos-sdk/version.Commit=$(COMMIT) \
	-X "github.com/enigmampc/cosmos-sdk/version.BuildTags=$(build_tags)"

ifeq ($(WITH_CLEVELDB),yes)
  ldflags += -X github.com/enigmampc/cosmos-sdk/types.DBBackend=cleveldb
endif
ldflags += $(LDFLAGS)
ldflags := $(strip $(ldflags))

BUILD_FLAGS := -tags "$(build_tags)" -ldflags '$(ldflags) -s -w'

all: build_all

go.sum: go.mod
	@echo "--> Ensure dependencies have not been modified"
	GO111MODULE=on go mod verify

xgo_build_secretd: go.sum
	xgo --go latest --targets $(XGO_TARGET) $(BUILD_FLAGS) github.com/enigmampc/SecretNetwork/cmd/secretd

xgo_build_secretcli: go.sum
	xgo --go latest --targets $(XGO_TARGET) $(BUILD_FLAGS) github.com/enigmampc/SecretNetwork/cmd/secretcli

build_local_no_rust:
	@ #this pulls out ELF symbols, 80% size reduction!
	go build -mod=readonly $(BUILD_FLAGS) ./cmd/secretd
	go build -mod=readonly $(BUILD_FLAGS) ./cmd/secretcli

build_local:
	# cd go-cosmwasm && rustup run nightly cargo build --release --features backtraces
	# cp go-cosmwasm/target/release/libgo_cosmwasm.so go-cosmwasm/api
	@ #this pulls out ELF symbols, 80% size reduction!
	go build -mod=readonly $(BUILD_FLAGS) ./cmd/secretd
	go build -mod=readonly $(BUILD_FLAGS) ./cmd/secretcli

build_linux: build_local

build_windows:
	$(MAKE) xgo_build_secretd XGO_TARGET=windows/amd64
	$(MAKE) xgo_build_secretcli XGO_TARGET=windows/amd64

build_macos:
	$(MAKE) xgo_build_secretd XGO_TARGET=darwin/amd64
	$(MAKE) xgo_build_secretcli XGO_TARGET=darwin/amd64

build_all: build_linux build_windows build_macos

deb: build_local
    ifneq ($(UNAME_S),Linux)
		exit 1
    endif
	rm -rf /tmp/SecretNetwork
	
	mkdir -p /tmp/SecretNetwork/deb/bin
	mv -f ./secretcli /tmp/SecretNetwork/deb/bin/secretcli
	mv -f ./secretd /tmp/SecretNetwork/deb/bin/secretd
	chmod +x /tmp/SecretNetwork/deb/bin/secretd /tmp/SecretNetwork/deb/bin/secretcli
	
	# mkdir -p /tmp/SecretNetwork/deb/usr/lib
	# mv -f ./go-cosmwasm/api/libgo_cosmwasm.so /tmp/SecretNetwork/deb/usr/lib/libgo_cosmwasm.so
	# chmod +x /tmp/SecretNetwork/deb/usr/lib/libgo_cosmwasm.so

	mkdir -p /tmp/SecretNetwork/deb/DEBIAN
	cp ./packaging_ubuntu/control /tmp/SecretNetwork/deb/DEBIAN/control
	printf "Version: " >> /tmp/SecretNetwork/deb/DEBIAN/control
	git tag | grep -P '^v' | tail -1 | tr -d v >> /tmp/SecretNetwork/deb/DEBIAN/control
	echo "" >> /tmp/SecretNetwork/deb/DEBIAN/control
	cp ./packaging_ubuntu/postinst /tmp/SecretNetwork/deb/DEBIAN/postinst
	chmod 755 /tmp/SecretNetwork/deb/DEBIAN/postinst
	cp ./packaging_ubuntu/postrm /tmp/SecretNetwork/deb/DEBIAN/postrm
	chmod 755 /tmp/SecretNetwork/deb/DEBIAN/postrm
	dpkg-deb --build /tmp/SecretNetwork/deb/ .
	-rm -rf /tmp/SecretNetwork

rename_for_release:
	-rename "s/windows-4.0-amd64/v${VERSION}-win64/" *.exe
	-rename "s/darwin-10.6-amd64/v${VERSION}-osx64/" *darwin*

sign_for_release: rename_for_release
	sha256sum secretnetwork*.deb > SHA256SUMS
	-sha256sum secretd-* secretcli-* >> SHA256SUMS
	gpg -u 91831DE812C6415123AFAA7B420BF1CB005FBCE6 --digest-algo sha256 --clearsign --yes SHA256SUMS
	rm -f SHA256SUMS
	
release: sign_for_release
	rm -rf ./release/
	mkdir -p ./release/
	cp secretnetwork_*.deb ./release/
	cp secretcli-* ./release/
	cp secretd-* ./release/
	cp SHA256SUMS.asc ./release/

clean:
	-rm -rf /tmp/SecretNetwork
	-rm -f ./secretcli-*
	-rm -f ./secretd-*
	-rm -f ./secretnetwork*.deb
	-rm -f ./SHA256SUMS*
