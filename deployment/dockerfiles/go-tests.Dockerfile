FROM ghcr.io/scrtlabs/compile-contracts:1.5.0

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

COPY deployment/ci/go-tests.sh .

RUN chmod +x go-tests.sh

COPY --from=rust-go-base-image /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/target/release/libgo_cosmwasm.so ./go-cosmwasm/api/libgo_cosmwasm.so
COPY --from=rust-go-base-image /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_enclave.signed.so x/compute/internal/keeper/librust_cosmwasm_enclave.signed.so

ENTRYPOINT ["/bin/bash", "go-tests.sh"]
