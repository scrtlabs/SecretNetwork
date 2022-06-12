#!/usr/bin/env bash
set -euv

SGX_MODE=HW GENESIS_PATH=~/.secretd/config/genesis.json MANTLEMINT_HOME=~/.secretd CHAIN_ID=secret-4 RPC_ENDPOINTS=http://20.104.20.173:26657,http://20.104.20.173:26657 MANTLEMINT_DB=mantlemint INDEXER_DB=indexer DISABLE_SYNC=false WS_ENDPOINTS=ws://20.104.20.173:26657/websocket,ws://20.104.20.173:26657/websocket SCRT_SGX_STORAGE="/opt/secret/.sgx_secrets" SCRT_ENCLAVE_DIR="/usr/lib" RUST_BACKTRACE=1 ./mantlemint