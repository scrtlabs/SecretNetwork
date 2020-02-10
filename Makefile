PACKAGES=$(shell go list ./... | grep -v '/simulation')

VERSION := $(shell echo $(shell git describe --tags) | sed 's/^v//')
COMMIT := $(shell git log -1 --format='%H')

ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=Enigmachain \
	-X github.com/cosmos/cosmos-sdk/version.ServerName=engd \
	-X github.com/cosmos/cosmos-sdk/version.ClientName=engcli \
	-X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
	-X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) 

BUILD_FLAGS := -ldflags '$(ldflags)'

include Makefile.ledger
all: install

install: go.sum
		go install -mod=readonly $(BUILD_FLAGS) ./cmd/engd
		go install -mod=readonly $(BUILD_FLAGS) ./cmd/engcli

go.sum: go.mod
		@echo "--> Ensure dependencies have not been modified"
		GO111MODULE=on go mod verify

test:
	@go test -mod=readonly $(PACKAGES)

deb: install
		rm -rf /tmp/enigmachain
		mkdir -p /tmp/enigmachain/deb/bin
		cp "$(GOPATH)/bin/engcli" /tmp/enigmachain/deb/bin
		cp "$(GOPATH)/bin/engd" /tmp/enigmachain/deb/bin
		mkdir -p /tmp/enigmachain/deb/DEBIAN
		echo "Package: enigmachain" > /tmp/enigmachain/deb/DEBIAN/control
		echo "Version: 0.0.1" >> /tmp/enigmachain/deb/DEBIAN/control
		echo "Priority: optional" >> /tmp/enigmachain/deb/DEBIAN/control
		echo "Architecture: amd64" >> /tmp/enigmachain/deb/DEBIAN/control
		echo "Homepage: https://github.com/enigmampc/Enigmachain" >> /tmp/enigmachain/deb/DEBIAN/control
		echo "Maintainer: https://github.com/enigmampc" >> /tmp/enigmachain/deb/DEBIAN/control
		echo "Installed-Size: $(ls -l --block-size=KB /tmp/enigmachain/deb/bin/eng* | tr -d 'kB' | awk '{sum+=$5} END{print sum}')" >> /tmp/enigmachain/deb/DEBIAN/control
		echo "Description: The Enigma blockchain" >> /tmp/enigmachain/deb/DEBIAN/control
		dpkg-deb --build /tmp/enigmachain/deb/ .