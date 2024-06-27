![Secret Network](sn-logo.png)

<div align="center">
  
[![version](https://img.shields.io/badge/version-1.12.1-blue)](https://github.com/scrtlabs/SecretNetwork/releases/tag/v1.12.1)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v2.0%20adopted-ff69b4.svg)](CODE_OF_CONDUCT.md)
<a href="https://twitter.com/intent/follow?screen_name=SecretNetwork">
<img src="https://img.shields.io/twitter/follow/SecretNetwork?style=social&logo=twitter"
alt="Follow"></a>

 </div>

Secret Network offers scalable permissionless smart contracts with a private by default design— bringing novel use cases to blockchain not feasible on public systems. Secret Network enables users to take back ownership over their private (financial) information and for them to share this information with whom they trust. Secret Network was the first protocol to provide private smart contracts on mainnet, live since September 2020. Secret Network is Built with the Cosmos Software Development Kit (SDK) bringing Interoperable privacy to the entire Cosmos ecosystem. Secret Network uses a combination of the Intel SGX (Software Guard Extension) Trusted Execution Environment technology, several encryption schemes and key management to bring privacy by default to blockchain users. Secret Contracts are an implementation of the Rust based smart contract compiling toolkit CosmWasm, adding private metadata possibilities. Secret Network is powered by the Native public coin SCRT which is used for fees, Proof Of Stake security and Governance. With more than 20+ Dapps, 100+ full time builders and a strong grassroots community Secret Network aims to bring privacy to the masses.


# Setting up Environment

## Prebuilt Environment

### Gitpod

Click the button below to start a new development environment:

[![Open in Gitpod](https://gitpod.io/button/open-in-gitpod.svg)](https://gitpod.io/#https://github.com/scrtlabs/SecretNetwork)

### VSCode Docker Environment

1. Install <vs code remote> extension

2. Clone this repository into a new dev container

### Docker Dev Environments

1. From Docker Desktop, create a new Dev Environment from the prebuilt image - `ghcr.io/scrtlabs/secretnetwork-dev:latest`
2. Connect with VSCode, or use the container directly
3. Make sure the code is updated by using `get fetch` and `git pull`

## Manual Set up

*You can find everything below in a handy script that you can copy and run from [here](https://github.com/scrtlabs/SecretNetwork/blob/master/scripts/install-everything.sh)*

### Install prerequisite packages

```
sudo apt-get install -y --no-install-recommends g++ libtool automake autoconf clang
```

#### Optinal packages to install ((needed for local testing))
```
sudo apt install -y  jq nodejs npm
```
### Consider installing nvm (https://github.com/nvm-sh/nvm)
```
wget -qO- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash
nvm install 22
```

Note: lcp - Simple proxy to bypass CORS issues. It solve the issue
```
No 'Access-Control-Allow-Origin' header is present on the requested resource. Origin 'http://localhost:3000' is therefore not allowed access. If an opaque response serves your needs, set the request's mode to 'no-cors' to fetch the resource with CORS disable
```
You may need to isntall __lcp__ CORS bypass proxy if you plan on running your network of nodes locally:
```
sudo npm install -g local-cors-proxy
```


#### Ubuntu 22+

The build depends on libssl1.1. Install using:

```bash
wget https://debian.mirror.ac.za/debian/pool/main/o/openssl/libssl1.1_1.1.1w-0%2Bdeb11u1_amd64.deb
```
```bash
sudo dpkg -i libssl1.1_1.1.1w-0+deb11u1_amd64.deb
```
### Clone Repo

Clone this repo to your favorite working directory. e.g. ~/SecretNetwork
```
git clone git@github.com:scrtlabs/SecretNetwork.git ~/SecretNetwork
cd ~SecretNetwork
git checkout <work-branch>
```

### Install Rust

Install rust from [https://rustup.rs/](https://rustup.rs/). 

```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

Then, while in your __SecretNetwork__ add the _rust-src_ componenti and _wasm32-unknown-unknown_ target. This will also install the version of rust that is defined by the workspace (in `rust-toolchain`) - 
```
rustup component add rust-src
```

To run tests you'll need to add the wasm32 target - 
```
rustup target add wasm32-unknown-unknown
```

### Install Go (v1.18+)

Install go from [https://go.dev/doc/install](https://go.dev/doc/install)
Any version up to 1.22.4 should work.

#### Install gobindata

```
sudo apt install go-bindata
```

### Install SGX

To compile the code and run tests, you'll need to install the SGX SDK and PSW. To run in simulation (or software) modes of SGX you do _not_ need to install the SGX driver. 
For a simple install, run the [install-sgx.sh](./scripts/install-sgx.sh) script in the following way:

```bash
chmod +x ./scripts/install-sgx.sh
sudo ./scripts/install-sgx.sh true true true false
```
After SGX installs successfully, please add the following line to your _.bashrc_ file
```
SGX_HOME=/opt/intel/sgxsdk # or the directory where installed it if defferent
. ${SGX_HOME}/environment
```
and
```
source ~/.bashrc
```
to bring into your shell the important paths from SGX that you will need when building _SecretNetwork_

It may be worth mentioning that your LIBRARY_PATH should also point to SGX_SDK/sdk_libs

If you plan to run in **SGX_MODE=SW** indicate it by exporting SGX_MODE
```
export SGX_MODE=SW
```
There is one more important step you need to do:
```
ln -s $SGX_HOME/lib64/libsgx_epid.so $SGX_HOME/sdk_libs/libsgx_epid.so
```
as this library will be required during building.

Note: If you are using WSL you'll need to use the 5.15 kernel which you can find how to do [here](https://github.com/scrtlabs/SecretNetwork/blob/master/docs/SGX%20on%20WSL%20(SW).md), otherwise you'll have to run anything SGX related only in docker
  
### Install Xargo

We need a very specific version of xargo for everything to compile happily together

```
cargo install xargo --version 0.3.25
```
# Install submodules

We use `incubator-teaclave-sgx-sdk` as a submodule. To compile the code, you must first sync this submodule

```
git submodule init
git submodule update --remote
```

# Build from Source

Use `make build-linux` to build the entire codebase. This will build both the Rust (enclave & contract engine) and the Go (blockchain) code.

To build just the rust code, you can use `make build-linux`, while to build just the Go code, there is the aptly named `make build_local_no_rust`.

Tip:
For a production build the enclave must be copied from the most recent release. 
This is due to non-reproducible builds, and the fact that enclaves must be signed with a specific key to be accepted on mainnet. 
Still, the non-enclave code can be modified and ran on mainnet as long as there are no consensus-breaking changes


# Running Something

## Run tests

To build run all tests, use `make go-tests`

## Start local network

Before you run your dev node, you need to point to the shared libraries needed by __secretd__:
```
export SCRT_ENCLAVE_DIR=~/SecretNetwork/go-cosmwasm
```
In order to run __secretd__ you need to have built _librust_cosmwasm_enclave.signed.so_ and _tendermint_enclave.so_.
The latter can be built by cloning:
```
git clone git@github.com:scrtlabs/tm-secret-enclave.git ~/tm-secret-enclave
```
This repo also uses submodules:
```
cd tm-secret-enclave
git submodule init
git submodule update --remote
``
and build _tendermint_enclave.so_:
```
make build-rust
```
You may want to copy _tendermint_enclave.so_ to ~/SecretNetwork/go-cosmwasm

Run `./scripts/start-node.sh`

# Documentation

For the latest documentation, check out [https://docs.scrt.network](https://docs.scrt.network)

# Community

- Homepage: [https://scrt.network](https://scrt.network)
- Blog: [https://blog.scrt.network](https://blog.scrt.network)
- Forum: [https://forum.scrt.network](https://forum.scrt.network)
- Docs: [https://docs.scrt.network](https://docs.scrt.network)
- Discord: [https://chat.scrt.network](https://chat.scrt.network)
- Twitter: [https://twitter.com/SecretNetwork](https://twitter.com/SecretNetwork)
- Community Telegram Channel: [https://t.me/SCRTnetwork](https://t.me/SCRTnetwork)
- Community Secret Nodes Telegram: [https://t.me/secretnodes](https://t.me/secretnodes)
