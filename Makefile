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
		chmod +x /tmp/enigmachain/deb/bin/engd /tmp/enigmachain/deb/bin/engcli
		mkdir -p /tmp/enigmachain/deb/DEBIAN
		cp ./packaging/control /tmp/enigmachain/deb/DEBIAN/control
		echo "Version: 0.0.1" >> /tmp/enigmachain/deb/DEBIAN/control
		echo "" >> /tmp/enigmachain/deb/DEBIAN/control
		cp ./packaging/postinst /tmp/enigmachain/deb/DEBIAN/postinst
		chmod 755 /tmp/enigmachain/deb/DEBIAN/postinst
		cp ./packaging/postrm /tmp/enigmachain/deb/DEBIAN/postrm
		chmod 755 /tmp/enigmachain/deb/DEBIAN/postrm
		dpkg-deb --build /tmp/enigmachain/deb/ .
		rm -rf /tmp/enigmachain
