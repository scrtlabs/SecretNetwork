#!/usr/bin/env bash

# Install prerequisite packages
sudo apt-get update
if [[ $(lsb_release -rs) == "22.04" ]]; then
  sudo apt-get install -y --no-install-recommends g++ libtool autoconf clang-14
else
  sudo apt-get install -y --no-install-recommends g++ libtool autoconf clang
fi

# Clone Repo
git clone https://github.com/scrtlabs/SecretNetwork.git
cd SecretNetwork

# Install Rust
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
echo 'export PATH="$HOME/.cargo/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
rustup component add rust-src
rustup target add wasm32-unknown-unknown

# Install Go
GO_VERSION=1.21.1
wget -q https://golang.org/dl/go$GO_VERSION.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go$GO_VERSION.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
if [[ ! -d "$HOME/go" ]]; then
    mkdir $HOME/go
fi
echo "export GOPATH=\$HOME/go" >> ~/.bashrc
echo "export PATH=\$PATH:\$GOPATH/bin" >> ~/.bashrc
source ~/.bashrc

# Install gobindata
sudo apt-get install -y go-bindata

# Install SGX
chmod +x ./scripts/install-sgx.sh
sudo ./scripts/install-sgx.sh true true true false

# Install Xargo
cargo install xargo --version 0.3.25

# Install submodules
git submodule init
git submodule update --remote

# Build from Source
make build-linux
