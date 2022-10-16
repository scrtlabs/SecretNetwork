ARG SCRT_BASE_IMAGE_ENCLAVE=baiduxlab/sgx-rust:2004-1.1.3
FROM $SCRT_BASE_IMAGE_ENCLAVE

RUN mkdir -p /opt/secret/.sgx_secrets

RUN rustup target add wasm32-unknown-unknown

COPY scripts/install-wasm-tools.sh .
RUN chmod +x install-wasm-tools.sh
RUN ./install-wasm-tools.sh

RUN make build-test-contract

COPY deployment/ci/go-tests.sh .

RUN chmod +x go-tests.sh

COPY --from=rust-go-base-image /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_enclave.signed.so x/compute/internal/keeper

ENTRYPOINT ["/bin/bash", "go-tests.sh"]
