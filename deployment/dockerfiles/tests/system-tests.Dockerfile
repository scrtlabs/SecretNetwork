# This dockerfile contains tests that only test the compute module, using a single node. They do not execute tests
# on multiple nodes, nor do they require a full network or interfaces with user libraries, network latency, etc.

FROM ghcr.io/scrtlabs/compile-contracts:1.6.0

RUN mkdir -p /opt/secret/.sgx_secrets

WORKDIR secretnetwork

COPY cosmwasm cosmwasm
COPY Makefile .
COPY x x

RUN . /root/.cargo/env && make build-test-contract

# Add source files
COPY go-cosmwasm go-cosmwasm
# This is due to some esoteric docker bug with the underlying filesystem, so until I figure out a better way, this should be a workaround

COPY types types
RUN true
COPY app app
COPY go.mod .
COPY go.sum .
COPY cmd cmd
RUN true
COPY client client
COPY ias_keys ias_keys

COPY spid.txt ias_keys/develop/spid.txt
COPY spid.txt ias_keys/sw_dummy/spid.txt
COPY spid.txt ias_keys/production/spid.txt

COPY api_key.txt ias_keys/develop/api_key.txt
COPY api_key.txt ias_keys/sw_dummy/api_key.txt
COPY api_key.txt ias_keys/production/api_key.txt

COPY deployment/ci/go-tests.sh .
COPY deployment/ci/go-tests-bench.sh .
#COPY path/to/tests.js
#RUN cd path/to/tests && npm i

RUN chmod +x go-tests.sh
RUN chmod +x go-tests-bench.sh

COPY --from=azcr.io/enigmampc/ci-base-image-local /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/target/release/libgo_cosmwasm.so ./go-cosmwasm/api/libgo_cosmwasm.so
COPY --from=azcr.io/enigmampc/ci-base-image-local /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_enclave.signed.so x/compute/internal/keeper/librust_cosmwasm_enclave.signed.so

ENTRYPOINT ["/bin/bash", "go-tests.sh"]
