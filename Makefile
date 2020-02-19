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

ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=Enigmachain \
	-X github.com/cosmos/cosmos-sdk/version.ServerName=enigmad \
	-X github.com/cosmos/cosmos-sdk/version.ClientName=enigmacli \
	-X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
	-X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
	-X "github.com/cosmos/cosmos-sdk/version.BuildTags=$(build_tags)"

ifeq ($(WITH_CLEVELDB),yes)
  ldflags += -X github.com/cosmos/cosmos-sdk/types.DBBackend=cleveldb
endif
ldflags += $(LDFLAGS)
ldflags := $(strip $(ldflags))

BUILD_FLAGS := -tags "$(build_tags)" -ldflags '$(ldflags)'

all: build_all

go.sum: go.mod
	@echo "--> Ensure dependencies have not been modified"
	GO111MODULE=on go mod verify

xgo_build_enigmad: go.sum
	xgo --go latest --targets $(XGO_TARGET) $(BUILD_FLAGS) github.com/enigmampc/enigmachain/cmd/enigmad

xgo_build_enigmacli: go.sum
	xgo --go latest --targets $(XGO_TARGET) $(BUILD_FLAGS) github.com/enigmampc/enigmachain/cmd/enigmacli

build_local:
	go build -mod=readonly $(BUILD_FLAGS) ./cmd/enigmad
	go build -mod=readonly $(BUILD_FLAGS) ./cmd/enigmacli

build_linux: build_local

build_windows:
	$(MAKE) xgo_build_enigmad XGO_TARGET=windows/amd64
	$(MAKE) xgo_build_enigmacli XGO_TARGET=windows/amd64

build_macos:
	$(MAKE) xgo_build_enigmad XGO_TARGET=darwin/amd64
	$(MAKE) xgo_build_enigmacli XGO_TARGET=darwin/amd64

build_all: build_linux build_windows build_macos

deb: build_local
    ifneq ($(UNAME_S),Linux)
		exit 1
    endif
	rm -rf /tmp/enigmachain
	mkdir -p /tmp/enigmachain/deb/bin
	mv -f ./enigmacli /tmp/enigmachain/deb/bin/enigmacli
	mv -f ./enigmad /tmp/enigmachain/deb/bin/enigmad
	chmod +x /tmp/enigmachain/deb/bin/enigmad /tmp/enigmachain/deb/bin/enigmacli
	mkdir -p /tmp/enigmachain/deb/DEBIAN
	cp ./packaging_ubuntu/control /tmp/enigmachain/deb/DEBIAN/control
	echo "Version: 0.0.1" >> /tmp/enigmachain/deb/DEBIAN/control
	echo "" >> /tmp/enigmachain/deb/DEBIAN/control
	cp ./packaging_ubuntu/postinst /tmp/enigmachain/deb/DEBIAN/postinst
	chmod 755 /tmp/enigmachain/deb/DEBIAN/postinst
	cp ./packaging_ubuntu/postrm /tmp/enigmachain/deb/DEBIAN/postrm
	chmod 755 /tmp/enigmachain/deb/DEBIAN/postrm
	dpkg-deb --build /tmp/enigmachain/deb/ .
	-rm -rf /tmp/enigmachain

rename_for_release:
	-rename "s/windows-4.0-amd64/v${VERSION}-win64/" *.exe
	-rename "s/darwin-10.6-amd64/v${VERSION}-osx64/" *darwin*

sign_for_release: rename_for_release
	sha256sum enigmachain*.deb enigmad-* enigmacli-* > SHA256SUMS
	gpg2 -u 91831DE812C6415123AFAA7B420BF1CB005FBCE6 --digest-algo sha256 --clearsign --yes SHA256SUMS
	rm -f SHA256SUMS
	
release: sign_for_release
	rm -rf ./release/
	mkdir -p ./release/
	cp enigmachain_*.deb ./release/ 
	cp enigmacli-* ./release/ 
	cp enigmad-* ./release/
	cp SHA256SUMS.asc ./release/

clean:
	-rm -rf /tmp/enigmachain
	-rm -rf enigmacli-*
	-rm -rf enigmad-*
	-rm -rf enigmachain-*.deb
	-rm -f ./SHA256SUMS*
