FROM rust-go-base-image:latest AS build-env-rust-go
# Final image
FROM enigmampc/enigma-sgx-base:2004-1.1.3

# wasmi-sgx-test script requirements
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    #### Base utilities ####
    git \
    make \
    wget && \
    rm -rf /var/lib/apt/lists/*

ARG BUILD_VERSION="v0.5.0-rc1"
ARG SGX_MODE=SW
ENV VERSION=${BUILD_VERSION}
ENV SGX_MODE=${SGX_MODE}

# Install ca-certificates
WORKDIR /root

RUN mkdir -p ./go-cosmwasm/api/

# COPY .git .git
COPY Makefile .

# Copy over binaries from the build-env
COPY --from=build-env-rust-go /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/target/release/libgo_cosmwasm.so ./go-cosmwasm/api/
COPY --from=build-env-rust-go /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_enclave.signed.so ./go-cosmwasm/
COPY --from=build-env-rust-go /go/src/github.com/enigmampc/SecretNetwork/go-cosmwasm/librust_cosmwasm_query_enclave.signed.so ./go-cosmwasm/
COPY --from=build-env-rust-go /go/src/github.com/enigmampc/SecretNetwork/secretd secretd
COPY --from=build-env-rust-go /go/src/github.com/enigmampc/SecretNetwork/secretcli secretcli

COPY ./deployment/deb ./deployment/deb
COPY ./deployment/docker/builder/build_deb.sh .

RUN chmod +x build_deb.sh

# Run secretd by default, omit entrypoint to ease using container with secretcli
CMD ["/bin/bash", "build_deb.sh"]
